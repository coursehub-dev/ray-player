//go:build windows

package externalmedia

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsDownloadsKnownFolderID = "{374DE290-123F-4565-9164-39C4925E467B}"

func platformDownloadsDir(home string) string {
	for _, keyPath := range []string{
		`Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders`,
	} {
		key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		dir := readWindowsDownloadsValue(key, home)
		_ = key.Close()
		if dir != "" {
			return dir
		}
	}
	return ""
}

func readWindowsDownloadsValue(key registry.Key, home string) string {
	for _, valueName := range []string{windowsDownloadsKnownFolderID, "Downloads"} {
		value, _, err := key.GetStringValue(valueName)
		if err != nil {
			continue
		}
		if dir := normalizeWindowsDownloadsPath(value, home); dir != "" {
			return dir
		}
	}
	return ""
}

func normalizeWindowsDownloadsPath(value, home string) string {
	value = strings.TrimSpace(strings.Trim(value, `"`))
	if value == "" {
		return ""
	}
	if expanded, err := registry.ExpandString(value); err == nil {
		value = expanded
	}
	value = strings.TrimSpace(strings.Trim(value, `"`))
	if value == "" {
		return ""
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(home, value)
	}
	return filepath.Clean(value)
}
