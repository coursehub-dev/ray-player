package audio

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
)

func looksLikeMP3FrameHeader(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	if b[0] != 0xFF || (b[1]&0xE0) != 0xE0 {
		return false
	}

	version := (b[1] >> 3) & 0x03
	layer := (b[1] >> 1) & 0x03
	bitrate := (b[2] >> 4) & 0x0F
	sampleRate := (b[2] >> 2) & 0x03

	if version == 0x01 {
		return false
	}
	if layer == 0x00 {
		return false
	}
	if bitrate == 0x00 || bitrate == 0x0F {
		return false
	}
	if sampleRate == 0x03 {
		return false
	}
	return true
}

func findMP3FrameOffset(f *os.File, maxScan int64) (int64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	buf := make([]byte, maxScan)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, err
	}
	buf = buf[:n]

	for i := 0; i+4 < len(buf); i++ {
		if looksLikeMP3FrameHeader(buf[i : i+4]) {
			return int64(i), nil
		}
	}

	return 0, errors.New("mp3 frame sync not found")
}

type cancelCloser struct {
	cancel context.CancelFunc
	closer io.Closer
}

func (c *cancelCloser) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}

func decodeMP3Robust(path string) (beep.StreamSeekCloser, beep.Format, io.Closer, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, nil, "", err
	}
	r := newBufferedReadSeeker(f, 512*1024)
	s, format, err := mp3.Decode(r)
	if err == nil {
		if probeErr := probeSeekableStream(s); probeErr == nil {
			if validateErr := validateFormat(format); validateErr == nil {
				return s, format, s, "beep/mp3", nil
			}
			_ = s.Close()
			audioLog.I("mp3 probe/format failed path=%q err=%v", path, probeErr)
		} else {
			_ = s.Close()
			audioLog.I("mp3 probe failed path=%q err=%v", path, probeErr)
		}
	} else {
		_ = r.Close()
		audioLog.I("mp3 normal decode failed path=%q err=%v", path, err)
	}

	f2, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, nil, "", err
	}
	offset, scanErr := findMP3FrameOffset(f2, 10*1024*1024)
	if scanErr == nil && offset > 0 {
		if _, seekErr := f2.Seek(offset, io.SeekStart); seekErr == nil {
			r2 := newBufferedReadSeeker(f2, 512*1024)
			s2, format2, err2 := mp3.Decode(r2)
			if err2 == nil {
				if probeErr := probeSeekableStream(s2); probeErr == nil {
					if validateErr := validateFormat(format2); validateErr == nil {
						audioLog.I("mp3 decoded after frame scan path=%q offset=%d", path, offset)
						return s2, format2, s2, "beep/mp3-frame-scan", nil
					}
					_ = s2.Close()
				} else {
					_ = s2.Close()
				}
				audioLog.I("mp3 frame-scan probe failed path=%q offset=%d err=%v", path, offset, err2)
			} else {
				_ = r2.Close()
				audioLog.I("mp3 frame-scan decode failed path=%q offset=%d err=%v", path, offset, err2)
			}
		}
	} else {
		_ = f2.Close()
	}

	return decodeWithFFmpegForPlayback(path, "mp3")
}
