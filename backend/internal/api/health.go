package api

import (
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

type channelHealth struct {
	Score  int    `json:"health_score"`
	Status string `json:"health_status"`
}

func computeChannelHealth(ch storage.Channel, now time.Time) channelHealth {
	score := 100
	status := "healthy"

	if !ch.MonitorEnabled {
		score -= 25
		status = "paused"
	}
	if ch.LastError != "" {
		score -= 55
		status = "failed"
	}
	if ch.LastBalance == nil {
		score -= 20
		if status == "healthy" {
			status = "idle"
		}
	} else if ch.BalanceThreshold > 0 && *ch.LastBalance < ch.BalanceThreshold {
		score -= 25
		if status == "healthy" {
			status = "low_balance"
		}
	}
	if ch.LastBalanceAt == nil {
		score -= 10
	} else {
		age := now.Sub(*ch.LastBalanceAt)
		switch {
		case age > 24*time.Hour:
			score -= 35
			if status == "healthy" {
				status = "stale"
			}
		case age > 2*time.Hour:
			score -= 15
			if status == "healthy" {
				status = "stale"
			}
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return channelHealth{Score: score, Status: status}
}
