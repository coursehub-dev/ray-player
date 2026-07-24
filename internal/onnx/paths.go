package onnx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ray-player1/internal/appdirs"
)

const (
	runtimePathEnv                = "RAY_PLAYER_ONNXRUNTIME_PATH"
	standardRuntimePathEnv        = "ONNXRUNTIME_SHARED_LIBRARY_PATH"
	miniLMDirEnv                  = "RAY_PLAYER_MINILM_DIR"
	essentiaDirEnv                = "RAY_PLAYER_ESSENTIA_DIR"
	requiredONNXRuntimeVersion    = "1.26.0"
	requiredONNXRuntimeAPIVersion = 26
)

const miniLMRelativeDir = "paraphrase-multilingual-MiniLM-L12-v2_onnx"

var essentiaRuntimeModelNames = []string{
	"discogs-effnet-bs64-1",
	"discogs-effnet-bsdynamic-1",
	"genre_discogs400-discogs-effnet-1",
	"deeptemp-k4-3",
	"danceability-discogs-effnet-1",
	"mood_happy-discogs-effnet-1",
	"mood_sad-discogs-effnet-1",
	"mood_relaxed-discogs-effnet-1",
	"mood_party-discogs-effnet-1",
	"mood_aggressive-discogs-effnet-1",
	"mood_acoustic-discogs-effnet-1",
	"mood_electronic-discogs-effnet-1",
	"voice_instrumental-discogs-effnet-1",
	"mtg_jamendo_moodtheme-discogs-effnet-1",
	"timbre-discogs-effnet-1",
	"tonal_atonal-discogs-effnet-1",
	"approachability_regression-discogs-effnet-1",
	"engagement_regression-discogs-effnet-1",
}

// RequiredEssentiaFiles returns the complete runtime bundle. TensorFlow .pb
// sources and unused experimental models are intentionally excluded.
func RequiredEssentiaFiles() []string {
	files := make([]string, 0, len(essentiaRuntimeModelNames)*2)
	for _, name := range essentiaRuntimeModelNames {
		files = append(files, name+".onnx", name+".json")
	}
	return files
}

func ResolveRuntimeLibraryPath(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return ResolveRuntimeLibrary()
	}
	path, err := filepath.Abs(configured)
	if err != nil {
		return "", err
	}
	if !isRegularFile(path) {
		return "", fmt.Errorf("runtime library missing: %s", path)
	}
	return path, nil
}

func ResolveRuntimeLibrary() (string, error) {
	for _, envName := range []string{runtimePathEnv, standardRuntimePathEnv} {
		configured := strings.TrimSpace(os.Getenv(envName))
		if configured == "" {
			continue
		}
		path, err := filepath.Abs(configured)
		if err != nil {
			return "", err
		}
		if isRegularFile(path) {
			return path, nil
		}
		return "", fmt.Errorf("%s points to a missing file: %s", envName, path)
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	cwd, _ := os.Getwd()
	libraryNames, err := runtimeLibraryNames()
	if err != nil {
		return "", err
	}

	managedRuntimeDir, _ := appdirs.ManagedONNXRuntimeDir()

	for _, candidate := range runtimeLibraryCandidates(
		filepath.Dir(executable),
		cwd,
		os.Getenv("PATH"),
		managedRuntimeDir,
		libraryNames,
	) {
		if isRegularFile(candidate) {
			absolute, absErr := filepath.Abs(candidate)
			if absErr != nil {
				return "", absErr
			}
			return absolute, nil
		}
	}

	return "", fmt.Errorf(
		"ONNX Runtime %s library was not found; run `just deps-onnxruntime`, set %s, or install the matching runtime system-wide",
		requiredONNXRuntimeVersion,
		runtimePathEnv,
	)
}

func ResolveMiniLMModelDir(configured string) (string, error) {
	return resolveAssetDir(
		configured,
		miniLMDirEnv,
		filepath.Join("assets", "runtime", "models", miniLMRelativeDir),
		func(dir string) error {
			_, _, err := ResolveModelFiles(dir)
			return err
		},
	)
}

func ResolveEssentiaModelDir(configured string) (string, error) {
	return resolveAssetDir(
		configured,
		essentiaDirEnv,
		filepath.Join("assets", "models", "essentia"),
		validateEssentiaModelDir,
	)
}

func resolveAssetDir(
	configured string,
	envName string,
	relative string,
	validate func(string) error,
) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return validateAssetDir(configured, validate)
	}
	if fromEnv := strings.TrimSpace(os.Getenv(envName)); fromEnv != "" {
		dir, err := validateAssetDir(fromEnv, validate)
		if err != nil {
			return "", fmt.Errorf("%s: %w", envName, err)
		}
		return dir, nil
	}

	executableDir := ""
	if executable, err := os.Executable(); err == nil {
		executableDir = filepath.Dir(executable)
	}
	cwd, _ := os.Getwd()
	managed := ""
	switch relative {
	case filepath.Join("assets", "runtime", "models", miniLMRelativeDir):
		managed, _ = appdirs.ManagedMiniLMDir()
	case filepath.Join("assets", "models", "essentia"):
		managed, _ = appdirs.ManagedEssentiaDir()
	}
	for _, candidate := range assetDirCandidates(executableDir, cwd, managed, relative) {
		if err := validate(candidate); err != nil {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		return absolute, nil
	}

	return "", fmt.Errorf(
		"model assets not found for %s; run `just deps` or configure %s",
		relative,
		envName,
	)
}

func validateAssetDir(path string, validate func(string) error) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	if err := validate(absolute); err != nil {
		return "", fmt.Errorf("invalid model directory %s: %w", absolute, err)
	}
	return absolute, nil
}

func assetDirCandidates(executableDir, cwd, managed, relative string) []string {
	dirs := make([]string, 0, 6)
	if strings.TrimSpace(managed) != "" {
		dirs = append(dirs, managed)
	}
	if strings.TrimSpace(executableDir) != "" {
		dirs = append(dirs,
			filepath.Join(executableDir, relative),
			filepath.Join(executableDir, "resources", relative),
			filepath.Join(executableDir, "..", "Resources", relative),
		)
	}
	if strings.TrimSpace(cwd) != "" {
		dirs = append(dirs, filepath.Join(cwd, relative))
	}
	return uniqueCleanPaths(dirs)
}

func validateEssentiaModelDir(dir string) error {
	for _, name := range RequiredEssentiaFiles() {
		path := filepath.Join(dir, name)
		if !isRegularFile(path) {
			return fmt.Errorf("required model file missing: %s", path)
		}
		if filepath.Ext(name) == ".json" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read model metadata %s: %w", path, err)
			}
			if !json.Valid(raw) {
				return fmt.Errorf("invalid model metadata JSON: %s", path)
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open model file %s: %w", path, err)
		}
		buf := make([]byte, 256)
		n, readErr := f.Read(buf)
		closeErr := f.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("read model file %s: %w", path, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close model file %s: %w", path, closeErr)
		}
		prefix := bytes.TrimSpace(buf[:n])
		if bytes.HasPrefix(prefix, []byte("version https://git-lfs.github.com/spec/v1")) {
			return fmt.Errorf("Git LFS pointer downloaded instead of ONNX model: %s", path)
		}
		lower := bytes.ToLower(prefix)
		if bytes.HasPrefix(lower, []byte("<!doctype html")) || bytes.HasPrefix(lower, []byte("<html")) {
			return fmt.Errorf("HTML downloaded instead of ONNX model: %s", path)
		}
	}
	return nil
}

func runtimeLibraryCandidates(
	executableDir string,
	cwd string,
	pathEnv string,
	managedDir string,
	libraryNames []string,
) []string {
	dirs := []string{
		executableDir,
		filepath.Join(executableDir, "resources"),
		filepath.Join(executableDir, "runtime"),
		filepath.Join(executableDir, "runtime", runtime.GOOS+"-"+runtime.GOARCH),
		filepath.Join(executableDir, "assets", "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH),
		filepath.Join(executableDir, "..", "Resources"),
		filepath.Join(executableDir, "..", "Resources", "runtime", runtime.GOOS+"-"+runtime.GOARCH),
		filepath.Join(executableDir, "..", "Resources", "assets", "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH),
	}
	if strings.TrimSpace(cwd) != "" {
		dirs = append(dirs,
			cwd,
			filepath.Join(cwd, "build", "runtime", runtime.GOOS+"-"+runtime.GOARCH),
			filepath.Join(cwd, "assets", "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH),
		)
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		if strings.TrimSpace(dir) != "" {
			dirs = append(dirs, dir)
		}
	}
	dirs = append(dirs, systemRuntimeDirs()...)
	if strings.TrimSpace(managedDir) != "" {
		dirs = append(dirs, managedDir)
	}

	dirs = uniqueCleanPaths(dirs)
	out := make([]string, 0, len(dirs)*len(libraryNames))
	for _, dir := range dirs {
		for _, name := range libraryNames {
			out = append(out, filepath.Clean(filepath.Join(dir, name)))
		}
	}
	return out
}

func uniqueCleanPaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		clean := filepath.Clean(path)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

func runtimeLibraryNames() ([]string, error) {
	switch runtime.GOOS {
	case "windows":
		return []string{"onnxruntime.dll"}, nil
	case "darwin":
		return []string{
			"libonnxruntime." + requiredONNXRuntimeVersion + ".dylib",
			"libonnxruntime.dylib",
		}, nil
	case "linux":
		return []string{
			"libonnxruntime.so." + requiredONNXRuntimeVersion,
			"libonnxruntime.so",
		}, nil
	default:
		return nil, errors.New("unsupported ONNX Runtime platform: " + runtime.GOOS)
	}
}

func runtimeLibraryName() (string, error) {
	names, err := runtimeLibraryNames()
	if err != nil {
		return "", err
	}
	return names[0], nil
}

func systemRuntimeDirs() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"/usr/local/lib",
			"/usr/lib",
			"/usr/lib64",
			"/usr/lib/x86_64-linux-gnu",
			"/usr/lib/aarch64-linux-gnu",
		}
	case "darwin":
		return []string{
			"/opt/homebrew/lib",
			"/usr/local/lib",
		}
	default:
		return nil
	}
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
