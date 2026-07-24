package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
)

const playbackSampleRate = beep.SampleRate(48000)

// MediaKind различает подкасты и музыку для выбора audio filter chain.
type MediaKind string

const (
	MediaMusic   MediaKind = "music"
	MediaPodcast MediaKind = "podcast"
)

// FFmpegStreamOptions управляет параметрами FFmpeg-потока.
type FFmpegStreamOptions struct {
	MediaKind        MediaKind
	NormalizePodcast bool
}

// podcastNormalizationFilter возвращает цепочку аудиофильтров для подкастов:
// highpass → компрессия → loudnorm −16 LUFS / −1.5 dBTP → limiter.
func podcastNormalizationFilter() string {
	return strings.Join([]string{
		"highpass=f=70",
		"acompressor=threshold=-20dB:ratio=3:attack=15:release=180:makeup=3dB:knee=2.5",
		"loudnorm=I=-16:LRA=11:TP=-1.5",
		"alimiter=limit=0.841:attack=5:release=50",
	}, ",")
}

type FFmpegPCMStreamer struct {
	path    string
	options FFmpegStreamOptions

	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr *bytes.Buffer
	cancel context.CancelFunc

	format         beep.Format
	duration       time.Duration
	durationFrames int

	startFrame    int
	decodedFrames int

	closed bool
	err    error
	mu     sync.Mutex
}

func NewFFmpegPCMStreamer(
	ctx context.Context,
	path string,
	sampleRate beep.SampleRate,
	duration time.Duration,
	options ...FFmpegStreamOptions,
) (*FFmpegPCMStreamer, beep.Format, error) {
	if sampleRate <= 0 {
		sampleRate = playbackSampleRate
	}

	format := beep.Format{
		SampleRate:  sampleRate,
		NumChannels: 2,
		Precision:   4,
	}

	opts := FFmpegStreamOptions{MediaKind: MediaMusic}
	if len(options) > 0 {
		opts = options[0]
	}

	streamer := &FFmpegPCMStreamer{
		path:           path,
		options:        opts,
		format:         format,
		duration:       duration,
		durationFrames: sampleRate.N(duration),
	}

	if err := streamer.startLocked(ctx, 0); err != nil {
		return nil, beep.Format{}, err
	}

	return streamer, format, nil
}

func (s *FFmpegPCMStreamer) startLocked(
	parent context.Context,
	startFrame int,
) error {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	start := time.Duration(0)
	if s.format.SampleRate > 0 && startFrame > 0 {
		start = time.Second *
			time.Duration(startFrame) /
			time.Duration(s.format.SampleRate)
	}

	args := []string{
		"-hide_banner",
		"-nostdin",
		"-loglevel",
		"warning",
		"-err_detect",
		"ignore_err",
	}

	if start > 0 {
		args = append(
			args,
			"-ss",
			strconv.FormatFloat(start.Seconds(), 'f', 3, 64),
		)
	}

	args = append(args,
		"-probesize",
		"65536",
		"-analyzeduration",
		"100000",
		"-threads",
		"1",
		"-i",
		s.path,
		"-vn",
		"-sn",
		"-dn",
	)

	if s.options.MediaKind == MediaPodcast && s.options.NormalizePodcast {
		args = append(args, "-af", podcastNormalizationFilter())
	}

	args = append(args,
		"-ac", "2",
		"-ar", fmt.Sprintf("%d", s.format.SampleRate),
		"-f", "f32le",
		"-flush_packets", "1",
		"-",
	)

	cmd := exec.CommandContext(ctx, FFmpegPath(), args...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}

	s.cmd = cmd
	s.stdout = stdout
	s.stderr = stderr
	s.cancel = cancel
	s.startFrame = startFrame
	s.decodedFrames = 0
	s.err = nil

	audioLog.I(
		"ffmpeg stream started path=%q startMs=%d durationMs=%d",
		s.path,
		start.Milliseconds(),
		s.duration.Milliseconds(),
	)
	return nil
}

func (s *FFmpegPCMStreamer) Stream(samples [][2]float64) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.err != nil {
		return 0, false
	}

	frameSize := 8
	need := len(samples) * frameSize
	buf := make([]byte, need)
	n, readErr := io.ReadFull(s.stdout, buf)

	frames := n / frameSize
	for i := 0; i < frames; i++ {
		off := i * frameSize
		l := math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4]))
		r := math.Float32frombits(binary.LittleEndian.Uint32(buf[off+4 : off+8]))
		if math.IsNaN(float64(l)) || math.IsInf(float64(l), 0) {
			l = 0
		}
		if math.IsNaN(float64(r)) || math.IsInf(float64(r), 0) {
			r = 0
		}
		samples[i][0] = float64(l)
		samples[i][1] = float64(r)
	}
	s.decodedFrames += frames

	if readErr == nil {
		return frames, true
	}
	if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
		return frames, false
	}

	s.err = readErr
	audioLog.I("ffmpeg stream error err=%v", readErr)
	return frames, false
}

func (s *FFmpegPCMStreamer) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *FFmpegPCMStreamer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.durationFrames
}

func (s *FFmpegPCMStreamer) Position() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startFrame + s.decodedFrames
}

func (s *FFmpegPCMStreamer) Seek(pos int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("ffmpeg streamer is closed")
	}

	if pos < 0 {
		pos = 0
	}
	if s.durationFrames > 0 && pos > s.durationFrames {
		pos = s.durationFrames
	}

	s.stopProcessLocked()

	if err := s.startLocked(context.Background(), pos); err != nil {
		s.err = err
		return err
	}
	return nil
}

func (s *FFmpegPCMStreamer) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	err := s.stopProcessLocked()
	s.mu.Unlock()
	return err
}

func (s *FFmpegPCMStreamer) stopProcessLocked() error {
	stdout := s.stdout
	cmd := s.cmd
	cancel := s.cancel
	stderr := s.stderr

	s.stdout = nil
	s.cmd = nil
	s.cancel = nil
	s.stderr = nil

	if cancel != nil {
		cancel()
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	err := cmd.Wait()

	stderrStr := ""
	if stderr != nil {
		stderrStr = stderr.String()
	}

	if err != nil && !isExpectedFFmpegStop(err) {
		return fmt.Errorf("ffmpeg playback failed: %w; stderr=%s", err, stderrStr)
	}
	if strings.TrimSpace(stderrStr) != "" {
		audioLog.I("ffmpeg warnings stderr=%s", stderrStr)
	}
	return nil
}

func isExpectedFFmpegStop(err error) bool {
	if err == nil {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "signal: killed") ||
		strings.Contains(text, "signal: interrupt") ||
		strings.Contains(text, "context canceled") ||
		strings.Contains(text, "exit status 255")
}
