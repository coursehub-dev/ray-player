package externalmedia

import (
	"path/filepath"
	"testing"
)

func TestValidateURL(t *testing.T) {
	valid := []string{
		"https://youtube.com/watch?v=test",
		"http://example.com/audio",
	}
	for _, value := range valid {
		if err := ValidateURL(value); err != nil {
			t.Fatalf("ValidateURL(%q): %v", value, err)
		}
	}

	invalid := []string{
		"",
		"youtube.com/watch?v=test",
		"file:///tmp/test.mp3",
		"javascript:alert(1)",
	}
	for _, value := range invalid {
		if err := ValidateURL(value); err == nil {
			t.Fatalf(
				"ValidateURL(%q) expected error",
				value,
			)
		}
	}
}

func TestParseProgress(t *testing.T) {
	tests := []struct {
		line string
		want float64
	}{
		{
			line: "[download]  42.3% of 4.12MiB at 1.21MiB/s",
			want: 0.423,
		},
		{
			line: "[download] 100.0% of 8.00MiB",
			want: 1,
		},
		{
			line: "[ExtractAudio] Destination: output.mp3",
			want: -1,
		},
	}

	for _, test := range tests {
		got := ParseProgress(test.line)
		if got != test.want {
			t.Fatalf(
				"ParseProgress(%q) = %v, want %v",
				test.line,
				got,
				test.want,
			)
		}
	}
}

func TestBuildDownloadArgs(t *testing.T) {
	client := NewClient("yt-dlp")
	outputDir := filepath.Join("tmp", "downloads")

	args := client.BuildDownloadArgs(DownloadRequest{
		URL:            "https://example.com/watch/1",
		OutputDir:      outputDir,
		OutputTemplate: OutputTemplate(outputDir),
		Bitrate:        128,
		FFmpegPath:     "/tools/ffmpeg",
	})

	required := []string{
		"--no-playlist",
		"--extract-audio",
		"--audio-format",
		"mp3",
		"--audio-quality",
		"128K",
		"--ffmpeg-location",
		"/tools/ffmpeg",
		"https://example.com/watch/1",
	}

	for _, value := range required {
		if !contains(args, value) {
			t.Fatalf(
				"args do not contain %q: %#v",
				value,
				args,
			)
		}
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
