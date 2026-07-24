package onnx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	runtimePathEnv                = "RAY_PLAYER_ONNXRUNTIME_PATH"
	standardRuntimePathEnv        = "ONNXRUNTIME_SHARED_LIBRARY_PATH"
	requiredONNXRuntimeVersion    = "1.26.0"
	requiredONNXRuntimeAPIVersion = 26
)

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

	for _, candidate := range runtimeLibraryCandidates(
		filepath.Dir(executable),
		cwd,
		os.Getenv("PATH"),
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

func runtimeLibraryCandidates(
	executableDir string,
	cwd string,
	pathEnv string,
	libraryNames []string,
) []string {
	dirs := []string{
		executableDir,
		filepath.Join(executableDir, "resources"),
		filepath.Join(executableDir, "runtime"),
		filepath.Join(executableDir, "runtime", runtime.GOOS+"-"+runtime.GOARCH),
		filepath.Join(executableDir, "assets", "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH),
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

	seen := map[string]bool{}
	out := make([]string, 0, len(dirs)*len(libraryNames))
	for _, dir := range dirs {
		for _, name := range libraryNames {
			candidate := filepath.Clean(filepath.Join(dir, name))
			if !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
		}
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
	return err == nil && info.Mode().IsRegular()
}
