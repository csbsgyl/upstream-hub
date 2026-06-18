package api

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func audit(c *gin.Context, d *Deps, action, resourceType string, resourceID uint, summary string, metadata map[string]any) {
	if d == nil || d.AuditLogs == nil {
		return
	}
	actor := "anonymous"
	if v, ok := c.Get("authSubject"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			actor = strings.TrimSpace(s)
		}
	}

	meta := ""
	if len(metadata) > 0 {
		if b, err := json.Marshal(metadata); err == nil {
			meta = string(b)
		}
	}
	if err := d.AuditLogs.Append(&storage.AuditLog{
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Summary:      summary,
		Metadata:     meta,
	}); err != nil && d.Log != nil {
		d.Log.Warn("append audit log failed", "err", err, "action", action, "resource", resourceType, "id", resourceID)
	}
}
