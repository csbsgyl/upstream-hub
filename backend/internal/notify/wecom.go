package notify

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-resty/resty/v2"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func init() {
	Register(storage.NotifyWecom, func(raw string) (Notifier, error) { return newWecom(raw) })
}

type wecomConfig struct {
	WebhookURL string `json:"webhook_url"`
}

type wecom struct {
	cfg  wecomConfig
	http *resty.Client
}

func newWecom(raw string) (*wecom, error) {
	var cfg wecomConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	var err error
	cfg.WebhookURL, err = normalizeNotifyURL(cfg.WebhookURL, "wecom webhook_url")
	if err != nil {
		return nil, err
	}
	return &wecom{cfg: cfg, http: newNotifyHTTPClient()}, nil
}

func (w *wecom) Type() storage.NotificationChannelType { return storage.NotifyWecom }

func (w *wecom) Send(ctx context.Context, msg Message) error {
	resp, err := w.http.R().
		SetContext(ctx).
		SetBody(map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": "**" + msg.Subject + "**\n" + msg.Body,
			},
		}).
		Post(w.cfg.WebhookURL)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return errors.New("wecom returned " + resp.Status())
	}
	return nil
}
