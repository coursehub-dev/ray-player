package audio

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"

	"ray-player1/internal/library"
)

func TestMemoryPCMStreamerSeekPositionLen(t *testing.T) {
	format := beep.Format{SampleRate: 48000, NumChannels: 2, Precision: 4}
	samples := make([][2]float64, 48000)
	s := newMemoryPCMStreamer(samples, format)

	if s.Len() != 48000 {
		t.Fatalf("Len=%d want 48000", s.Len())
	}

	if err := s.Seek(24000); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	if s.Position() != 24000 {
		t.Fatalf("Position=%d want 24000", s.Position())
	}

	buf := make([][2]float64, 512)
	n, ok := s.Stream(buf)
	if n != 512 {
		t.Fatalf("Stream n=%d want 512", n)
	}
	if !ok {
		t.Fatal("Stream ok=false too early")
	}
	if s.Position() != 24512 {
		t.Fatalf("Position=%d want 24512", s.Position())
	}
}

func TestMemoryPCMStreamerSeekClamps(t *testing.T) {
	s := newMemoryPCMStreamer(make([][2]float64, 100), beep.Format{SampleRate: 48000, NumChannels: 2, Precision: 4})

	_ = s.Seek(-10)
	if s.Position() != 0 {
		t.Fatalf("negative seek should clamp to 0, got %d", s.Position())
	}

	_ = s.Seek(999)
	if s.Position() != 100 {
		t.Fatalf("overflow seek should clamp to len, got %d", s.Position())
	}
}

type singleFrameStreamer struct {
	called bool
}

func (s *singleFrameStreamer) Stream(
	samples [][2]float64,
) (int, bool) {
	if s.called || len(samples) == 0 {
		return 0, false
	}
	s.called = true
	samples[0] = [2]float64{0.25, 0.25}
	return 1, true
}

func (s *singleFrameStreamer) Err() error { return nil }
func (s *singleFrameStreamer) Len() int   { return 1 }
func (s *singleFrameStreamer) Position() int {
	return 0
}
func (s *singleFrameStreamer) Seek(int) error {
	return nil
}
func (s *singleFrameStreamer) Close() error {
	return nil
}

func TestPlaybackStreamSignalsFirstSampleOnce(
	t *testing.T,
) {
	var starts atomic.Int32
	service := &Service{
		currentTrack: library.Track{ID: "track-1"},
		onStarted: func(library.Track, string) {
			starts.Add(1)
		},
	}
	atomic.StoreUint64(&service.playToken, 1)

	stream := &playbackStream{
		owner:    service,
		token:    1,
		stream:   &singleFrameStreamer{},
		queuedAt: time.Now(),
	}

	buffer := make([][2]float64, 4)
	stream.Stream(buffer)
	stream.Stream(buffer)

	time.Sleep(10 * time.Millisecond)
	if got := starts.Load(); got != 1 {
		t.Fatalf("onStarted calls = %d, want 1", got)
	}
}
