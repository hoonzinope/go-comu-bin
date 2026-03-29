package common

import "strings"

func TruncateNotificationSnapshot(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	end := 0
	for i := range value {
		if i > limit {
			break
		}
		end = i
	}
	if end <= 0 {
		return ""
	}
	return strings.TrimSpace(value[:end])
}
