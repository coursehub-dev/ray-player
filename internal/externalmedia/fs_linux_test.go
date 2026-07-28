//go:build linux

package externalmedia

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxXDGDownloadDir(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "ray")
	path := filepath.Join(t.TempDir(), "user-dirs.dirs")
	if err := os.WriteFile(path, []byte("XDG_DOWNLOAD_DIR=\"$HOME/Shared Downloads\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Shared Downloads")
	if got := linuxXDGDownloadDir(path, home); got != want {
		t.Fatalf("linuxXDGDownloadDir()=%q want %q", got, want)
	}
}

func TestResolveDownloadDirKeepsSeparateLibraries(t *testing.T) {
	settings := Settings{
		MusicDownloadDir:   filepath.Join("root", "music"),
		PodcastDownloadDir: filepath.Join("root", "podcasts"),
	}
	if got := ResolveDownloadDir(settings, LibraryMusic); got != filepath.Clean(settings.MusicDownloadDir) {
		t.Fatalf("music dir=%q", got)
	}
	if got := ResolveDownloadDir(settings, LibraryPodcast); got != filepath.Clean(settings.PodcastDownloadDir) {
		t.Fatalf("podcast dir=%q", got)
	}
}
