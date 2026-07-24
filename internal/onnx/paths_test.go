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
		filepath.Join(root, "bin"), root, "", "",
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

func TestRuntimeCandidatesPreferSystemSearchBeforeManagedAssets(t *testing.T) {
	names, err := runtimeLibraryNames()
	if err != nil {
		t.Skipf("platform unsupported: %v", err)
	}
	managed := filepath.Join("tmp", "managed-runtime")
	pathDir := filepath.Join("tmp", "system-path")
	candidates := runtimeLibraryCandidates("", "", pathDir, managed, names)
	pathCandidate := filepath.Clean(filepath.Join(pathDir, names[0]))
	managedCandidate := filepath.Clean(filepath.Join(managed, names[0]))
	indexOf := func(want string) int {
		for i, candidate := range candidates {
			if candidate == want {
				return i
			}
		}
		return -1
	}
	pathIndex, managedIndex := indexOf(pathCandidate), indexOf(managedCandidate)
	if pathIndex < 0 || managedIndex < 0 {
		t.Fatalf("missing candidates path=%d managed=%d in %#v", pathIndex, managedIndex, candidates)
	}
	if pathIndex >= managedIndex {
		t.Fatalf("managed runtime must be fallback after PATH/system candidates: path=%d managed=%d", pathIndex, managedIndex)
	}
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

func TestResolveRuntimeLibraryFallsBackToPATH(t *testing.T) {
	names, err := runtimeLibraryNames()
	if err != nil {
		t.Skipf("platform unsupported: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, names[0])
	if err := os.WriteFile(path, []byte("runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(runtimePathEnv, "")
	t.Setenv(standardRuntimePathEnv, "")
	t.Setenv("PATH", dir)
	previousCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cleanCWD := t.TempDir()
	if err := os.Chdir(cleanCWD); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousCWD) })

	got, err := ResolveRuntimeLibrary()
	if err != nil {
		t.Fatalf("ResolveRuntimeLibrary: %v", err)
	}
	want, _ := filepath.Abs(path)
	if got != want {
		t.Fatalf("path=%q want=%q", got, want)
	}
}

func TestRuntimeCandidatesIncludeMacAppResourcesLayout(t *testing.T) {
	names, err := runtimeLibraryNames()
	if err != nil {
		t.Skipf("platform unsupported: %v", err)
	}
	root := filepath.Join("tmp", "Ray Player.app", "Contents")
	executableDir := filepath.Join(root, "MacOS")
	candidates := runtimeLibraryCandidates(executableDir, "", "", "", names)
	want := filepath.Clean(filepath.Join(
		root,
		"Resources",
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
	t.Fatalf("missing app resource candidate %q in %#v", want, candidates)
}

func TestResolveMiniLMModelDirHonorsExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.onnx"), []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tokenizer.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveMiniLMModelDir(dir)
	if err != nil {
		t.Fatalf("ResolveMiniLMModelDir: %v", err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("dir=%q want=%q", got, want)
	}
}

func TestResolveEssentiaModelDirRejectsPartialBundle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "discogs-effnet-bs64-1.onnx"), []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveEssentiaModelDir(dir); err == nil {
		t.Fatal("expected partial Essentia bundle to be rejected")
	}
}

func TestRequiredEssentiaFilesIncludesEveryLoadedHead(t *testing.T) {
	files := RequiredEssentiaFiles()
	seen := make(map[string]bool, len(files))
	for _, name := range files {
		seen[name] = true
	}
	for _, name := range append([]string{
		"discogs-effnet-bs64-1",
		"discogs-effnet-bsdynamic-1",
		"genre_discogs400-discogs-effnet-1",
		"deeptemp-k4-3",
	}, essentiaHeadNames...) {
		for _, ext := range []string{".onnx", ".json"} {
			if !seen[name+ext] {
				t.Fatalf("runtime bundle is missing %s", name+ext)
			}
		}
	}
}

func TestResolveEssentiaModelDirAcceptsCompleteRuntimeBundle(t *testing.T) {
	dir := t.TempDir()
	for _, name := range RequiredEssentiaFiles() {
		content := []byte("onnx-fixture")
		if filepath.Ext(name) == ".json" {
			content = []byte(`{"classes":[]}`)
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ResolveEssentiaModelDir(dir)
	if err != nil {
		t.Fatalf("ResolveEssentiaModelDir: %v", err)
	}
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("dir=%q want=%q", got, want)
	}
}

func TestResolveEssentiaModelDirRejectsGitLFSPointer(t *testing.T) {
	dir := t.TempDir()
	for _, name := range RequiredEssentiaFiles() {
		content := []byte("onnx-fixture")
		if filepath.Ext(name) == ".json" {
			content = []byte(`{"classes":[]}`)
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:fixture\nsize 123\n"
	if err := os.WriteFile(filepath.Join(dir, "discogs-effnet-bsdynamic-1.onnx"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveEssentiaModelDir(dir); err == nil {
		t.Fatal("expected Git LFS pointer to be rejected")
	}
}
