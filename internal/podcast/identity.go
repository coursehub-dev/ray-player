package podcast

import "strings"

func IsItemID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}

	return strings.HasPrefix(id, "podcast_") ||
		strings.HasPrefix(id, "external-podcast-") ||
		strings.HasPrefix(id, "podcast-external-")
}
