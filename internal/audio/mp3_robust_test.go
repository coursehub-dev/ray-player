package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gopxl/beep/v2"
)

func TestLooksLikeMP3FrameHeader(t *testing.T) {
	valid := []byte{0xFF, 0xFB, 0x90, 0x00}
	if !looksLikeMP3FrameHeader(valid) {
		t.Fatal("expected valid MPEG frame header")
	}
	invalid := []byte{0x49, 0x44, 0x33, 0x04}
	if looksLikeMP3FrameHeader(invalid) {
		t.Fatal("expected ID3 bytes to be rejected")
	}
}

func TestFindMP3FrameOffsetSkipsID3(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tagged.mp3")
	payload := append([]byte("ID3\x04\x00\x00\x00\x00\x00\x16"), []byte{0xFF, 0xFB, 0x90, 0x00, 0x01, 0x02, 0x03, 0x04}...)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	offset, err := findMP3FrameOffset(f, 1024)
	if err != nil {
		t.Fatalf("find offset: %v", err)
	}
	if offset != 10 {
		t.Fatalf("expected offset 10, got %d", offset)
	}
}

func TestValidateFormat(t *testing.T) {
	if err := validateFormat(beep.Format{SampleRate: 48000, NumChannels: 2, Precision: 4}); err != nil {
		t.Fatalf("valid format rejected: %v", err)
	}
	if err := validateFormat(beep.Format{SampleRate: 0, NumChannels: 2, Precision: 4}); err == nil {
		t.Fatal("expected invalid sample rate error")
	}
}

func TestClassifyPlaybackEndReason(t *testing.T) {
	cases := []struct {
		name      string
		samples   int64
		playedMs  int
		hasFirst  bool
		streamErr bool
		want      PlaybackEndReason
	}{
		{"natural", 100000, 5000, true, false, PlaybackEndNatural},
		{"stream error", 1000, 100, true, true, PlaybackEndStreamError},
		{"empty", 0, 0, false, false, PlaybackEndEmptyStream},
		{"too short", 1000, 500, true, false, PlaybackEndEmptyStream},
	}
	for _, tc := range cases {
		got := classifyPlaybackEnd(tc.samples, tc.playedMs, tc.hasFirst, tc.streamErr)
		if got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}
