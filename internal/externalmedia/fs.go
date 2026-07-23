package externalmedia

import (
	"os"
	"path/filepath"
)

func DefaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}

	dir := filepath.Join(home, "Downloads")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return os.TempDir()
}

func ResolveDownloadBase(settings Settings) string {
	base := settings.YtDlpDownloadDir
	if base == "" {
		base = DefaultDownloadsDir()
	}
	return filepath.Join(base, "RayPlayer", "yt-dlp")
}

func ResolveDownloadDir(
	settings Settings,
	kind LibraryType,
) string {
	return filepath.Join(
		ResolveDownloadBase(settings),
		OutputSubdirectory(kind),
	)
}

func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0o755)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
