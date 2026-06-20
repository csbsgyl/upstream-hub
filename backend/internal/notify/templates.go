package notify

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

var (
	notifyLocationOnce sync.Once
	notifyLocation     *time.Location
)

var sensitiveTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(["']?(?:token|access_token|refresh_token|api[_-]?key|authorization|cookie|session|password|secret)["']?\s*[:=]\s*)(["'][^"']*["']|[^\s,;&]+)`),
	regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._~+/-]+=*`),
}

// BuildTestMessage creates a polished message for the notification-channel test button.
func BuildTestMessage(ch *storage.NotificationChannel) Message {
	displayName := DisplayName(ch)
	return Message{
		Subject: fmt.Sprintf("【%s】通知通道测试", displayName),
		Body: joinNotifySections(
			[]string{
				"通知类型：通道测试",
				"通知通道：" + displayName,
				"发送结果：链路可用",
				"发送时间：" + formatNotifyTime(time.Now()),
			},
			[]string{
				"处理建议：",
				"1. 如果能收到这条消息，说明当前通知配置可以正常推送",
				"2. 后续余额预警、倍率变动和采集异常会按订阅规则发送到这里",
			},
		),
	}
}

// BuildBalanceLowMessage creates the alert sent when an upstream balance is below its threshold.
func BuildBalanceLowMessage(channel *storage.Channel, balance, threshold float64, sampledAt time.Time) Message {
	name, id := upstreamIdentity(channel)
	if sampledAt.IsZero() {
		sampledAt = time.Now()
	}
	return Message{
		Event:     storage.EventBalanceLow,
		ChannelID: id,
		Subject:   fmt.Sprintf("⚠️【%s】余额告警 · %s", defaultAppName, name),
		Body: joinNotifySections([]string{
			"🚨 余额低于阈值",
			"上游：" + name,
			fmt.Sprintf("余额：%.4f / 阈值 %.4f", balance, threshold),
			"时间：" + formatNotifyTime(sampledAt),
		}),
	}
}

// BuildFailureMessage creates login/captcha/monitor failure alerts.
func BuildFailureMessage(channel *storage.Channel, event storage.NotificationEvent, title string, err error) Message {
	name, id := upstreamIdentity(channel)
	reason := "未知错误"
	if err != nil {
		reason = sanitizeNotifyText(err.Error())
	}
	return Message{
		Event:     event,
		ChannelID: id,
		Subject:   fmt.Sprintf("🚫【%s】%s · %s", defaultAppName, strings.TrimSpace(title), name),
		Body: joinNotifySections([]string{
			"🚫 " + strings.TrimSpace(title),
			"上游：" + name,
			"原因：" + reason,
			"时间：" + formatNotifyTime(time.Now()),
		}),
	}
}

func upstreamIdentity(channel *storage.Channel) (string, uint) {
	if channel == nil {
		return "未知上游", 0
	}
	name := strings.TrimSpace(channel.Name)
	if name == "" {
		name = "未命名上游"
	}
	return name, channel.ID
}

func joinNotifySections(sections ...[]string) string {
	var blocks []string
	for _, lines := range sections {
		clean := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimRight(line, " \t")
			if line != "" {
				clean = append(clean, line)
			}
		}
		if len(clean) > 0 {
			blocks = append(blocks, strings.Join(clean, "\n"))
		}
	}
	return strings.Join(blocks, "\n\n")
}

func formatNotifyTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.In(chinaNotifyLocation()).Format("2006-01-02 15:04:05")
}

func chinaNotifyLocation() *time.Location {
	notifyLocationOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			loc = time.FixedZone("Asia/Shanghai", 8*60*60)
		}
		notifyLocation = loc
	})
	return notifyLocation
}

func sanitizeNotifyText(s string) string {
	s = strings.TrimSpace(s)
	for _, pattern := range sensitiveTextPatterns {
		s = pattern.ReplaceAllString(s, "${1}[已隐藏]")
	}
	return s
}
