package notify

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
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
		Subject:   fmt.Sprintf("【%s】余额预警 · %s", defaultAppName, name),
		Body: joinNotifySections(
			[]string{
				"告警类型：余额预警",
				"影响上游：" + name,
				fmt.Sprintf("当前余额：%.4f", balance),
				fmt.Sprintf("告警阈值：%.4f", threshold),
				"采集时间：" + formatNotifyTime(sampledAt),
			},
			[]string{
				"处理建议：",
				"1. 及时补充上游余额，避免用户请求失败",
				"2. 如果阈值设置偏高，可在渠道配置中调整告警阈值",
			},
		),
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
		Subject:   fmt.Sprintf("【%s】%s · %s", defaultAppName, strings.TrimSpace(title), name),
		Body: joinNotifySections(
			[]string{
				"告警类型：" + strings.TrimSpace(title),
				"影响上游：" + name,
				"失败原因：" + reason,
				"发生时间：" + formatNotifyTime(time.Now()),
			},
			[]string{
				"处理建议：",
				"1. 检查账号状态、Token/Cookie 或上游后台权限",
				"2. 确认服务器网络可访问该上游，再重新触发同步",
			},
		),
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
	return t.Format("2006-01-02 15:04:05")
}

func sanitizeNotifyText(s string) string {
	s = strings.TrimSpace(s)
	for _, pattern := range sensitiveTextPatterns {
		s = pattern.ReplaceAllString(s, "${1}[已隐藏]")
	}
	return s
}
