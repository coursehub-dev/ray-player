package externalmedia

import (
	"os"
	"path/filepath"
	"strings"
)

func DefaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return os.TempDir()
	}
	if dir := strings.TrimSpace(platformDownloadsDir(home)); dir != "" {
		return filepath.Clean(dir)
	}

	// Do not fall back to the system temp directory merely because Downloads
	// has not been created yet. The yt-dlp worker creates the final directory.
	return filepath.Join(home, "Downloads")
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
