package onnx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveRuntimeLibraryHonorsExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.bin")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(runtimePathEnv, path)

	got, err := ResolveRuntimeLibrary()
	if err != nil {
		t.Fatalf("ResolveRuntimeLibrary: %v", err)
	}
	want, _ := filepath.Abs(path)
	if got != want {
		t.Fatalf("path=%q want=%q", got, want)
	}
}

func TestRuntimeCandidatesIncludeLocalRuntimeAssets(t *testing.T) {
	names, err := runtimeLibraryNames()
	if err != nil {
		t.Skipf("platform unsupported: %v", err)
	}
	root := filepath.Join("tmp", "ray-player")
	candidates := runtimeLibraryCandidates(
		filepath.Join(root, "bin"),
		root,
		"",
		names,
	)
	want := filepath.Clean(filepath.Join(
		root,
		"assets",
		"runtime",
		"onnxruntime",
		runtime.GOOS+"-"+runtime.GOARCH,
		names[0],
	))
	for _, candidate := range candidates {
		if candidate == want {
			return
		}
	}
	t.Fatalf("missing local runtime candidate %q in %#v", want, candidates)
}

func TestResolveRuntimeLibraryHonorsStandardWrapperEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.bin")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(runtimePathEnv, "")
	t.Setenv(standardRuntimePathEnv, path)

	got, err := ResolveRuntimeLibrary()
	if err != nil {
		t.Fatalf("ResolveRuntimeLibrary: %v", err)
	}
	want, _ := filepath.Abs(path)
	if got != want {
		t.Fatalf("path=%q want=%q", got, want)
	}
}
