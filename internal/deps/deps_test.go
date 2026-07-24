package deps

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplyPatchOnlyOverridesReturnedValues(t *testing.T) {
	base := Settings{
		ONNXRuntimePath: "/old/ort",
		MiniLMModelDir:  "/old/minilm",
		FFmpegPath:      "/old/ffmpeg",
		FFprobePath:     "/old/ffprobe",
	}
	got := applyPatch(base, SettingsPatch{
		MiniLMModelDir: "/new/minilm",
		FFmpegPath:     "/new/ffmpeg",
	})
	if got.ONNXRuntimePath != base.ONNXRuntimePath || got.FFprobePath != base.FFprobePath {
		t.Fatalf("unrelated settings changed: %#v", got)
	}
	if got.MiniLMModelDir != "/new/minilm" || got.FFmpegPath != "/new/ffmpeg" {
		t.Fatalf("patch was not applied: %#v", got)
	}
}

func TestResolveFFmpegToolsExplicitPair(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := filepath.Join(dir, executableName("ffmpeg"))
	ffprobe := filepath.Join(dir, executableName("ffprobe"))
	for _, path := range []string{ffmpeg, ffprobe} {
		if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gotFFmpeg, gotFFprobe, err := ResolveFFmpegTools(ffmpeg, "ffprobe")
	if err != nil {
		t.Fatalf("ResolveFFmpegTools: %v", err)
	}
	if gotFFmpeg != ffmpeg || gotFFprobe != ffprobe {
		t.Fatalf("got (%q,%q), want (%q,%q)", gotFFmpeg, gotFFprobe, ffmpeg, ffprobe)
	}
}

func TestFFmpegAssetManifestsCoverDesktopTargets(t *testing.T) {
	targets := [][2]string{
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}
	for _, target := range targets {
		goos, goarch := target[0], target[1]
		t.Run(goos+"-"+goarch, func(t *testing.T) {
			ff, ok := ffmpegAssetFor(goos, goarch)
			if !ok {
				t.Fatal("ffmpeg asset missing")
			}
			if ff.ZipURL == "" && (ff.FFmpegURL == "" || ff.FFprobeURL == "") {
				t.Fatalf("incomplete ffmpeg asset: %#v", ff)
			}
			for _, raw := range []string{ff.ZipURL, ff.FFmpegURL, ff.FFprobeURL} {
				if raw != "" && !strings.HasPrefix(raw, "https://") {
					t.Fatalf("dependency URL is not HTTPS: %q", raw)
				}
			}
		})
	}
}

func TestONNXRuntimeAssetManifestsCoverPublishedTargets(t *testing.T) {
	targets := [][2]string{
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}
	for _, target := range targets {
		goos, goarch := target[0], target[1]
		t.Run(goos+"-"+goarch, func(t *testing.T) {
			ort, ok := OnnxRuntimeAsset(goos, goarch)
			if !ok || ort.Archive == "" || ort.Library == "" || ort.Member == "" {
				t.Fatalf("incomplete ONNX Runtime asset: %#v ok=%v", ort, ok)
			}
		})
	}
	if _, ok := OnnxRuntimeAsset("darwin", "amd64"); ok {
		t.Fatal("ONNX Runtime 1.26 has no official macOS x86_64 release asset")
	}
}

func TestCurrentPlatformManifestIsKnownWhenSupportedByApplication(t *testing.T) {
	if _, ok := OnnxRuntimeAsset(runtime.GOOS, runtime.GOARCH); !ok {
		t.Skipf("current target is intentionally unsupported: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if _, ok := ffmpegAssetFor(runtime.GOOS, runtime.GOARCH); !ok {
		t.Fatalf("ONNX Runtime is supported but ffmpeg manifest is missing for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}
