package analysis

import (
	"path/filepath"
	"strings"
	"sync"
)

var (
	ffprobeMu   sync.RWMutex
	ffprobePath string
)

func SetFFprobePath(path string) {
	path = strings.TrimSpace(path)
	ffprobeMu.Lock()
	defer ffprobeMu.Unlock()
	if path == "" {
		ffprobePath = "ffprobe"
		return
	}
	ffprobePath = path
}

func FFprobePath() string {
	ffprobeMu.RLock()
	path := strings.TrimSpace(ffprobePath)
	ffprobeMu.RUnlock()
	if path != "" {
		return path
	}
	ffmpeg := strings.TrimSpace(FFmpegPath())
	if ffmpeg == "" || ffmpeg == "ffmpeg" {
		return "ffprobe"
	}
	base := filepath.Base(ffmpeg)
	if strings.EqualFold(base, "ffmpeg") {
		return filepath.Join(filepath.Dir(ffmpeg), "ffprobe")
	}
	return "ffprobe"
}
