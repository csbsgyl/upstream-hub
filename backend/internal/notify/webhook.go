package notify

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func init() {
	Register(storage.NotifyWebhook, func(raw string) (Notifier, error) { return newWebhook(raw) })
}

type webhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type webhook struct {
	cfg  webhookConfig
	http *resty.Client
}

func newWebhook(raw string) (*webhook, error) {
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	var err error
	cfg.URL, err = normalizeNotifyURL(cfg.URL, "webhook url")
	if err != nil {
		return nil, err
	}
	if cfg.Method == "" {
		cfg.Method = "POST"
	}
	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	switch cfg.Method {
	case "GET", "POST", "PUT":
	default:
		return nil, errors.New("webhook method must be GET, POST, or PUT")
	}
	return &webhook{cfg: cfg, http: newNotifyHTTPClient()}, nil
}

func (w *webhook) Type() storage.NotificationChannelType { return storage.NotifyWebhook }

func (w *webhook) Send(ctx context.Context, msg Message) error {
	req := w.http.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"event":   msg.Event,
			"subject": msg.Subject,
			"body":    msg.Body,
			"extra":   msg.Extra,
		})
	for k, v := range w.cfg.Headers {
		req.SetHeader(k, v)
	}
	resp, err := req.Execute(w.cfg.Method, w.cfg.URL)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return errors.New("webhook returned " + resp.Status())
	}
	return nil
}
