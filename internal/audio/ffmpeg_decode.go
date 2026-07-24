package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"ray-player1/internal/logx"
	"time"
)

type FFmpegConfig struct {
	Path       string
	SampleRate int
	Timeout    time.Duration
	Start      time.Duration
	Duration   time.Duration
}

func DefaultFFmpegConfig() FFmpegConfig {
	return FFmpegConfig{Path: FFmpegPath(), SampleRate: 16000, Timeout: 60 * time.Second}
}

var ffmpegLog = logx.New("ffmpeg")

func CheckFFmpeg(path string) (string, error) {
	if path == "" {
		path = "ffmpeg"
	}
	cmd := exec.Command(path, "-version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg not available path=%q: %w", path, err)
	}
	firstLine := string(out)
	if idx := bytes.IndexByte(out, '\n'); idx >= 0 {
		firstLine = string(out[:idx])
	}
	ffmpegLog.I("available path=%q version=%q", path, firstLine)
	return firstLine, nil
}

func DecodeMonoFloat32WithFFmpeg(ctx context.Context, audioPath string, cfg FFmpegConfig) ([]float32, int, error) {
	if cfg.Path == "" {
		cfg.Path = FFmpegPath()
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	args := []string{"-hide_banner", "-loglevel", "error"}
	if cfg.Start > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", cfg.Start.Seconds()))
	}
	if cfg.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", cfg.Duration.Seconds()))
	}
	args = append(args,
		"-i", audioPath,
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", cfg.SampleRate),
		"-f", "f32le",
		"-",
	)

	cmd := exec.CommandContext(ctx, cfg.Path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, 0, fmt.Errorf("ffmpeg timeout: %w", ctx.Err())
	}
	if err != nil {
		return nil, 0, fmt.Errorf("ffmpeg decode failed: %w; stderr=%s", err, stderr.String())
	}
	if len(out) == 0 {
		return nil, 0, errors.New("ffmpeg produced empty audio")
	}
	if len(out)%4 != 0 {
		return nil, 0, fmt.Errorf("ffmpeg produced invalid f32le byte length=%d", len(out))
	}

	samples := make([]float32, len(out)/4)
	for i := range samples {
		u := binary.LittleEndian.Uint32(out[i*4 : i*4+4])
		x := math.Float32frombits(u)
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			x = 0
		}
		samples[i] = x
	}
	ffmpegLog.I("decoded path=%q samples=%d sampleRate=%d duration=%.2fs", audioPath, len(samples), cfg.SampleRate, float64(len(samples))/float64(cfg.SampleRate))
	return samples, cfg.SampleRate, nil
}
