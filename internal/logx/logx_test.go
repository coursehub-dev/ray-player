package logx

import (
	"bytes"
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
