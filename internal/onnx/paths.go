package onnx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	runtimePathEnv                = "RAY_PLAYER_ONNXRUNTIME_PATH"
	requiredONNXRuntimeAPIVersion = 26
)

func ResolveRuntimeLibrary() (string, error) {
	if configured := os.Getenv(runtimePathEnv); configured != "" {
		path, err := filepath.Abs(configured)
		if err != nil {
			return "", err
		}
		if isRegularFile(path) {
			return path, nil
		}
		return "", fmt.Errorf("%s points to a missing file: %s", runtimePathEnv, path)
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	executableDir := filepath.Dir(executable)
	libraryName, err := runtimeLibraryName()
	if err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(executableDir, libraryName),
		filepath.Join(executableDir, "resources", libraryName),
		filepath.Join(executableDir, "runtime", libraryName),
		filepath.Join(executableDir, "runtime", runtime.GOOS+"-"+runtime.GOARCH, libraryName),
	}

	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		candidates = append(candidates,
			filepath.Join(cwd, libraryName),
			filepath.Join(cwd, "build", "runtime", runtime.GOOS+"-"+runtime.GOARCH, libraryName),
		)
	}

	for _, candidate := range candidates {
		if isRegularFile(candidate) {
			absolute, absErr := filepath.Abs(candidate)
			if absErr != nil {
				return "", absErr
			}
			return absolute, nil
		}
	}

	return "", fmt.Errorf("ONNX Runtime library %q was not found; set %s or place it next to the executable", libraryName, runtimePathEnv)
}

func runtimeLibraryName() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll", nil
	case "darwin":
		return "libonnxruntime.dylib", nil
	case "linux":
		return "libonnxruntime.so.1.26.0", nil
	default:
		return "", errors.New("unsupported ONNX Runtime platform: " + runtime.GOOS)
	}
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
