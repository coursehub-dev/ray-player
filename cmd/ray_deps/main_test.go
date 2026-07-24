package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestONNXRuntimeAssetMatchesSupportedPlatforms(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"darwin", "arm64", "osx-arm64-1.26.0.tgz"},
		{"linux", "amd64", "linux-x64-1.26.0.tgz"},
		{"linux", "arm64", "linux-aarch64-1.26.0.tgz"},
		{"windows", "amd64", "win-x64-1.26.0.zip"},
		{"windows", "arm64", "win-arm64-1.26.0.zip"},
	}
	for _, tc := range tests {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			asset, ok := onnxRuntimeAsset(tc.goos, tc.goarch)
			if !ok {
				t.Fatal("expected supported runtime asset")
			}
			if !strings.Contains(asset.Archive, tc.want) {
				t.Fatalf("archive=%q want substring %q", asset.Archive, tc.want)
			}
			if asset.Library == "" || asset.Member == "" {
				t.Fatalf("incomplete asset: %+v", asset)
			}
		})
	}
	if _, ok := onnxRuntimeAsset("darwin", "amd64"); ok {
		t.Fatal("darwin/amd64 must require an explicit/system runtime instead of a guessed archive")
	}
}

func TestExtractTarGzipMember(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "runtime.tgz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("runtime")
	if err := tw.WriteHeader(&tar.Header{Name: "onnxruntime/lib/libonnxruntime.so.1.26.0", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "libonnxruntime.so.1.26.0")
	if err := extractTarGzipMember(archive, "/lib/libonnxruntime.so.1.26.0", dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "runtime" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractZipMember(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "runtime.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("onnxruntime/lib/onnxruntime.dll")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("dll")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "onnxruntime.dll")
	if err := extractZipMember(archive, "/lib/onnxruntime.dll", dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "dll" {
		t.Fatalf("got %q", got)
	}
}

func TestHuggingFaceURLUsesOfficialRepository(t *testing.T) {
	got := huggingFaceURL("onnx/model.onnx")
	if !strings.Contains(got, "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2") || !strings.Contains(got, "/onnx/model.onnx") {
		t.Fatalf("unexpected url %q", got)
	}
}

func TestWriteReaderAtomicallyReplacesCorruptDestination(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "runtime.bin")
	if err := os.WriteFile(dst, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeReaderAtomically(dst, strings.NewReader("runtime"), 0o755); err != nil {
		t.Fatalf("replace destination: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "runtime" {
		t.Fatalf("destination=%q want runtime", data)
	}
}

func TestCopyWithLimitRejectsOversizedPayload(t *testing.T) {
	var dst strings.Builder
	if _, err := copyWithLimit(&dst, strings.NewReader("12345"), 4); err == nil {
		t.Fatal("expected oversized payload error")
	}
}

func TestValidateJSONFileRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokenizer.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateJSONFile(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if err := os.WriteFile(path, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateJSONFile(path); err != nil {
		t.Fatalf("valid JSON rejected: %v", err)
	}
}

func TestStageAssetsRootMatchesBundleLayout(t *testing.T) {
	buildDir := filepath.Join("build", "bin")
	mac := stageAssetsRoot("darwin", buildDir, "ray-player1")
	wantMac := filepath.Join(buildDir, "ray-player1.app", "Contents", "Resources", "assets")
	if mac != wantMac {
		t.Fatalf("darwin root=%q want=%q", mac, wantMac)
	}
	windows := stageAssetsRoot("windows", buildDir, "ray-player1")
	wantWindows := filepath.Join(buildDir, "assets")
	if windows != wantWindows {
		t.Fatalf("windows root=%q want=%q", windows, wantWindows)
	}
}

func TestCopyTreeFiltersEssentiaSourceArtifacts(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"model.onnx": "onnx",
		"model.json": "{}",
		"model.pb":   "tensorflow",
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyTree(src, dst, func(path string) bool {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".onnx" || ext == ".json"
	}); err != nil {
		t.Fatal(err)
	}
	if !regularFile(filepath.Join(dst, "model.onnx")) || !regularFile(filepath.Join(dst, "model.json")) {
		t.Fatal("expected ONNX and JSON files to be staged")
	}
	if _, err := os.Stat(filepath.Join(dst, "model.pb")); !os.IsNotExist(err) {
		t.Fatalf("TensorFlow source artifact should not be staged, err=%v", err)
	}
}
