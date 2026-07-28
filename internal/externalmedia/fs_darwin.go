//go:build darwin

package externalmedia

import "path/filepath"

func platformDownloadsDir(home string) string {
	return filepath.Join(home, "Downloads")
}
