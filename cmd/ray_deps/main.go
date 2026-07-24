package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	rayonnx "ray-player1/internal/onnx"
)

const (
	onnxRuntimeVersion = "1.26.0"
	miniLMRepository   = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
	maxDownloadBytes   = int64(2 << 30)
)

type runtimeAsset struct {
	Archive string
	Member  string
	Library string
	Zip     bool
}

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
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		path, err := exec.LookPath(name)
		if err != nil {
			failures = append(failures, name+" not found in PATH")
			continue
		}
		fmt.Printf("ok %-12s %s\n", name, path)
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
	if ffmpegReady() {
		fmt.Println("ffmpeg and ffprobe are available")
		return nil
	}
	if !*install {
		return errors.New("ffmpeg/ffprobe not found; run `just deps-ffmpeg`")
	}
	if err := installFFmpeg(ctx); err != nil {
		return err
	}
	if !ffmpegReady() {
		return errors.New("ffmpeg install command completed, but ffmpeg/ffprobe are still not visible in PATH; restart the shell and run `just deps-check`")
	}
	fmt.Println("ffmpeg and ffprobe installed")
	return nil
}

func ffmpegReady() bool {
	_, ffmpegErr := exec.LookPath("ffmpeg")
	_, ffprobeErr := exec.LookPath("ffprobe")
	return ffmpegErr == nil && ffprobeErr == nil
}

func installFFmpeg(ctx context.Context) error {
	var command []string
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return errors.New("Homebrew not found; install Homebrew or ffmpeg manually")
		}
		command = []string{"brew", "install", "ffmpeg"}
	case "windows":
		if _, err := exec.LookPath("winget"); err != nil {
			return errors.New("winget not found; install ffmpeg manually and add it to PATH")
		}
		command = []string{"winget", "install", "--id", "Gyan.FFmpeg", "--exact", "--accept-package-agreements", "--accept-source-agreements"}
	case "linux":
		command = linuxInstallCommand()
		if len(command) == 0 {
			return errors.New("no supported package manager found; install ffmpeg and ffprobe manually")
		}
	default:
		return fmt.Errorf("automatic ffmpeg installation is unsupported on %s", runtime.GOOS)
	}
	fmt.Println("running:", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install ffmpeg: %w", err)
	}
	return nil
}

func linuxInstallCommand() []string {
	prefix := []string{}
	if os.Geteuid() != 0 {
		if _, err := exec.LookPath("sudo"); err != nil {
			return nil
		}
		prefix = []string{"sudo"}
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		command := "apt-get update && apt-get install -y ffmpeg"
		if len(prefix) > 0 {
			command = "sudo apt-get update && sudo apt-get install -y ffmpeg"
		}
		return []string{"sh", "-c", command}
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return append(prefix, "dnf", "install", "-y", "ffmpeg")
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return append(prefix, "pacman", "-S", "--needed", "--noconfirm", "ffmpeg")
	}
	return nil
}

func installMiniLM(ctx context.Context) error {
	dir := miniLMDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := []struct {
		Name string
		URL  string
	}{
		{Name: "model.onnx", URL: huggingFaceURL("onnx/model.onnx")},
		{Name: "tokenizer.json", URL: huggingFaceURL("tokenizer.json")},
	}
	for _, file := range files {
		dst := filepath.Join(dir, file.Name)
		if regularFile(dst) {
			if file.Name != "tokenizer.json" || validateJSONFile(dst) == nil {
				fmt.Println("exists:", dst)
				continue
			}
			fmt.Println("invalid cached file, downloading again:", dst)
			_ = os.Remove(dst)
		}
		if err := downloadFile(ctx, file.URL, dst); err != nil {
			return fmt.Errorf("download %s: %w", file.Name, err)
		}
	}
	if err := validateJSONFile(filepath.Join(dir, "tokenizer.json")); err != nil {
		return fmt.Errorf("validate tokenizer.json: %w", err)
	}
	if _, _, err := rayonnx.ResolveModelFiles(dir); err != nil {
		return fmt.Errorf("validate MiniLM bundle: %w", err)
	}
	fmt.Println("MiniLM ready:", dir)
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
	if _, _, err := rayonnx.ResolveModelFiles(miniLMDir()); err != nil {
		return fmt.Errorf("stage MiniLM: %w", err)
	}
	asset, ok := onnxRuntimeAsset(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return fmt.Errorf("portable runtime staging is unsupported for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if !regularFile(filepath.Join(onnxRuntimeDir(), asset.Library)) {
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
	if err := copyTree(filepath.Join("assets", "runtime"), runtimeDst, func(path string) bool {
		return !strings.HasSuffix(path, ".part") &&
			!strings.HasSuffix(path, ".download")
	}); err != nil {
		return fmt.Errorf("stage runtime assets: %w", err)
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

func huggingFaceURL(path string) string {
	return fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s?download=true", miniLMRepository, path)
}

func installONNXRuntime(ctx context.Context) error {
	asset, ok := onnxRuntimeAsset(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return fmt.Errorf("no managed ONNX Runtime %s archive for %s/%s; install ONNX Runtime %s manually and set RAY_PLAYER_ONNXRUNTIME_PATH", onnxRuntimeVersion, runtime.GOOS, runtime.GOARCH, onnxRuntimeVersion)
	}
	dir := onnxRuntimeDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dir, asset.Library)
	if regularFile(dst) {
		if err := rayonnx.TestRuntime(dst); err == nil {
			fmt.Println("ONNX Runtime exists and passed smoke test:", dst)
			return nil
		}
		fmt.Println("invalid cached ONNX Runtime, downloading again:", dst)
		_ = os.Remove(dst)
	}

	archivePath := filepath.Join(dir, asset.Archive+".download")
	url := fmt.Sprintf("https://github.com/microsoft/onnxruntime/releases/download/v%s/%s", onnxRuntimeVersion, asset.Archive)
	if err := downloadFile(ctx, url, archivePath); err != nil {
		return err
	}
	defer os.Remove(archivePath)

	if asset.Zip {
		if err := extractZipMember(archivePath, asset.Member, dst); err != nil {
			return err
		}
	} else if err := extractTarGzipMember(archivePath, asset.Member, dst); err != nil {
		return err
	}
	if err := os.Chmod(dst, 0o755); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if err := rayonnx.TestRuntime(dst); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("downloaded ONNX Runtime failed smoke test: %w", err)
	}
	fmt.Println("ONNX Runtime ready:", dst)
	return nil
}

func onnxRuntimeAsset(goos, goarch string) (runtimeAsset, bool) {
	prefix := "onnxruntime-"
	switch goos + "/" + goarch {
	case "darwin/arm64":
		archive := prefix + "osx-arm64-" + onnxRuntimeVersion + ".tgz"
		return runtimeAsset{Archive: archive, Member: "/lib/libonnxruntime." + onnxRuntimeVersion + ".dylib", Library: "libonnxruntime." + onnxRuntimeVersion + ".dylib"}, true
	case "linux/amd64":
		archive := prefix + "linux-x64-" + onnxRuntimeVersion + ".tgz"
		return runtimeAsset{Archive: archive, Member: "/lib/libonnxruntime.so." + onnxRuntimeVersion, Library: "libonnxruntime.so." + onnxRuntimeVersion}, true
	case "linux/arm64":
		archive := prefix + "linux-aarch64-" + onnxRuntimeVersion + ".tgz"
		return runtimeAsset{Archive: archive, Member: "/lib/libonnxruntime.so." + onnxRuntimeVersion, Library: "libonnxruntime.so." + onnxRuntimeVersion}, true
	case "windows/amd64":
		archive := prefix + "win-x64-" + onnxRuntimeVersion + ".zip"
		return runtimeAsset{Archive: archive, Member: "/lib/onnxruntime.dll", Library: "onnxruntime.dll", Zip: true}, true
	case "windows/arm64":
		archive := prefix + "win-arm64-" + onnxRuntimeVersion + ".zip"
		return runtimeAsset{Archive: archive, Member: "/lib/onnxruntime.dll", Library: "onnxruntime.dll", Zip: true}, true
	default:
		return runtimeAsset{}, false
	}
}

func miniLMDir() string {
	return filepath.Join("assets", "runtime", "models", "paraphrase-multilingual-MiniLM-L12-v2_onnx")
}

func onnxRuntimeDir() string {
	return filepath.Join("assets", "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH)
}

func downloadFile(ctx context.Context, url, dst string) error {
	part := dst + ".part"
	_ = os.Remove(part)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if resp.ContentLength > maxDownloadBytes {
		return fmt.Errorf("GET %s: payload too large: %d bytes", url, resp.ContentLength)
	}
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := copyWithLimit(f, resp.Body, maxDownloadBytes)
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

func extractTarGzipMember(archivePath, memberSuffix, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(filepath.ToSlash(hdr.Name), memberSuffix) {
			continue
		}
		return writeReaderAtomically(dst, tr, 0o755)
	}
	return fmt.Errorf("archive member %q not found in %s", memberSuffix, archivePath)
}

func extractZipMember(archivePath, memberSuffix, dst string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(filepath.ToSlash(file.Name), memberSuffix) {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return err
		}
		err = writeReaderAtomically(dst, r, 0o755)
		closeErr := r.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return fmt.Errorf("archive member %q not found in %s", memberSuffix, archivePath)
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

func validateJSONFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return nil
}
