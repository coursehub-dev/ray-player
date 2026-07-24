package appdirs

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestManagedAssetsLiveUnderAppRoot(t *testing.T) {
	root, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	ffmpegDir, err := ManagedFFmpegDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Clean(ffmpegDir), filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("managed ffmpeg dir %q is outside app root %q", ffmpegDir, root)
	}
	if !strings.Contains(ffmpegDir, runtime.GOOS+"-"+runtime.GOARCH) {
		t.Fatalf("managed ffmpeg dir %q does not include platform", ffmpegDir)
	}
}
