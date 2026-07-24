package logx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/appdirs"
)

type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
)

const colorReset = "[0m"

type Logger struct {
	module string
}

var (
	mu           sync.Mutex
	out          io.Writer = os.Stdout
	fileOut      io.Writer
	fileHandle   *os.File
	currentLevel = parseLevel(os.Getenv("RAY_LOG"))
	useColor     = os.Getenv("NO_COLOR") == ""
)

func New(module string) Logger { return Logger{module: strings.TrimSpace(module)} }

const defaultMaxLogBytes int64 = 10 << 20

func ConfigureFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("RAY_LOG_FILE"))
	}
	if path == "" {
		base, err := appdirs.DefaultRoot()
		if err != nil {
			return "", err
		}
		path = filepath.Join(base, "logs", "ray-player.log")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve log path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", fmt.Errorf("create log directory: %w", err)
	}
	if err := rotateLogIfNeeded(absolute, defaultMaxLogBytes); err != nil {
		return "", err
	}
	fh, err := os.OpenFile(absolute, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("open log file: %w", err)
	}
	mu.Lock()
	old := fileHandle
	fileHandle = fh
	fileOut = fh
	mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return absolute, nil
}

func CloseFile() error {
	mu.Lock()
	fh := fileHandle
	fileHandle = nil
	fileOut = nil
	mu.Unlock()
	if fh == nil {
		return nil
	}
	return fh.Close()
}

func rotateLogIfNeeded(path string, maxBytes int64) error {
	if maxBytes <= 0 {
		return nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat log file: %w", err)
	}
	if info.Size() < maxBytes {
		return nil
	}
	backup := path + ".1"
	_ = os.Remove(backup)
	if err := os.Rename(path, backup); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return nil
}

func T(format string, args ...any) { logf(LevelTrace, "", format, args...) }
func D(format string, args ...any) { logf(LevelDebug, "", format, args...) }
func I(format string, args ...any) { logf(LevelInfo, "", format, args...) }
func W(format string, args ...any) { logf(LevelWarn, "", format, args...) }
func E(format string, args ...any) { logf(LevelError, "", format, args...) }

func Tracef(format string, args ...any) { T(format, args...) }
func Debugf(format string, args ...any) { D(format, args...) }
func Infof(format string, args ...any)  { I(format, args...) }
func Warnf(format string, args ...any)  { W(format, args...) }
func Errorf(format string, args ...any) { E(format, args...) }

func (l Logger) T(format string, args ...any) { logf(LevelTrace, l.module, format, args...) }
func (l Logger) D(format string, args ...any) { logf(LevelDebug, l.module, format, args...) }
func (l Logger) I(format string, args ...any) { logf(LevelInfo, l.module, format, args...) }
func (l Logger) W(format string, args ...any) { logf(LevelWarn, l.module, format, args...) }
func (l Logger) E(format string, args ...any) { logf(LevelError, l.module, format, args...) }

func logf(level Level, module string, format string, args ...any) {
	if level < currentLevel {
		return
	}
	if strings.TrimSpace(module) == "" {
		module = inferModule()
	}
	msg := fmt.Sprintf(format, args...)
	msg = trimModulePrefix(msg, module)
	plainLevel := levelString(level)
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	plainLine := fmt.Sprintf("%s | %-5s | %s | %s\n", ts, plainLevel, module, msg)
	consoleLevel := plainLevel
	if useColor {
		consoleLevel = colorFor(level) + plainLevel + colorReset
	}
	consoleLine := fmt.Sprintf("%s | %-5s | %s | %s\n", ts, consoleLevel, module, msg)

	mu.Lock()
	defer mu.Unlock()
	_, _ = out.Write([]byte(consoleLine))
	if fileOut != nil {
		_, _ = fileOut.Write([]byte(plainLine))
	}
}

func trimModulePrefix(msg, module string) string {
	module = strings.TrimSpace(module)
	if module == "" {
		return msg
	}
	prefix := "[" + module + "] "
	return strings.TrimSpace(strings.TrimPrefix(msg, prefix))
}

func parseLevel(v string) Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "info", "":
		return LevelInfo
	default:
		return LevelInfo
	}
}

func levelString(level Level) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

func colorFor(level Level) string {
	switch level {
	case LevelTrace:
		return "[90m"
	case LevelDebug:
		return "[36m"
	case LevelInfo:
		return "[32m"
	case LevelWarn:
		return "[33m"
	case LevelError:
		return "[31m"
	default:
		return ""
	}
}

func inferModule() string {
	if _, file, _, ok := runtime.Caller(3); ok {
		base := filepath.Base(file)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return "App"
}
