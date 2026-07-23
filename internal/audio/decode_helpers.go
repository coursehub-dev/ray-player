package audio

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

func validateFormat(format beep.Format) error {
	if format.SampleRate <= 0 {
		return errors.New("invalid sample rate")
	}
	if format.NumChannels <= 0 || format.NumChannels > 8 {
		return fmt.Errorf("invalid channels=%d", format.NumChannels)
	}
	if format.Precision <= 0 {
		return fmt.Errorf("invalid precision=%d", format.Precision)
	}
	return nil
}

func probeSeekableStream(s beep.StreamSeekCloser) error {
	buf := make([][2]float64, 2048)
	n, ok := s.Stream(buf)
	err := s.Err()

	if seekErr := s.Seek(0); seekErr != nil {
		return fmt.Errorf("probe seek-back failed: %w", seekErr)
	}
	if err != nil {
		return fmt.Errorf("probe stream error: %w", err)
	}
	if n == 0 && !ok {
		return errors.New("probe got empty stream")
	}
	return nil
}

func safeDecode(fn func() (beep.StreamSeekCloser, beep.Format, io.Closer, string, error)) (s beep.StreamSeekCloser, f beep.Format, c io.Closer, backend string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("decoder panic: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}

func decodeWithFFmpegForPlayback(path, label string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	return decodeWithFFmpegOptions(path, label, FFmpegStreamOptions{MediaKind: MediaMusic})
}

func decodeWithFFmpegForPlaybackNormalized(path, label string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	return decodeWithFFmpegOptions(path, label, FFmpegStreamOptions{
		MediaKind:        MediaPodcast,
		NormalizePodcast: true,
	})
}

func decodeWithFFmpegOptions(path, label string, options FFmpegStreamOptions) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	if _, err := CheckFFmpeg(FFmpegPath()); err != nil {
		return nil, beep.Format{}, nil, "", fmt.Errorf("unsupported playback format: %s requires ffmpeg: %w", label, err)
	}

	audioLog.I("fallback start path=%q format=%s", path, label)

	duration, durationErr := ProbeDuration(path)
	if durationErr != nil {
		audioLog.I(
			"ffprobe duration failed path=%q err=%v",
			path,
			durationErr,
		)
	}

	ff, ffFormat, ffErr := NewFFmpegPCMStreamer(
		context.Background(),
		path,
		playbackSampleRate,
		duration,
		options,
	)
	if ffErr != nil {
		return nil, beep.Format{}, nil, "", ffErr
	}
	if validateErr := validateFormat(ffFormat); validateErr != nil {
		_ = ff.Close()
		return nil, beep.Format{}, nil, "", validateErr
	}

	audioLog.I(
		"fallback ok path=%q backend=ffmpeg-stream format=%s len=%d",
		path,
		label,
		ff.Len(),
	)
	return ff, ffFormat, ff, "ffmpeg-stream", nil
}

func decodeBeepFormat(path string, decodeFn func(io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error), backend string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, nil, "", err
	}
	r := newBufferedReadSeeker(f, 512*1024)
	s, format, err := decodeFn(r)
	if err != nil {
		_ = r.Close()
		return nil, beep.Format{}, nil, "", err
	}
	if probeErr := probeSeekableStream(s); probeErr != nil {
		_ = s.Close()
		audioLog.I("probe failed path=%q backend=%s err=%v", path, backend, probeErr)
		return nil, beep.Format{}, nil, "", probeErr
	}
	if validateErr := validateFormat(format); validateErr != nil {
		_ = s.Close()
		return nil, beep.Format{}, nil, "", validateErr
	}
	return s, format, s, backend, nil
}

func decodeWAV(path string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	s, format, closer, backend, err := decodeBeepFormat(path, func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return wav.Decode(r)
	}, "beep/wav")
	if err == nil {
		return s, format, closer, backend, nil
	}
	audioLog.I("wav beep decode failed path=%q err=%v", path, err)
	return decodeWithFFmpegForPlayback(path, "wav")
}

func decodeFLAC(path string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	s, format, closer, backend, err := decodeBeepFormat(path, func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return flac.Decode(r)
	}, "beep/flac")
	if err == nil {
		return s, format, closer, backend, nil
	}
	audioLog.I("flac beep decode failed path=%q err=%v", path, err)
	return decodeWithFFmpegForPlayback(path, "flac")
}

func decodeOGG(path string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	s, format, closer, backend, err := decodeBeepFormat(path, func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return vorbis.Decode(r)
	}, "beep/vorbis")
	if err == nil {
		return s, format, closer, backend, nil
	}
	audioLog.I("ogg beep decode failed path=%q err=%v", path, err)
	return decodeWithFFmpegForPlayback(path, "ogg")
}

func DecodeForPlayback(path string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	return DecodeForPlaybackWithOptions(path, false)
}

// DecodeForPlaybackWithOptions декодирует аудиофайл с опциональной нормализацией для подкастов.
func DecodeForPlaybackWithOptions(path string, normalizePodcast bool) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	ext := strings.ToLower(path)
	if normalizePodcast {
		// Для нормализованного воспроизведения всегда используем FFmpeg с filter chain.
		return decodeWithFFmpegForPlaybackNormalized(path, filepathExt(ext))
	}
	switch {
	case strings.HasSuffix(ext, ".mp3"):
		return decodeMP3Robust(path)
	case strings.HasSuffix(ext, ".m4a"), strings.HasSuffix(ext, ".aac"):
		return decodeWithFFmpegForPlayback(path, "m4a/aac")
	case strings.HasSuffix(ext, ".wma"), strings.HasSuffix(ext, ".opus"):
		return decodeWithFFmpegForPlayback(path, filepathExt(ext))
	case strings.HasSuffix(ext, ".wav"):
		return decodeWAV(path)
	case strings.HasSuffix(ext, ".flac"):
		return decodeFLAC(path)
	case strings.HasSuffix(ext, ".ogg"):
		return decodeOGG(path)
	default:
		return decodeWithFFmpegForPlayback(path, "auto")
	}
}

func filepathExt(ext string) string {
	if i := strings.LastIndex(ext, "."); i >= 0 {
		return ext[i+1:]
	}
	return ext
}

type bufferedReadSeeker struct {
	f *os.File
	r *bufio.Reader
}

func newBufferedReadSeeker(f *os.File, size int) *bufferedReadSeeker {
	return &bufferedReadSeeker{f: f, r: bufio.NewReaderSize(f, size)}
}

func (b *bufferedReadSeeker) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func (b *bufferedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	pos, err := b.f.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	b.r.Reset(b.f)
	return pos, nil
}

func (b *bufferedReadSeeker) Close() error {
	return b.f.Close()
}
