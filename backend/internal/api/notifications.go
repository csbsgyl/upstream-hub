package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/notify"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func registerNotifications(g *gin.RouterGroup, d *Deps) {
	gpc := g.Group("/notifications/channels")
	gpc.GET("", func(c *gin.Context) {
		list, err := d.Notifies.ListChannels()
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": list})
	})
	gpc.POST("", func(c *gin.Context) { createNotifyChannel(c, d) })
	gpc.PUT("/:id", func(c *gin.Context) { updateNotifyChannel(c, d) })
	gpc.DELETE("/:id", func(c *gin.Context) {
		id, err := uintParam(c, "id")
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		ch, _ := d.Notifies.FindChannel(id)
		if err := d.Notifies.DeleteChannel(id); err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		name := ""
		if ch != nil {
			name = ch.Name
		}
		audit(c, d, "notification_channel.delete", "notification_channel", id, "deleted notification channel "+name, gin.H{"name": name})
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	gpc.POST("/:id/test", func(c *gin.Context) { testNotify(c, d) })

	g.GET("/notifications/logs", func(c *gin.Context) {
		limit := queryIntClamped(c, "limit", 100, 1, 500)
		list, err := d.Notifies.ListLogs(limit)
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": list})
	})
	g.GET("/notifications/failed", func(c *gin.Context) {
		limit := queryIntClamped(c, "limit", 100, 1, 500)
		list, err := d.Notifies.ListFailedLogs(limit)
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": list})
	})
	g.POST("/notifications/logs/:id/retry", func(c *gin.Context) { retryNotificationLog(c, d) })
}

type notifyChannelInput struct {
	Name          string                          `json:"name"`
	Type          storage.NotificationChannelType `json:"type"`
	Config        string                          `json:"config"` // JSON string；编辑时可留空保留原值
	Subscriptions *string                         `json:"subscriptions"`
	Enabled       *bool                           `json:"enabled"`
}

// normalizeSubscriptions 把输入的订阅 JSON 字符串规整为 "[]" 或合法 JSON 数组。
// 解析失败返回错误以便 API 返回 400。
func normalizeSubscriptions(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return "[]", nil
	}
	var list []notify.Subscription
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return "", err
	}
	for i := range list {
		if list[i].ChannelID == 0 {
			return "", errors.New("subscription channel_id is required")
		}
		switch list[i].Mode {
		case "", notify.SubscriptionModeAll:
			list[i].Mode = notify.SubscriptionModeAll
			list[i].Groups = nil
		case notify.SubscriptionModeGroups:
			groups := make([]string, 0, len(list[i].Groups))
			seen := map[string]struct{}{}
			for _, group := range list[i].Groups {
				group = strings.TrimSpace(group)
				if group == "" {
					continue
				}
				if _, ok := seen[group]; ok {
					continue
				}
				seen[group] = struct{}{}
				groups = append(groups, group)
			}
			if len(groups) == 0 {
				return "", errors.New("groups subscription requires at least one group")
			}
			list[i].Groups = groups
		default:
			return "", errors.New("subscription mode must be all or groups")
		}
	}
	out, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func createNotifyChannel(c *gin.Context, d *Deps) {
	var in notifyChannelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		fail(c, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if in.Type == "" {
		fail(c, http.StatusBadRequest, errors.New("type is required"))
		return
	}
	if in.Config == "" {
		fail(c, http.StatusBadRequest, errors.New("config is required"))
		return
	}
	if err := validateNotifyConfig(in.Type, in.Config); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	rawSubs := ""
	if in.Subscriptions != nil {
		rawSubs = *in.Subscriptions
	}
	subs, err := normalizeSubscriptions(rawSubs)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	cipherCfg, err := d.Cipher.Encrypt(in.Config)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	ch := &storage.NotificationChannel{
		Name:          in.Name,
		Type:          in.Type,
		ConfigCipher:  cipherCfg,
		Subscriptions: subs,
		Enabled:       enabled,
	}
	if err := d.Notifies.CreateChannel(ch); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	audit(c, d, "notification_channel.create", "notification_channel", ch.ID, "created notification channel "+ch.Name, gin.H{
		"name":    ch.Name,
		"type":    ch.Type,
		"enabled": ch.Enabled,
	})
	c.JSON(http.StatusOK, gin.H{"data": ch})
}

func updateNotifyChannel(c *gin.Context, d *Deps) {
	id, err := uintParam(c, "id")
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ch, err := d.Notifies.FindChannel(id)
	if err != nil {
		fail(c, http.StatusNotFound, err)
		return
	}
	var in notifyChannelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(in.Name) != "" {
		ch.Name = strings.TrimSpace(in.Name)
	}
	if in.Type != "" && in.Type != ch.Type {
		fail(c, http.StatusBadRequest, errors.New("notification type cannot be changed after creation"))
		return
	}
	if in.Enabled != nil {
		ch.Enabled = *in.Enabled
	}
	if in.Subscriptions != nil {
		subs, err := normalizeSubscriptions(*in.Subscriptions)
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		ch.Subscriptions = subs
	}
	if in.Config != "" {
		if err := validateNotifyConfig(ch.Type, in.Config); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		cipherCfg, err := d.Cipher.Encrypt(in.Config)
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		ch.ConfigCipher = cipherCfg
	}
	if err := d.Notifies.UpdateChannel(ch); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	audit(c, d, "notification_channel.update", "notification_channel", ch.ID, "updated notification channel "+ch.Name, gin.H{
		"name":    ch.Name,
		"type":    ch.Type,
		"enabled": ch.Enabled,
	})
	c.JSON(http.StatusOK, gin.H{"data": ch})
}

func validateNotifyConfig(t storage.NotificationChannelType, raw string) error {
	_, err := notify.Build(&storage.NotificationChannel{Type: t}, raw)
	return err
}

func testNotify(c *gin.Context, d *Deps) {
	id, err := uintParam(c, "id")
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ch, err := d.Notifies.FindChannel(id)
	if err != nil {
		fail(c, http.StatusNotFound, err)
		return
	}
	displayName := notify.DisplayName(ch)
	msg := notify.Message{
		Subject: "[" + displayName + "] 测试通知",
		Body:    "这是一条来自 " + displayName + " 的测试消息。",
	}
	if err := d.Dispatcher.Send(c.Request.Context(), ch, msg); err != nil {
		audit(c, d, "notification_channel.test", "notification_channel", ch.ID, "tested notification channel "+ch.Name, gin.H{
			"ok":    false,
			"error": err.Error(),
		})
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	audit(c, d, "notification_channel.test", "notification_channel", ch.ID, "tested notification channel "+ch.Name, gin.H{"ok": true})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func retryNotificationLog(c *gin.Context, d *Deps) {
	id, err := uintParam(c, "id")
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	logRow, err := d.Notifies.FindLog(id)
	if err != nil {
		fail(c, http.StatusNotFound, err)
		return
	}
	ch, err := d.Notifies.FindChannel(logRow.ChannelID)
	if err != nil {
		fail(c, http.StatusNotFound, err)
		return
	}
	msg := notify.Message{
		Event:   logRow.Event,
		Subject: logRow.Subject,
		Body:    logRow.Body,
	}
	if err := d.Dispatcher.Resend(c.Request.Context(), ch, msg); err != nil {
		audit(c, d, "notification_log.retry", "notification_log", logRow.ID, "retried failed notification "+logRow.Subject, gin.H{
			"ok":                      false,
			"notification_channel_id": ch.ID,
			"event":                   logRow.Event,
			"error":                   err.Error(),
		})
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	audit(c, d, "notification_log.retry", "notification_log", logRow.ID, "retried notification "+logRow.Subject, gin.H{
		"ok":                      true,
		"notification_channel_id": ch.ID,
		"event":                   logRow.Event,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
