package analysis

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"ray-player1/internal/logx"
)

type FFmpegConfig struct {
	Path       string
	SampleRate int
	Channels   int
	Timeout    time.Duration
	Start      time.Duration
	Duration   time.Duration
	MaxSeconds int
}

type AudioProbe struct {
	Streams []AudioProbeStream `json:"streams"`
	Format  AudioProbeFormat   `json:"format"`
}

type AudioProbeStream struct {
	CodecName string  `json:"codec_name"`
	Duration  string  `json:"duration"`
	Channels  int     `json:"channels"`
	BitRate   string  `json:"bit_rate"`
	Index     int     `json:"index"`
	Score     float64 `json:"score,omitempty"`
}

type AudioProbeFormat struct {
	Duration string `json:"duration"`
	BitRate  string `json:"bit_rate"`
}

func (p AudioProbe) HasAudio() bool {
	return len(p.Streams) > 0
}

func DefaultFFmpegConfig() FFmpegConfig {
	return FFmpegConfig{Path: FFmpegPath(), SampleRate: 16000, Channels: 1, Timeout: 2 * time.Minute, MaxSeconds: 240}
}

var analysisLog = logx.New("analysis")

func CheckFFmpeg(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = FFmpegPath()
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
	analysisLog.I("available path=%q version=%q", path, firstLine)
	return firstLine, nil
}

func ProbeAudioFile(ctx context.Context, ffprobePath, audioPath string) (AudioProbe, error) {
	if strings.TrimSpace(ffprobePath) == "" {
		ffprobePath = FFprobePath()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		ffprobePath,
		"-v", "warning",
		"-select_streams", "a:0",
		"-show_entries", "stream=index,codec_name,channels,duration,bit_rate:format=duration,bit_rate",
		"-of", "json",
		audioPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	warnings := strings.TrimSpace(stderr.String())
	if warnings != "" {
		analysisLog.W("ffprobe warnings path=%q warnings=%s", audioPath, warnings)
	}
	if ctx.Err() == context.DeadlineExceeded {
		return AudioProbe{}, fmt.Errorf("ffprobe timeout path=%q", audioPath)
	}
	if err != nil {
		return AudioProbe{}, fmt.Errorf("ffprobe failed path=%q err=%w stderr=%s", audioPath, err, warnings)
	}
	if len(out) == 0 {
		return AudioProbe{}, fmt.Errorf("ffprobe returned empty output path=%q", audioPath)
	}
	var probe AudioProbe
	if err := json.Unmarshal(out, &probe); err != nil {
		return AudioProbe{}, fmt.Errorf("ffprobe invalid json path=%q: %w", audioPath, err)
	}
	if !probe.HasAudio() {
		return AudioProbe{}, fmt.Errorf("no audio stream path=%q", audioPath)
	}
	return probe, nil
}

func DecodeMonoFloat32WithFFmpeg(ctx context.Context, audioPath string, cfg FFmpegConfig) ([]float32, int, string, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = FFmpegPath()
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	if cfg.Channels <= 0 {
		cfg.Channels = 1
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	args := []string{"-hide_banner", "-v", "warning", "-nostdin"}
	if cfg.Start > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", cfg.Start.Seconds()))
	}
	args = append(args, "-i", audioPath)
	if cfg.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", cfg.Duration.Seconds()))
	} else if cfg.MaxSeconds > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", cfg.MaxSeconds))
	}
	args = append(args,
		"-map", "0:a:0",
		"-vn",
		"-ac", fmt.Sprintf("%d", cfg.Channels),
		"-ar", fmt.Sprintf("%d", cfg.SampleRate),
		"-f", "f32le",
		"-",
	)

	cmd := exec.CommandContext(ctx, cfg.Path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	warnings := strings.TrimSpace(stderr.String())
	if ctx.Err() == context.DeadlineExceeded {
		return nil, 0, warnings, fmt.Errorf("ffmpeg decode timeout path=%q", audioPath)
	}
	if err != nil {
		return nil, 0, warnings, fmt.Errorf("ffmpeg decode failed path=%q err=%w stderr=%s", audioPath, err, warnings)
	}
	if len(out) == 0 {
		return nil, 0, warnings, fmt.Errorf("ffmpeg decoded empty audio path=%q stderr=%s", audioPath, warnings)
	}
	if len(out)%4 != 0 {
		return nil, 0, warnings, fmt.Errorf("ffmpeg returned invalid f32le byte length=%d path=%q", len(out), audioPath)
	}

	samples := make([]float32, len(out)/4)
	for i := range samples {
		u := binary.LittleEndian.Uint32(out[i*4 : i*4+4])
		x := math.Float32frombits(u)
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			x = 0
		}
		if x > 1 {
			x = 1
		} else if x < -1 {
			x = -1
		}
		samples[i] = x
	}
	analysisLog.I("ffmpeg decoded path=%q samples=%d sampleRate=%d channels=%d duration=%.2fs", audioPath, len(samples), cfg.SampleRate, cfg.Channels, float64(len(samples))/float64(cfg.SampleRate))
	if warnings != "" {
		analysisLog.W("ffmpeg warnings path=%q warnings=%s", audioPath, warnings)
	}
	return samples, cfg.SampleRate, warnings, nil
}

func DecodeAudioDuration(ctx context.Context, path string) (int, error) {
	probe, err := ProbeAudioFile(ctx, FFprobePath(), path)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(probe.Format.Duration)
	if value == "" && len(probe.Streams) > 0 {
		value = strings.TrimSpace(probe.Streams[0].Duration)
	}
	if value == "" {
		return 0, errors.New("ffprobe duration missing")
	}
	seconds, err := time.ParseDuration(value + "s")
	if err != nil {
		return 0, fmt.Errorf("invalid ffprobe duration %q: %w", value, err)
	}
	return int(seconds / time.Millisecond), nil
}
