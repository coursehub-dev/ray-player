package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
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
	currentLevel           = parseLevel(os.Getenv("RAY_LOG"))
	useColor               = os.Getenv("NO_COLOR") == ""
)

func New(module string) Logger { return Logger{module: strings.TrimSpace(module)} }

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
	levelName := levelString(level)
	ts := time.Now().Format("15:04:05.000")
	if useColor {
		levelName = colorFor(level) + levelName + colorReset
	}
	line := fmt.Sprintf("%s | %-5s | %s | %s\n", ts, levelName, module, msg)
	mu.Lock()
	defer mu.Unlock()
	_, _ = out.Write([]byte(line))
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
