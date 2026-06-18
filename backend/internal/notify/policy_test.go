package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

func TestBuildBatchMessageSingleRateChange(t *testing.T) {
	channel := &storage.Channel{ID: 7, Name: "低价GPT"}
	msg := BuildBatchMessage(channel, []RateChange{{
		GroupName: "gpt pro",
		OldRatio:  0.20,
		NewRatio:  0.15,
		ChangedAt: time.Now(),
	}})

	if msg.Event != storage.EventRateChanged {
		t.Fatalf("event = %q, want %q", msg.Event, storage.EventRateChanged)
	}
	if msg.ChannelID != channel.ID {
		t.Fatalf("channel id = %d, want %d", msg.ChannelID, channel.ID)
	}
	if msg.ModelName != "gpt pro" {
		t.Fatalf("model name = %q, want gpt pro", msg.ModelName)
	}
	for _, want := range []string{"【倍率变动】", "低价GPT", "gpt pro", "下调"} {
		if !strings.Contains(msg.Subject, want) {
			t.Fatalf("subject %q does not contain %q", msg.Subject, want)
		}
	}
	for _, want := range []string{"上游渠道：低价GPT", "分组名称：gpt pro", "0.2 -> 0.15", "-25.0%", "变化方向：下调"} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("body %q does not contain %q", msg.Body, want)
		}
	}
}

func TestBuildBatchMessageMultipleRateChanges(t *testing.T) {
	channel := &storage.Channel{ID: 8, Name: "质量上游"}
	msg := BuildBatchMessage(channel, []RateChange{
		{GroupName: "codex pro", OldRatio: 1.0, NewRatio: 1.2},
		{GroupName: "claude", OldRatio: 2.1, NewRatio: 1.8},
	})

	if !strings.Contains(msg.Subject, "2 个分组变化") {
		t.Fatalf("subject = %q, want merged count", msg.Subject)
	}
	for _, want := range []string{"codex pro：1 -> 1.2（+20.0%，上调）", "claude：2.1 -> 1.8（-14.3%，下调）"} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("body %q does not contain %q", msg.Body, want)
		}
	}
}

func TestSubsetForSubscriptionsFiltersRateGroups(t *testing.T) {
	changes := []RateChange{
		{GroupName: "gpt pro", OldRatio: 0.2, NewRatio: 0.15},
		{GroupName: "claude", OldRatio: 2.1, NewRatio: 1.8},
	}
	subs := []Subscription{{
		ChannelID: 42,
		Mode:      SubscriptionModeGroups,
		Groups:    []string{"claude"},
	}}

	got := subsetForSubscriptions(42, changes, subs)
	if len(got) != 1 || got[0].GroupName != "claude" {
		t.Fatalf("filtered changes = %#v, want only claude", got)
	}
}
