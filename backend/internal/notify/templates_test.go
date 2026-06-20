package notify

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

func TestBuildTestMessageUsesNotificationChannelName(t *testing.T) {
	ch := &storage.NotificationChannel{Name: "小航云中转额度"}
	msg := BuildTestMessage(ch)

	for _, want := range []string{"【小航云中转额度】通知通道测试", "通知类型：通道测试", "发送结果：链路可用"} {
		if !strings.Contains(msg.Subject+"\n"+msg.Body, want) {
			t.Fatalf("message %q\n%q does not contain %q", msg.Subject, msg.Body, want)
		}
	}
}

func TestBuildBalanceLowMessageIncludesOperationalDetails(t *testing.T) {
	channel := &storage.Channel{ID: 3, Name: "可达鸭"}
	msg := BuildBalanceLowMessage(channel, 7.3249, 10, time.Date(2026, 6, 19, 9, 30, 0, 0, time.Local))

	if msg.Event != storage.EventBalanceLow {
		t.Fatalf("event = %q, want %q", msg.Event, storage.EventBalanceLow)
	}
	if msg.ChannelID != channel.ID {
		t.Fatalf("channel id = %d, want %d", msg.ChannelID, channel.ID)
	}
	for _, want := range []string{"【upstream-hub】余额预警 · 可达鸭", "当前余额：7.3249", "告警阈值：10.0000", "及时补充上游余额"} {
		if !strings.Contains(msg.Subject+"\n"+msg.Body, want) {
			t.Fatalf("message %q\n%q does not contain %q", msg.Subject, msg.Body, want)
		}
	}
}

func TestBuildFailureMessageRedactsSensitiveValues(t *testing.T) {
	channel := &storage.Channel{ID: 5, Name: "质量上游"}
	msg := BuildFailureMessage(channel, storage.EventLoginFailed, "登录失败", errors.New(`status 403 token=ghp_secret Bearer abc.def.ghi cookie: "sid=secret"`))

	if msg.Event != storage.EventLoginFailed {
		t.Fatalf("event = %q, want %q", msg.Event, storage.EventLoginFailed)
	}
	if strings.Contains(msg.Body, "ghp_secret") || strings.Contains(msg.Body, "abc.def.ghi") || strings.Contains(msg.Body, "sid=secret") {
		t.Fatalf("body leaked sensitive value: %q", msg.Body)
	}
	for _, want := range []string{"告警类型：登录失败", "影响上游：质量上游", "[已隐藏]", "检查账号状态"} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("body %q does not contain %q", msg.Body, want)
		}
	}
}

func TestFormatNotifyTimeUsesChinaTime(t *testing.T) {
	utc := time.Date(2026, 6, 19, 1, 2, 3, 0, time.UTC)
	got := formatNotifyTime(utc)
	want := "2026-06-19 09:02:03"
	if got != want {
		t.Fatalf("formatNotifyTime() = %q, want %q", got, want)
	}
}
