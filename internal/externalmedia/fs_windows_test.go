//go:build windows

package externalmedia

import (
	"path/filepath"
	"testing"
)

func TestNormalizeWindowsDownloadsPath(t *testing.T) {
	home := `C:\Users\Ray`
	t.Setenv("USERPROFILE", home)
	if got := normalizeWindowsDownloadsPath(`Downloads`, home); got != filepath.Join(home, "Downloads") {
		t.Fatalf("relative path=%q", got)
	}
	if got := normalizeWindowsDownloadsPath(`D:\Media\Incoming`, home); got != filepath.Clean(`D:\Media\Incoming`) {
		t.Fatalf("absolute path=%q", got)
	}
	if got := normalizeWindowsDownloadsPath(`%USERPROFILE%\Downloads`, home); got != filepath.Join(home, "Downloads") {
		t.Fatalf("expanded path=%q", got)
	}
}
