package analysis

import (
	"strings"
	"sync"
)

var (
	ffmpegMu   sync.RWMutex
	ffmpegPath = "ffmpeg"
)

func SetFFmpegPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "ffmpeg"
	}
	ffmpegMu.Lock()
	ffmpegPath = path
	ffmpegMu.Unlock()
}

func FFmpegPath() string {
	ffmpegMu.RLock()
	path := strings.TrimSpace(ffmpegPath)
	ffmpegMu.RUnlock()
	if path == "" {
		return "ffmpeg"
	}
	return path
}
