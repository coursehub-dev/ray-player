package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerFormatsModule(t *testing.T) {
	prevOut, prevColor, prevLevel := out, useColor, currentLevel
	defer func() {
		out = prevOut
		useColor = prevColor
		currentLevel = prevLevel
	}()
	var buf bytes.Buffer
	out = &buf
	useColor = false
	currentLevel = LevelTrace

	New("TestMod").I("hello %d", 7)
	line := buf.String()
	if !strings.Contains(line, "| INFO") {
		t.Fatalf("missing level: %q", line)
	}
	if !strings.Contains(line, "| TestMod") {
		t.Fatalf("missing module: %q", line)
	}
	if !strings.Contains(line, "hello 7") {
		t.Fatalf("missing message: %q", line)
	}
}

func TestLoggerTrimsModulePrefixAndPadsLevel(t *testing.T) {
	prevOut, prevColor, prevLevel := out, useColor, currentLevel
	defer func() {
		out = prevOut
		useColor = prevColor
		currentLevel = prevLevel
	}()
	var buf bytes.Buffer
	out = &buf
	useColor = false
	currentLevel = LevelTrace

	New("db").I("[db] scan track id=abc")
	line := buf.String()
	if !strings.Contains(line, "| INFO ") {
		t.Fatalf("expected padded level: %q", line)
	}
	if !strings.Contains(line, "| db |") {
		t.Fatalf("expected compact module column: %q", line)
	}
	if strings.Contains(line, "[db] scan track") {
		t.Fatalf("expected trimmed prefix: %q", line)
	}
	if !strings.Contains(line, "scan track id=abc") {
		t.Fatalf("missing payload: %q", line)
	}
}

func TestConfigureFileWritesPlainLog(t *testing.T) {
	prevOut, prevColor, prevLevel := out, useColor, currentLevel
	defer func() {
		_ = CloseFile()
		out = prevOut
		useColor = prevColor
		currentLevel = prevLevel
	}()

	var console bytes.Buffer
	out = &console
	useColor = true
	currentLevel = LevelTrace

	path := filepath.Join(t.TempDir(), "ray.log")
	got, err := ConfigureFile(path)
	if err != nil {
		t.Fatalf("ConfigureFile: %v", err)
	}
	if got != path {
		t.Fatalf("path=%q want=%q", got, path)
	}

	New("ml").W("bad score %.2f", 1.2)
	if err := CloseFile(); err != nil {
		t.Fatalf("CloseFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "| WARN  | ml | bad score 1.20") {
		t.Fatalf("missing file log payload: %q", text)
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("file log must not contain ANSI: %q", text)
	}
}

func TestRotateLogIfNeeded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ray.log")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rotateLogIfNeeded(path, 5); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("active file should be moved, err=%v", err)
	}
}
