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
)

func AcquireEnvironment() error {
	return AcquireEnvironmentWithPath("")
}

func AcquireEnvironmentWithPath(runtimePath string) error {
	environmentMu.Lock()
	defer environmentMu.Unlock()

	if environmentInitialized {
		environmentReferences++
		return nil
	}

	libraryPath, err := resolveRuntimeLibraryWithOverride(runtimePath)
	if err != nil {
		return err
	}

	ort.SetSharedLibraryPath(libraryPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("initialize ONNX Runtime from %q: %w", libraryPath, err)
	}

	environmentInitialized = true
	environmentReferences = 1
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
	if err != nil {
		return fmt.Errorf("destroy ONNX Runtime environment: %w", err)
	}
	return nil
}
