package externalmedia

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func DefaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	if runtime.GOOS == "linux" {
		configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		if dir := linuxXDGDownloadDir(filepath.Join(configHome, "user-dirs.dirs"), home); dir != "" {
			return dir
		}
	}

	dir := filepath.Join(home, "Downloads")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return os.TempDir()
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
		value := strings.Trim(strings.TrimPrefix(line, "XDG_DOWNLOAD_DIR="), `"`)
		value = strings.ReplaceAll(value, "$HOME", home)
		if value != "" {
			return filepath.Clean(value)
		}
	}
	return ""
}

func ResolveDownloadDir(
	settings Settings,
	kind LibraryType,
) string {
	configured := strings.TrimSpace(settings.MusicDownloadDir)
	if kind == LibraryPodcast {
		configured = strings.TrimSpace(settings.PodcastDownloadDir)
	}
	if configured != "" {
		return filepath.Clean(configured)
	}
	return filepath.Join(DefaultDownloadsDir(), "RayPlayer", "yt-dlp", OutputSubdirectory(kind))
}

func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0o755)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
