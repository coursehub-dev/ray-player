//go:build linux

package externalmedia

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func platformDownloadsDir(home string) string {
	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return linuxXDGDownloadDir(filepath.Join(configHome, "user-dirs.dirs"), home)
}

func linuxXDGDownloadDir(path, home string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "XDG_DOWNLOAD_DIR=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "XDG_DOWNLOAD_DIR="))
		value = strings.Trim(value, `"`)
		value = strings.ReplaceAll(value, "${HOME}", home)
		value = strings.ReplaceAll(value, "$HOME", home)
		value = strings.ReplaceAll(value, `\\`, `\`)
		value = strings.ReplaceAll(value, `\"`, `"`)
		if value == "" {
			return ""
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(home, value)
		}
		return filepath.Clean(value)
	}
	return ""
}
