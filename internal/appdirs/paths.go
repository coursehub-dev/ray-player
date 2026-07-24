package appdirs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const AppName = "ray-player1"

func Root(appName string) (string, error) {
	if appName == "" {
		appName = AppName
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	root := filepath.Join(base, appName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create app data dir %q: %w", root, err)
	}
	return root, nil
}

func DefaultRoot() (string, error) { return Root(AppName) }

func AssetsRoot() (string, error) {
	root, err := DefaultRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "assets"), nil
}

func ManagedFFmpegDir() (string, error) {
	root, err := AssetsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "bin", runtime.GOOS+"-"+runtime.GOARCH), nil
}

func ManagedONNXRuntimeDir() (string, error) {
	root, err := AssetsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH), nil
}

func ManagedEssentiaDir() (string, error) {
	root, err := AssetsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "runtime", "models", "essentia"), nil
}

func ManagedMiniLMDir() (string, error) {
	root, err := AssetsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "runtime", "models", "paraphrase-multilingual-MiniLM-L12-v2_onnx"), nil
}
