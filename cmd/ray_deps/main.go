package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ray-player1/internal/appdirs"
	runtimeassets "ray-player1/internal/deps"
	rayonnx "ray-player1/internal/onnx"
)

const maxDownloadBytes = int64(2 << 30)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	var err error
	switch os.Args[1] {
	case "check":
		err = checkDependencies()
	case "ffmpeg":
		err = ffmpegCommand(ctx, os.Args[2:])
	case "minilm":
		err = installMiniLM(ctx)
	case "onnxruntime":
		err = installONNXRuntime(ctx)
	case "stage":
		err = stageCommand(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ray-deps:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./cmd/ray_deps <check|ffmpeg|minilm|onnxruntime|stage>")
	fmt.Fprintln(os.Stderr, "       go run ./cmd/ray_deps ffmpeg --install")
	fmt.Fprintln(os.Stderr, "       go run ./cmd/ray_deps stage --build-dir build/bin")
}

func checkDependencies() error {
	var failures []string
	ffmpegPath, ffprobePath, ffmpegErr := runtimeassets.ResolveFFmpegTools("", "")
	if ffmpegErr != nil {
		failures = append(failures, ffmpegErr.Error())
	} else {
		fmt.Printf("ok %-12s %s\n", "ffmpeg", ffmpegPath)
		fmt.Printf("ok %-12s %s\n", "ffprobe", ffprobePath)
	}

	runtimePath, runtimeErr := rayonnx.ResolveRuntimeLibrary()
	if runtimeErr != nil {
		failures = append(failures, runtimeErr.Error())
	} else if err := rayonnx.TestRuntime(runtimePath); err != nil {
		failures = append(failures, "ONNX Runtime failed smoke test: "+err.Error())
	} else {
		fmt.Printf("ok %-12s %s\n", "onnxruntime", runtimePath)
	}

	modelDir, modelErr := rayonnx.ResolveMiniLMModelDir("")
	if modelErr != nil {
		failures = append(failures, modelErr.Error())
	} else if runtimeErr == nil {
		engine, err := rayonnx.New(runtimePath, modelDir)
		if err != nil {
			failures = append(failures, "MiniLM failed to load: "+err.Error())
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			vec, encodeErr := engine.Encode(ctx, "ray player dependency smoke test")
			cancel()
			closeErr := engine.Close()
			switch {
			case encodeErr != nil:
				failures = append(failures, "MiniLM inference failed: "+encodeErr.Error())
			case len(vec) == 0:
				failures = append(failures, "MiniLM returned an empty embedding")
			case closeErr != nil:
				failures = append(failures, "MiniLM close failed: "+closeErr.Error())
			default:
				fmt.Printf("ok %-12s %s embedding=%d\n", "minilm", modelDir, len(vec))
			}
		}
	}

	essentiaDir, essentiaErr := rayonnx.ResolveEssentiaModelDir("")
	if essentiaErr != nil {
		failures = append(failures, essentiaErr.Error())
	} else if runtimeErr == nil {
		probe, err := rayonnx.ProbeEssentia(runtimePath, essentiaDir)
		if err != nil || !probe.Ready || !probe.Base.Loaded || !probe.Genre.Loaded {
			if err == nil {
				err = errors.New(probe.Message)
			}
			failures = append(failures, "Essentia failed smoke test: "+err.Error())
		} else {
			fmt.Printf("ok %-12s %s\n", "essentia", essentiaDir)
		}
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func ffmpegCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ffmpeg", flag.ContinueOnError)
	install := fs.Bool("install", false, "install ffmpeg when it is missing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if ffmpeg, ffprobe, err := runtimeassets.ResolveFFmpegTools("", ""); err == nil {
		fmt.Println("ffmpeg and ffprobe are available:", ffmpeg, ffprobe)
		return nil
	}
	if !*install {
		return errors.New("ffmpeg/ffprobe not found; run `just deps-ffmpeg`")
	}
	ffmpeg, ffprobe, err := runtimeassets.EnsureFFmpeg(ctx)
	if err != nil {
		return err
	}
	fmt.Println("managed ffmpeg ready:", ffmpeg)
	fmt.Println("managed ffprobe ready:", ffprobe)
	return nil
}

func installMiniLM(ctx context.Context) error {
	dir, err := runtimeassets.EnsureMiniLM(ctx, "")
	if err != nil {
		return err
	}
	fmt.Println("MiniLM ready:", dir)
	return nil
}

func installONNXRuntime(ctx context.Context) error {
	path, err := runtimeassets.EnsureManagedONNXRuntime(ctx)
	if err != nil {
		return err
	}
	fmt.Println("managed ONNX Runtime ready:", path)
	return nil
}

func stageCommand(args []string) error {
	fs := flag.NewFlagSet("stage", flag.ContinueOnError)
	buildDir := fs.String("build-dir", filepath.Join("build", "bin"), "Wails build output directory")
	appName := fs.String("app-name", "ray-player1", "Wails output application name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := rayonnx.ResolveEssentiaModelDir(filepath.Join("assets", "models", "essentia")); err != nil {
		return fmt.Errorf("stage Essentia models: %w", err)
	}
	managedMiniLMDir, err := appdirs.ManagedMiniLMDir()
	if err != nil {
		return err
	}
	if _, _, err := rayonnx.ResolveModelFiles(managedMiniLMDir); err != nil {
		return fmt.Errorf("stage MiniLM: %w", err)
	}
	managedRuntimePath, err := runtimeassets.ManagedONNXRuntimePath()
	if err != nil {
		return err
	}
	if !regularFile(managedRuntimePath) {
		return errors.New("managed ONNX Runtime is missing; run `just deps-onnxruntime`")
	}

	artifactRoot := *buildDir
	if runtime.GOOS == "darwin" {
		artifactRoot = filepath.Join(*buildDir, *appName+".app")
	}
	info, err := os.Stat(artifactRoot)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("build artifact directory is missing: %s", artifactRoot)
	}

	assetsRoot := stageAssetsRoot(runtime.GOOS, *buildDir, *appName)
	essentiaDst := filepath.Join(assetsRoot, "models", "essentia")
	runtimeDst := filepath.Join(assetsRoot, "runtime")
	if err := os.RemoveAll(essentiaDst); err != nil {
		return err
	}
	if err := os.RemoveAll(runtimeDst); err != nil {
		return err
	}
	if err := copyTree(
		filepath.Join("assets", "models", "essentia"),
		essentiaDst,
		func(path string) bool {
			ext := strings.ToLower(filepath.Ext(path))
			return ext == ".onnx" || ext == ".json"
		},
	); err != nil {
		return fmt.Errorf("stage Essentia models: %w", err)
	}
	managedRoot, err := appdirs.AssetsRoot()
	if err != nil {
		return err
	}
	managedRuntime := filepath.Join(managedRoot, "runtime")
	if err := copyTree(managedRuntime, runtimeDst, func(path string) bool {
		return !strings.HasSuffix(path, ".part") &&
			!strings.HasSuffix(path, ".download")
	}); err != nil {
		return fmt.Errorf("stage runtime assets: %w", err)
	}
	managedBin := filepath.Join(managedRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH)
	if info, statErr := os.Stat(managedBin); statErr == nil && info.IsDir() {
		if err := copyTree(managedBin, filepath.Join(assetsRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH), nil); err != nil {
			return fmt.Errorf("stage ffmpeg assets: %w", err)
		}
	}
	fmt.Println("portable assets staged:", assetsRoot)
	return nil
}

func stageAssetsRoot(goos, buildDir, appName string) string {
	if goos == "darwin" {
		return filepath.Join(buildDir, appName+".app", "Contents", "Resources", "assets")
	}
	return filepath.Join(buildDir, "assets")
}

func copyTree(src, dst string, include func(string) bool) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() || (include != nil && !include(path)) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		err = writeReaderAtomically(target, input, info.Mode().Perm())
		closeErr := input.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}

func writeReaderAtomically(dst string, r io.Reader, mode os.FileMode) error {
	part := dst + ".part"
	_ = os.Remove(part)
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := copyWithLimit(f, r, maxDownloadBytes)
	syncErr := f.Sync()
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(part)
		return copyErr
	}
	if syncErr != nil {
		_ = os.Remove(part)
		return syncErr
	}
	if closeErr != nil {
		_ = os.Remove(part)
		return closeErr
	}
	if err := replaceFile(part, dst); err != nil {
		_ = os.Remove(part)
		return err
	}
	return nil
}

func replaceFile(part, dst string) error {
	if runtime.GOOS == "windows" {
		if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return os.Rename(part, dst)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func copyWithLimit(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	reader := io.LimitReader(src, limit+1)
	written, err := io.Copy(dst, reader)
	if err != nil {
		return written, err
	}
	if written > limit {
		return written, fmt.Errorf("payload exceeds %d bytes", limit)
	}
	return written, nil
}
