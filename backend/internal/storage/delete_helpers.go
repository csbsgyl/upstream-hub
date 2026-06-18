package storage

import (
	"fmt"
	"time"
)

func deletedName(name string, id uint) string {
	suffix := fmt.Sprintf("#deleted-%d-%d", id, time.Now().Unix())
	const maxLen = 128
	baseRunes := []rune(name)
	maxBase := maxLen - len([]rune(suffix))
	if maxBase < 1 {
		return suffix[1:]
	}
	if len(baseRunes) > maxBase {
		baseRunes = baseRunes[:maxBase]
	}
	if len(baseRunes) == 0 {
		baseRunes = []rune("deleted")
	}
	return string(baseRunes) + suffix
}
