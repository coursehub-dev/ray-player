package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
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
)

const (
	onnxRuntimeVersion = "1.26.0"
	miniLMRepository   = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
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
	fmt.Fprintln(os.Stderr, "usage: go run ./cmd/ray_deps <check|ffmpeg|minilm|onnxruntime>")
	fmt.Fprintln(os.Stderr, "       go run ./cmd/ray_deps ffmpeg --install")
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

	modelDir := miniLMDir()
	for _, name := range []string{"model.onnx", "tokenizer.json"} {
		path := filepath.Join(modelDir, name)
		if !regularFile(path) {
			failures = append(failures, "MiniLM missing: "+path)
			continue
		}
		fmt.Printf("ok %-12s %s\n", "minilm", path)
	}

	if path := findONNXRuntimeForCheck(); path != "" {
		fmt.Printf("ok %-12s %s\n", "onnxruntime", path)
	} else if _, ok := onnxRuntimeAsset(runtime.GOOS, runtime.GOARCH); ok {
		failures = append(failures, "ONNX Runtime not found; run `just deps-onnxruntime` or set RAY_PLAYER_ONNXRUNTIME_PATH")
	} else {
		failures = append(failures, fmt.Sprintf("ONNX Runtime not found for %s/%s; install %s manually and set RAY_PLAYER_ONNXRUNTIME_PATH", runtime.GOOS, runtime.GOARCH, onnxRuntimeVersion))
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func findONNXRuntimeForCheck() string {
	for _, envName := range []string{"RAY_PLAYER_ONNXRUNTIME_PATH", "ONNXRUNTIME_SHARED_LIBRARY_PATH"} {
		if value := strings.TrimSpace(os.Getenv(envName)); regularFile(value) {
			return value
		}
	}
	names := runtimeLibraryNamesForCheck()
	dirs := []string{onnxRuntimeDir()}
	if executable, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(executable))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs,
			cwd,
			filepath.Join(cwd, "assets", "runtime", "onnxruntime", runtime.GOOS+"-"+runtime.GOARCH),
		)
	}
	dirs = append(dirs, filepath.SplitList(os.Getenv("PATH"))...)
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs, "/opt/homebrew/lib", "/usr/local/lib")
	case "linux":
		dirs = append(dirs, "/usr/local/lib", "/usr/lib", "/usr/lib64", "/usr/lib/x86_64-linux-gnu", "/usr/lib/aarch64-linux-gnu")
	}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		for _, name := range names {
			path := filepath.Join(dir, name)
			if regularFile(path) {
				return path
			}
		}
	}
	return ""
}

func runtimeLibraryNamesForCheck() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"onnxruntime.dll"}
	case "darwin":
		return []string{"libonnxruntime." + onnxRuntimeVersion + ".dylib", "libonnxruntime.dylib"}
	case "linux":
		return []string{"libonnxruntime.so." + onnxRuntimeVersion, "libonnxruntime.so"}
	default:
		return nil
	}
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
			fmt.Println("exists:", dst)
			continue
		}
		if err := downloadFile(ctx, file.URL, dst); err != nil {
			return fmt.Errorf("download %s: %w", file.Name, err)
		}
	}
	fmt.Println("MiniLM ready:", dir)
	return nil
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
		fmt.Println("ONNX Runtime exists:", dst)
		return nil
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
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(part)
		return copyErr
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
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(part)
		return copyErr
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
