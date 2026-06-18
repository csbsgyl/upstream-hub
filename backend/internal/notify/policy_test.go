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
		OldComp:   0.30,
		NewComp:   0.25,
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
	for _, want := range []string{"【upstream-hub】倍率变动", "低价GPT"} {
		if !strings.Contains(msg.Subject, want) {
			t.Fatalf("subject %q does not contain %q", msg.Subject, want)
		}
	}
	for _, want := range []string{"告警类型：倍率变动", "影响上游：低价GPT", "变动分组：gpt pro", "0.2 -> 0.15", "-25.0%", "0.3 -> 0.25", "-16.7%", "变化方向：下调", "处理建议："} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("body %q does not contain %q", msg.Body, want)
		}
	}
}

func TestBuildBatchMessageMultipleRateChanges(t *testing.T) {
	channel := &storage.Channel{ID: 8, Name: "质量上游"}
	msg := BuildBatchMessage(channel, []RateChange{
		{GroupName: "codex pro", OldRatio: 1.0, NewRatio: 1.2, OldComp: 1.0, NewComp: 1.0},
		{GroupName: "claude", OldRatio: 2.1, NewRatio: 1.8, OldComp: 2.0, NewComp: 1.7},
	})

	if !strings.Contains(msg.Subject, "2 个分组") {
		t.Fatalf("subject = %q, want merged count", msg.Subject)
	}
	for _, want := range []string{"告警类型：倍率变动", "影响上游：质量上游", "变动数量：2 个分组", "codex pro：倍率 1 -> 1.2（+20.0%），补全 1 -> 1（+0.0%，上调）", "claude：倍率 2.1 -> 1.8（-14.3%），补全 2 -> 1.7（-15.0%，下调）", "处理建议："} {
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

func TestSubsetForSubscriptionsMatchesRateGroupsLoosely(t *testing.T) {
	changes := []RateChange{
		{GroupName: "Claude", OldRatio: 2.1, NewRatio: 1.8},
	}
	subs := []Subscription{{
		ChannelID: 42,
		Mode:      SubscriptionModeGroups,
		Groups:    []string{" claude "},
	}}

	got := subsetForSubscriptions(42, changes, subs)
	if len(got) != 1 || got[0].GroupName != "Claude" {
		t.Fatalf("filtered changes = %#v, want loose match for Claude", got)
	}
}

func TestRateChangeAllowedByPolicyDirectionAndQuietGroups(t *testing.T) {
	up := RateChange{GroupName: "gpt pro", OldRatio: 1, NewRatio: 1.5}
	down := RateChange{GroupName: "claude", OldRatio: 2, NewRatio: 1.5}

	if !up.AllowedByPolicy(Policy{RateChangeDirection: "increase"}) {
		t.Fatal("increase policy should allow upward change")
	}
	if down.AllowedByPolicy(Policy{RateChangeDirection: "increase"}) {
		t.Fatal("increase policy should reject downward change")
	}
	if up.AllowedByPolicy(Policy{RateChangeDirection: "decrease"}) {
		t.Fatal("decrease policy should reject upward change")
	}
	if !down.AllowedByPolicy(Policy{RateChangeDirection: "decrease"}) {
		t.Fatal("decrease policy should allow downward change")
	}
	if up.AllowedByPolicy(Policy{QuietGroups: []string{" GPT PRO "}}) {
		t.Fatal("quiet group should reject matching group case-insensitively")
	}
}

func TestRateChangeAllowedByPolicyMinPct(t *testing.T) {
	small := RateChange{GroupName: "gpt pro", OldRatio: 1, NewRatio: 1.01}
	if small.AllowedByPolicy(Policy{MinChangePct: 5}) {
		t.Fatal("min pct policy should reject small changes")
	}
	if !small.AllowedByPolicy(Policy{MinChangePct: 0}) {
		t.Fatal("zero min pct should allow changes")
	}
}

func TestRateChangeAllowedByPolicyCompletionRatio(t *testing.T) {
	completionOnly := RateChange{GroupName: "gpt pro", OldRatio: 1, NewRatio: 1, OldComp: 1, NewComp: 1.2}
	if !completionOnly.AllowedByPolicy(Policy{MinChangePct: 5}) {
		t.Fatal("min pct policy should allow completion-ratio changes above threshold")
	}
	if !completionOnly.AllowedByPolicy(Policy{RateChangeDirection: "increase"}) {
		t.Fatal("increase policy should allow completion-ratio increase")
	}
	if completionOnly.AllowedByPolicy(Policy{RateChangeDirection: "decrease"}) {
		t.Fatal("decrease policy should reject completion-ratio increase")
	}
}
