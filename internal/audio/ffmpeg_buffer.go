package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"time"

	"github.com/gopxl/beep/v2"
)

type memoryPCMStreamer struct {
	samples [][2]float64
	pos     int
	format  beep.Format
	closed  bool
}

func newMemoryPCMStreamer(samples [][2]float64, format beep.Format) *memoryPCMStreamer {
	return &memoryPCMStreamer{samples: samples, format: format}
}

func (s *memoryPCMStreamer) Stream(out [][2]float64) (int, bool) {
	if s.closed {
		return 0, false
	}
	if s.pos >= len(s.samples) {
		return 0, false
	}
	n := copy(out, s.samples[s.pos:])
	s.pos += n
	return n, s.pos < len(s.samples)
}

func (s *memoryPCMStreamer) Err() error { return nil }

func (s *memoryPCMStreamer) Len() int { return len(s.samples) }

func (s *memoryPCMStreamer) Position() int { return s.pos }

func (s *memoryPCMStreamer) Seek(pos int) error {
	if pos < 0 {
		pos = 0
	}
	if pos > len(s.samples) {
		pos = len(s.samples)
	}
	s.pos = pos
	return nil
}

func (s *memoryPCMStreamer) Close() error {
	s.closed = true
	return nil
}

func decodeFFmpegToSeekablePCM(ctx context.Context, path string, sampleRate beep.SampleRate) (beep.StreamSeekCloser, beep.Format, error) {
	if sampleRate <= 0 {
		sampleRate = playbackSampleRate
	}

	args := []string{
		"-hide_banner",
		"-nostdin",
		"-loglevel", "warning",
		"-err_detect", "ignore_err",
		"-i", path,
		"-vn",
		"-ac", "2",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-f", "f32le",
		"-",
	}

	cmd := exec.CommandContext(ctx, FFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, beep.Format{}, fmt.Errorf("ffmpeg decode timeout: %w", ctx.Err())
	}
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("ffmpeg decode failed: %w; stderr=%s", err, stderr.String())
	}
	if len(out) == 0 {
		return nil, beep.Format{}, errors.New("ffmpeg produced empty audio")
	}
	if len(out)%8 != 0 {
		return nil, beep.Format{}, fmt.Errorf("ffmpeg produced invalid stereo f32le byte length=%d", len(out))
	}

	frames := len(out) / 8
	samples := make([][2]float64, frames)
	for i := 0; i < frames; i++ {
		off := i * 8
		l := math.Float32frombits(binary.LittleEndian.Uint32(out[off : off+4]))
		r := math.Float32frombits(binary.LittleEndian.Uint32(out[off+4 : off+8]))
		if math.IsNaN(float64(l)) || math.IsInf(float64(l), 0) {
			l = 0
		}
		if math.IsNaN(float64(r)) || math.IsInf(float64(r), 0) {
			r = 0
		}
		samples[i][0] = float64(l)
		samples[i][1] = float64(r)
	}

	format := beep.Format{SampleRate: sampleRate, NumChannels: 2, Precision: 4}
	audioLog.I("ffmpeg seekable decode ok path=%q frames=%d sr=%d durationMs=%d", path, frames, sampleRate, int((time.Second*time.Duration(frames)/time.Duration(sampleRate))/time.Millisecond))
	return newMemoryPCMStreamer(samples, format), format, nil
}
