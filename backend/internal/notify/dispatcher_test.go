package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

type cooldownStub struct {
	claimOK bool
	claims  int
	resets  int
}

func (s *cooldownStub) TryClaimCooldown(uint, storage.NotificationEvent, time.Duration) (bool, error) {
	s.claims++
	return s.claimOK, nil
}

func (s *cooldownStub) ResetCooldown(uint, storage.NotificationEvent) error {
	s.resets++
	return nil
}

func TestDispatchResetsBalanceCooldownWhenUndelivered(t *testing.T) {
	cooldown := &cooldownStub{claimOK: true}
	d := NewDispatcherWithCooldown(nil, nil, nil, Policy{
		BalanceLowCooldown: time.Hour,
		SendMaxAttempts:    1,
	}, cooldown)
	d.fanoutFunc = func(context.Context, Message, func(*storage.NotificationChannel) bool) fanoutResult {
		return fanoutResult{attempted: 1, succeeded: 0, err: errors.New("send failed")}
	}

	err := d.Dispatch(context.Background(), Message{
		Event:     storage.EventBalanceLow,
		ChannelID: 11,
		Subject:   "low",
	})

	if err == nil {
		t.Fatal("Dispatch returned nil, want send error")
	}
	if cooldown.claims != 1 {
		t.Fatalf("cooldown claims = %d, want 1", cooldown.claims)
	}
	if cooldown.resets != 1 {
		t.Fatalf("cooldown resets = %d, want 1", cooldown.resets)
	}
}

func TestDispatchKeepsBalanceCooldownAfterSuccess(t *testing.T) {
	cooldown := &cooldownStub{claimOK: true}
	d := NewDispatcherWithCooldown(nil, nil, nil, Policy{
		BalanceLowCooldown: time.Hour,
		SendMaxAttempts:    1,
	}, cooldown)
	d.fanoutFunc = func(context.Context, Message, func(*storage.NotificationChannel) bool) fanoutResult {
		return fanoutResult{attempted: 2, succeeded: 1, err: errors.New("one channel failed")}
	}

	err := d.Dispatch(context.Background(), Message{
		Event:     storage.EventBalanceLow,
		ChannelID: 11,
		Subject:   "low",
	})

	if err == nil {
		t.Fatal("Dispatch returned nil, want partial send error")
	}
	if cooldown.resets != 0 {
		t.Fatalf("cooldown resets = %d, want 0", cooldown.resets)
	}
}

func TestDispatchSuppressesBalanceCooldown(t *testing.T) {
	cooldown := &cooldownStub{claimOK: false}
	d := NewDispatcherWithCooldown(nil, nil, nil, Policy{
		BalanceLowCooldown: time.Hour,
		SendMaxAttempts:    1,
	}, cooldown)
	called := false
	d.fanoutFunc = func(context.Context, Message, func(*storage.NotificationChannel) bool) fanoutResult {
		called = true
		return fanoutResult{}
	}

	err := d.Dispatch(context.Background(), Message{
		Event:     storage.EventBalanceLow,
		ChannelID: 11,
		Subject:   "low",
	})

	if err != nil {
		t.Fatalf("Dispatch error = %v, want nil", err)
	}
	if called {
		t.Fatal("fanout was called while cooldown should suppress send")
	}
	if cooldown.resets != 0 {
		t.Fatalf("cooldown resets = %d, want 0", cooldown.resets)
	}
}
