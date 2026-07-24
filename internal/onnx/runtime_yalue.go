package onnx

import (
	"fmt"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	environmentMu          sync.Mutex
	environmentInitialized bool
	environmentReferences  int
	environmentLibraryPath string
)

func AcquireEnvironment() error {
	return AcquireEnvironmentWithPath("")
}

func AcquireEnvironmentWithPath(runtimePath string) error {
	libraryPath, err := resolveRuntimeLibraryWithOverride(runtimePath)
	if err != nil {
		return err
	}

	environmentMu.Lock()
	defer environmentMu.Unlock()

	if environmentInitialized {
		if !sameRuntimeLibrary(environmentLibraryPath, libraryPath) {
			return fmt.Errorf(
				"ONNX Runtime already initialized from %q; refusing incompatible runtime %q",
				environmentLibraryPath,
				libraryPath,
			)
		}
		environmentReferences++
		return nil
	}

	ort.SetSharedLibraryPath(libraryPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("initialize ONNX Runtime from %q: %w", libraryPath, err)
	}

	environmentInitialized = true
	environmentReferences = 1
	environmentLibraryPath = libraryPath
	return nil
}

func resolveRuntimeLibraryWithOverride(runtimePath string) (string, error) {
	if runtimePath != "" {
		path, err := filepath.Abs(runtimePath)
		if err != nil {
			return "", err
		}
		if !isRegularFile(path) {
			return "", fmt.Errorf("runtime library missing: %s", path)
		}
		return path, nil
	}
	return ResolveRuntimeLibrary()
}

func ReleaseEnvironment() error {
	environmentMu.Lock()
	defer environmentMu.Unlock()

	if !environmentInitialized {
		return nil
	}

	environmentReferences--
	if environmentReferences > 0 {
		return nil
	}

	err := ort.DestroyEnvironment()
	environmentInitialized = false
	environmentReferences = 0
	environmentLibraryPath = ""
	if err != nil {
		return fmt.Errorf("destroy ONNX Runtime environment: %w", err)
	}
	return nil
}

func sameRuntimeLibrary(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
