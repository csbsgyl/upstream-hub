package notify

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

// Policy 通知去抖策略。所有字段都是面向"少烦用户"取向：
//   - BatchRateChanges：同次扫描中合并多条 rate_changed 成一条消息
//   - MinChangePct：涨跌幅小于阈值时跳过推送（仍写入 RateChangeLog 表）
//   - RateChangeDirection：rate_changed 方向过滤，all / increase / decrease
//   - QuietGroups：不推送指定分组名的 rate_changed（仍写入 RateChangeLog 表）
//   - BalanceLowCooldown：同渠道 balance_low 在窗口内不重复发送
//   - FailureCooldown：同渠道登录/采集失败在窗口内不重复发送
//   - SendMaxAttempts：单条消息最多发送尝试次数（含首发），<=1 表示不重试
type Policy struct {
	BatchRateChanges    bool
	MinChangePct        float64
	RateChangeDirection string
	QuietGroups         []string
	BalanceLowCooldown  time.Duration
	FailureCooldown     time.Duration
	SendMaxAttempts     int
}

// CooldownStore Dispatcher 用来判断某个 (channelID, event) 是否还在冷却窗口。
//
// 抽象成 interface 是为了让 dispatcher 不依赖具体存储；
// 生产实现是 *storage.Notifications.TryClaimCooldown（PostgreSQL UPSERT）；
// 测试时可以注入一个内存 stub。
type CooldownStore interface {
	TryClaimCooldown(channelID uint, event storage.NotificationEvent, cooldown time.Duration) (bool, error)
	ResetCooldown(channelID uint, event storage.NotificationEvent) error
}

// RateChange 是一条待发送的倍率变化记录（去抖 / 合并的基本单元）。
type RateChange struct {
	GroupName string
	OldRatio  float64
	NewRatio  float64
	OldComp   float64
	NewComp   float64
	ChangedAt time.Time
}

// ChangePctAbove 涨跌幅是否达到阈值。
// minPct = 0 表示不过滤。OldRatio = 0 时按"新出现的分组"处理，永远算"达到阈值"。
func (rc RateChange) ChangePctAbove(minPct float64) bool {
	if minPct <= 0 {
		return true
	}
	if rc.OldRatio == 0 && rc.NewRatio != rc.OldRatio {
		return true
	}
	ratioPct := math.Abs(rc.NewRatio-rc.OldRatio) / math.Abs(rc.OldRatio) * 100
	if ratioPct >= minPct {
		return true
	}
	if rc.NewComp == rc.OldComp {
		return false
	}
	if rc.OldComp == 0 {
		return true
	}
	compPct := math.Abs(rc.NewComp-rc.OldComp) / math.Abs(rc.OldComp) * 100
	return compPct >= minPct
}

func (rc RateChange) AllowedByPolicy(p Policy) bool {
	if !rc.ChangePctAbove(p.MinChangePct) {
		return false
	}
	if rc.quietedBy(p.QuietGroups) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(p.RateChangeDirection)) {
	case "", "all":
		return true
	case "increase", "up":
		return rc.NewRatio > rc.OldRatio || rc.NewComp > rc.OldComp
	case "decrease", "down":
		return rc.NewRatio < rc.OldRatio || rc.NewComp < rc.OldComp
	default:
		return true
	}
}

func (rc RateChange) quietedBy(groups []string) bool {
	name := strings.TrimSpace(strings.ToLower(rc.GroupName))
	if name == "" || len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if strings.TrimSpace(strings.ToLower(g)) == name {
			return true
		}
	}
	return false
}

// BuildBatchMessage 把多条 RateChange 合并成一条 notify.Message。
// 当只有 1 条时仍走这个路径，但 Subject / Body 自然退化成单条提醒。
func BuildBatchMessage(channel *storage.Channel, changes []RateChange) Message {
	if len(changes) == 0 {
		return Message{}
	}
	now := time.Now()
	if len(changes) == 1 {
		c := changes[0]
		direction := changeDirectionLabel(c)
		return Message{
			Event:     storage.EventRateChanged,
			ChannelID: channel.ID,
			ModelName: c.GroupName,
			Subject:   fmt.Sprintf("【倍率变动】%s · %s %s", channel.Name, c.GroupName, direction),
			Body: fmt.Sprintf(
				"上游渠道：%s\n分组名称：%s\n倍率变化：%s -> %s（%s）\n补全倍率：%s -> %s（%s）\n变化方向：%s\n采集时间：%s",
				channel.Name,
				c.GroupName,
				formatRatio(c.OldRatio),
				formatRatio(c.NewRatio),
				formatChangePct(c.OldRatio, c.NewRatio),
				formatRatio(c.OldComp),
				formatRatio(c.NewComp),
				formatChangePct(c.OldComp, c.NewComp),
				direction,
				now.Format("2006-01-02 15:04"),
			),
		}
	}

	// 合并多条：subject 简短，body 列出每条。
	var b strings.Builder
	fmt.Fprintf(&b, "上游渠道：%s\n共 %d 个分组倍率发生变化：\n", channel.Name, len(changes))
	for _, c := range changes {
		fmt.Fprintf(&b, "- %s：倍率 %s -> %s（%s），补全 %s -> %s（%s，%s）\n",
			c.GroupName,
			formatRatio(c.OldRatio),
			formatRatio(c.NewRatio),
			formatChangePct(c.OldRatio, c.NewRatio),
			formatRatio(c.OldComp),
			formatRatio(c.NewComp),
			formatChangePct(c.OldComp, c.NewComp),
			changeDirectionLabel(c),
		)
	}
	fmt.Fprintf(&b, "采集时间：%s", now.Format("2006-01-02 15:04"))

	// ModelName 在合并消息里没有单一值；填空，订阅过滤改在 Dispatcher 里按"先按订阅切片再合并"处理。
	return Message{
		Event:     storage.EventRateChanged,
		ChannelID: channel.ID,
		ModelName: "",
		Subject:   fmt.Sprintf("【倍率变动】%s · %d 个分组变化", channel.Name, len(changes)),
		Body:      b.String(),
	}
}

func directionLabel(oldV, newV float64) string {
	switch {
	case newV > oldV:
		return "上调"
	case newV < oldV:
		return "下调"
	default:
		return "调整"
	}
}

func changeDirectionLabel(c RateChange) string {
	ratioDir := directionLabel(c.OldRatio, c.NewRatio)
	compDir := directionLabel(c.OldComp, c.NewComp)
	if ratioDir == compDir {
		return ratioDir
	}
	if ratioDir == "调整" {
		return compDir
	}
	if compDir == "调整" {
		return ratioDir
	}
	return "调整"
}

func formatRatio(v float64) string {
	return fmt.Sprintf("%.4g", v)
}

func formatChangePct(oldV, newV float64) string {
	if oldV == 0 {
		return "新倍率"
	}
	pct := (newV - oldV) / math.Abs(oldV) * 100
	return fmt.Sprintf("%+.1f%%", pct)
}
