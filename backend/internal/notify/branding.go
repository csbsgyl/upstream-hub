package notify

import (
	"strings"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

const defaultAppName = "upstream-hub"

// DisplayName is the user-facing name for messages sent through a notification channel.
func DisplayName(ch *storage.NotificationChannel) string {
	if ch == nil {
		return defaultAppName
	}
	name := strings.TrimSpace(ch.Name)
	if name == "" {
		return defaultAppName
	}
	return name
}

func brandMessage(ch *storage.NotificationChannel, msg Message) Message {
	displayName := DisplayName(ch)
	if displayName == defaultAppName {
		return msg
	}
	branded := msg
	branded.Subject = strings.ReplaceAll(branded.Subject, "["+defaultAppName+"]", "["+displayName+"]")
	branded.Subject = strings.ReplaceAll(branded.Subject, "【"+defaultAppName+"】", "【"+displayName+"】")
	branded.Body = strings.ReplaceAll(branded.Body, "来自 "+defaultAppName, "来自 "+displayName)
	return branded
}
