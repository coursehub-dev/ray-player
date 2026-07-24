package audio

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestFFprobePathFollowsManagedFFmpegSibling(t *testing.T) {
	oldFFmpeg, oldFFprobe := FFmpegPath(), FFprobePath()
	t.Cleanup(func() {
		SetFFmpegPath(oldFFmpeg)
		SetFFprobePath(oldFFprobe)
	})
	name := "ffmpeg"
	probe := "ffprobe"
	if runtime.GOOS == "windows" {
		name += ".exe"
		probe += ".exe"
	}
	SetFFmpegPath(filepath.Join("tmp", "managed", name))
	SetFFprobePath("ffprobe")
	want := filepath.Join("tmp", "managed", probe)
	if got := FFprobePath(); got != want {
		t.Fatalf("FFprobePath=%q want=%q", got, want)
	}
}
