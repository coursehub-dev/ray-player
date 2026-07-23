package audio

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
)

func TestFFmpegPCMStreamerReportsPositionAndLength(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}

	path := filepath.Join(t.TempDir(), "tone.m4a")
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=4",
		"-c:a", "aac",
		path,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create fixture: %v: %s", err, output)
	}

	duration, err := ProbeDuration(path)
	if err != nil {
		t.Fatalf("probe duration: %v", err)
	}
	if duration < 3*time.Second {
		t.Fatalf("duration = %v, want about 4s", duration)
	}

	streamer, _, err := NewFFmpegPCMStreamer(
		context.Background(),
		path,
		beep.SampleRate(48000),
		duration,
	)
	if err != nil {
		t.Fatalf("new streamer: %v", err)
	}
	defer streamer.Close()

	buffer := make([][2]float64, 4096)
	n, ok := streamer.Stream(buffer)
	if n <= 0 || !ok {
		t.Fatalf("stream returned n=%d ok=%v", n, ok)
	}
	if streamer.Position() <= 0 {
		t.Fatal("position must advance after streaming")
	}
	if streamer.Len() <= 0 {
		t.Fatal("length must be known")
	}

	target := 2 * 48000
	if err := streamer.Seek(target); err != nil {
		t.Fatalf("seek: %v", err)
	}
	if got := streamer.Position(); got != target {
		t.Fatalf("position after seek = %d, want %d", got, target)
	}

	n, _ = streamer.Stream(buffer)
	if n <= 0 {
		t.Fatal("stream must continue after seek")
	}
	if streamer.Position() <= target {
		t.Fatal("position must advance from seek target")
	}
}
