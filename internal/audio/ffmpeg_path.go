package audio

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	ffmpegPathMu sync.RWMutex
	ffmpegPath   = "ffmpeg"
	ffprobePath  = "ffprobe"
)

func SetFFmpegPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "ffmpeg"
	}
	ffmpegPathMu.Lock()
	ffmpegPath = path
	ffmpegPathMu.Unlock()
}

func SetFFprobePath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "ffprobe"
	}
	ffmpegPathMu.Lock()
	ffprobePath = path
	ffmpegPathMu.Unlock()
}

func FFmpegPath() string {
	ffmpegPathMu.RLock()
	path := strings.TrimSpace(ffmpegPath)
	ffmpegPathMu.RUnlock()
	if path == "" {
		return "ffmpeg"
	}
	return path
}

func FFprobePath() string {
	ffmpegPathMu.RLock()
	path := strings.TrimSpace(ffprobePath)
	ffmpeg := strings.TrimSpace(ffmpegPath)
	ffmpegPathMu.RUnlock()
	// If ffprobe is an explicit path (not a bare command name), use it directly.
	if path != "" && !isBareCommand(path, "ffprobe") {
		return path
	}
	// If ffmpeg is a managed/explicit path (not a bare command), derive ffprobe
	// as a sibling binary in the same directory.
	if ffmpeg != "" && !isBareCommand(ffmpeg, "ffmpeg") {
		name := "ffprobe"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		return filepath.Join(filepath.Dir(ffmpeg), name)
	}
	if path != "" {
		return path
	}
	return "ffprobe"
}

func isBareCommand(value, name string) bool {
	if strings.ContainsAny(value, `/\\`) {
		return false
	}
	base := strings.ToLower(strings.TrimSpace(value))
	return base == name || base == name+".exe"
}
