package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/worryzyy/upstream-hub/internal/crypto"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

// Dispatcher 把单条事件 fan-out 到所有启用的通知渠道。
type Dispatcher struct {
	repo   *storage.Notifications
	cipher *crypto.Cipher
	log    *slog.Logger
}

func NewDispatcher(repo *storage.Notifications, cipher *crypto.Cipher, log *slog.Logger) *Dispatcher {
	return &Dispatcher{repo: repo, cipher: cipher, log: log}
}

// Send 把消息发送到一个具体的渠道（用于"测试发送"按钮）。
func (d *Dispatcher) Send(ctx context.Context, ch *storage.NotificationChannel, msg Message) error {
	cfgJSON, err := d.cipher.Decrypt(ch.ConfigCipher)
	if err != nil {
		return fmt.Errorf("decrypt config: %w", err)
	}
	n, err := Build(ch, cfgJSON)
	if err != nil {
		return err
	}
	err = n.Send(ctx, msg)
	d.logResult(ch.ID, msg, err)
	return err
}

// Dispatch 按事件类型广播到所有启用的通知渠道，返回累计错误（部分失败也会写日志）。
//
// 订阅过滤：渠道配置 Subscriptions 非空时，必须有任意一条订阅命中 msg 才发送；
// 空订阅列表（""/null/[]）视为"订阅一切"，向后兼容已有通知渠道。
func (d *Dispatcher) Dispatch(ctx context.Context, msg Message) error {
	channels, err := d.repo.ListEnabledChannels()
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		return nil
	}
	var errs []error
	for i := range channels {
		ch := channels[i]
		subs, _ := ParseSubscriptions(ch.Subscriptions)
		if len(subs) > 0 && !AnyMatch(subs, msg) {
			continue
		}
		cfgJSON, err := d.cipher.Decrypt(ch.ConfigCipher)
		if err != nil {
			errs = append(errs, fmt.Errorf("decrypt %s: %w", ch.Name, err))
			d.logResult(ch.ID, msg, err)
			continue
		}
		n, err := Build(&ch, cfgJSON)
		if err != nil {
			errs = append(errs, fmt.Errorf("build %s: %w", ch.Name, err))
			d.logResult(ch.ID, msg, err)
			continue
		}
		if err := n.Send(ctx, msg); err != nil {
			errs = append(errs, fmt.Errorf("send via %s: %w", ch.Name, err))
			d.logResult(ch.ID, msg, err)
			continue
		}
		d.logResult(ch.ID, msg, nil)
	}
	return errors.Join(errs...)
}

func (d *Dispatcher) logResult(channelID uint, msg Message, sendErr error) {
	log := &storage.NotificationLog{
		ChannelID: channelID,
		Event:     msg.Event,
		Subject:   msg.Subject,
		Body:      msg.Body,
		Success:   sendErr == nil,
	}
	if sendErr != nil {
		log.ErrorMessage = sendErr.Error()
	}
	if err := d.repo.AppendLog(log); err != nil && d.log != nil {
		d.log.Warn("append notification log", "err", err)
	}
}
