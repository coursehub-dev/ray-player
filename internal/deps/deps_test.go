package deps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rayonnx "ray-player1/internal/onnx"
)

func TestApplyPatchOnlyOverridesReturnedValues(t *testing.T) {
	base := Settings{
		ONNXRuntimePath:  "/old/ort",
		MiniLMModelDir:   "/old/minilm",
		EssentiaModelDir: "/old/essentia",
		FFmpegPath:       "/old/ffmpeg",
		FFprobePath:      "/old/ffprobe",
	}
	got := applyPatch(base, SettingsPatch{
		MiniLMModelDir:   "/new/minilm",
		EssentiaModelDir: "/new/essentia",
		FFmpegPath:       "/new/ffmpeg",
	})
	if got.ONNXRuntimePath != base.ONNXRuntimePath || got.FFprobePath != base.FFprobePath {
		t.Fatalf("unrelated settings changed: %#v", got)
	}
	if got.MiniLMModelDir != "/new/minilm" || got.EssentiaModelDir != "/new/essentia" || got.FFmpegPath != "/new/ffmpeg" {
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

func TestEnsureEssentiaDownloadsCompleteManagedBundle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if filepath.Ext(name) == ".json" {
			_, _ = w.Write([]byte(`{"classes":[]}`))
			return
		}
		_, _ = w.Write([]byte("onnx-fixture"))
	}))
	defer server.Close()

	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	t.Setenv(essentiaModelBaseURLEnv, server.URL)

	dir, err := EnsureEssentia(context.Background(), "")
	if err != nil {
		t.Fatalf("EnsureEssentia: %v", err)
	}
	for _, name := range rayonnx.RequiredEssentiaFiles() {
		if !regularFile(filepath.Join(dir, name)) {
			t.Fatalf("downloaded bundle is missing %s", name)
		}
	}
}
func TestEnsureEssentiaUsesConfiguredBundleWithoutDownloading(t *testing.T) {
	dir := t.TempDir()
	for _, name := range rayonnx.RequiredEssentiaFiles() {
		content := []byte("onnx-fixture")
		if filepath.Ext(name) == ".json" {
			content = []byte(`{"classes":[]}`)
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(essentiaModelBaseURLEnv, "http://127.0.0.1:1")

	got, err := EnsureEssentia(context.Background(), dir)
	if err != nil {
		t.Fatalf("EnsureEssentia: %v", err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("dir=%q want=%q", got, want)
	}
}
