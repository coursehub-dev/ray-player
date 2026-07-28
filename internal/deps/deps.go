package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/appdirs"
	rayonnx "ray-player1/internal/onnx"
)

const (
	ONNXRuntimeVersion      = "1.26.0"
	MiniLMRepository        = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
	FFmpegStaticTag         = "b6.1.1"
	EssentiaModelBaseURL    = "https://raw.githubusercontent.com/coursehub-dev/ray-player/main/assets/models/essentia"
	essentiaModelBaseURLEnv = "RAY_PLAYER_ESSENTIA_BASE_URL"
	maxDownloadBytes        = int64(2 << 30)
)

type Status string

const (
	StatusReady      Status = "ready"
	StatusRepairable Status = "repairable"
	StatusBlocked    Status = "blocked"
)

type Settings struct {
	ONNXRuntimePath  string `json:"onnxRuntimePath"`
	MiniLMModelDir   string `json:"miniLMModelDir"`
	EssentiaModelDir string `json:"essentiaModelDir"`
	FFmpegPath       string `json:"ffmpegPath"`
	FFprobePath      string `json:"ffprobePath"`
	YtDlpPath        string `json:"ytDlpPath"`
}

type Check struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     Status `json:"status"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`
	Repairable bool   `json:"repairable"`
}

type SettingsPatch struct {
	ONNXRuntimePath  string `json:"onnxRuntimePath,omitempty"`
	MiniLMModelDir   string `json:"miniLMModelDir,omitempty"`
	EssentiaModelDir string `json:"essentiaModelDir,omitempty"`
	FFmpegPath       string `json:"ffmpegPath,omitempty"`
	FFprobePath      string `json:"ffprobePath,omitempty"`
	YtDlpPath        string `json:"ytDlpPath,omitempty"`
}

type RepairResult struct {
	Check Check         `json:"check"`
	Patch SettingsPatch `json:"patch"`
}

type runtimeAsset struct {
	Archive string
	Member  string
	Library string
	Zip     bool
}

type ffmpegAsset struct {
	FFmpegURL  string
	FFprobeURL string
	ZipURL     string
}

var repairMu sync.Mutex

func CheckComponent(ctx context.Context, id string, cfg Settings) Check {
	switch strings.TrimSpace(strings.ToLower(id)) {
	case "storage":
		return checkStorage()
	case "ffmpeg":
		return checkFFmpeg(ctx, cfg)
	case "ytdlp":
		return checkYtDlp(ctx, cfg)
	case "onnxruntime":
		return checkONNXRuntime(cfg)
	case "minilm":
		return checkMiniLM(ctx, cfg)
	case "essentia":
		return checkEssentia(cfg)
	default:
		return Check{ID: id, Title: id, Status: StatusBlocked, Message: "Неизвестная проверка"}
	}
}

func RepairComponent(ctx context.Context, id string, cfg Settings) RepairResult {
	repairMu.Lock()
	defer repairMu.Unlock()

	var patch SettingsPatch
	var err error
	switch strings.TrimSpace(strings.ToLower(id)) {
	case "ffmpeg":
		patch.FFmpegPath, patch.FFprobePath, err = EnsureFFmpeg(ctx)
	case "ytdlp":
		patch.YtDlpPath, err = EnsureYtDlp(ctx)
	case "onnxruntime":
		patch.ONNXRuntimePath, err = EnsureONNXRuntime(ctx)
	case "minilm":
		patch.MiniLMModelDir, err = EnsureMiniLM(ctx, cfg.MiniLMModelDir)
	case "essentia":
		patch.EssentiaModelDir, err = ensureEssentia(ctx, cfg.EssentiaModelDir, true)
	default:
		return RepairResult{Check: Check{ID: id, Title: id, Status: StatusBlocked, Message: "Автоисправление для этого пункта не предусмотрено"}}
	}
	if err != nil {
		return RepairResult{Check: Check{ID: id, Title: componentTitle(id), Status: StatusRepairable, Repairable: true, Message: err.Error()}}
	}
	cfg = applyPatch(cfg, patch)
	check := CheckComponent(ctx, id, cfg)
	return RepairResult{Check: check, Patch: patch}
}

func componentTitle(id string) string {
	switch id {
	case "storage":
		return "Папка данных и ассетов"
	case "ffmpeg":
		return "FFmpeg / ffprobe"
	case "ytdlp":
		return "yt-dlp"
	case "onnxruntime":
		return "ONNX Runtime"
	case "minilm":
		return "MiniLM"
	case "essentia":
		return "Essentia models"
	default:
		return id
	}
}

func applyPatch(cfg Settings, patch SettingsPatch) Settings {
	if patch.ONNXRuntimePath != "" {
		cfg.ONNXRuntimePath = patch.ONNXRuntimePath
	}
	if patch.MiniLMModelDir != "" {
		cfg.MiniLMModelDir = patch.MiniLMModelDir
	}
	if patch.EssentiaModelDir != "" {
		cfg.EssentiaModelDir = patch.EssentiaModelDir
	}
	if patch.FFmpegPath != "" {
		cfg.FFmpegPath = patch.FFmpegPath
	}
	if patch.FFprobePath != "" {
		cfg.FFprobePath = patch.FFprobePath
	}
	if patch.YtDlpPath != "" {
		cfg.YtDlpPath = patch.YtDlpPath
	}
	return cfg
}

func checkYtDlp(ctx context.Context, cfg Settings) Check {
	path, err := ResolveYtDlp(cfg.YtDlpPath)
	if err != nil {
		return Check{ID: "ytdlp", Title: componentTitle("ytdlp"), Status: StatusRepairable, Repairable: true, Message: err.Error()}
	}
	if err := runVersion(ctx, path, "--version"); err != nil {
		return Check{ID: "ytdlp", Title: componentTitle("ytdlp"), Status: StatusRepairable, Repairable: true, Message: "yt-dlp не запускается: " + err.Error(), Path: path}
	}
	return Check{ID: "ytdlp", Title: componentTitle("ytdlp"), Status: StatusReady, Message: "yt-dlp доступен", Path: path}
}

func ResolveYtDlp(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" && !isCommandDefault(configured, "yt-dlp") {
		return resolveExecutable(configured)
	}
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		return path, nil
	}
	dir, err := appdirs.ManagedYtDlpDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, executableName("yt-dlp"))
	if regularFile(path) {
		return path, nil
	}
	return "", errors.New("yt-dlp не найден")
}

func EnsureYtDlp(ctx context.Context) (string, error) {
	if path, err := ResolveYtDlp(""); err == nil && runVersion(ctx, path, "--version") == nil {
		return path, nil
	}
	dir, err := appdirs.ManagedYtDlpDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := executableName("yt-dlp")
	url := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
	if runtime.GOOS == "windows" {
		url += ".exe"
	} else if runtime.GOOS == "darwin" {
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"
	}
	dst := filepath.Join(dir, name)
	if err := downloadFile(ctx, url, dst); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dst, 0o755); err != nil {
			return "", err
		}
	}
	if err := runVersion(ctx, dst, "--version"); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("downloaded yt-dlp failed smoke-test: %w", err)
	}
	return dst, nil
}

func checkStorage() Check {
	root, err := appdirs.AssetsRoot()
	if err != nil {
		return Check{ID: "storage", Title: componentTitle("storage"), Status: StatusBlocked, Message: err.Error()}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Check{ID: "storage", Title: componentTitle("storage"), Status: StatusBlocked, Message: err.Error(), Path: root}
	}
	probe, err := os.CreateTemp(root, ".doctor-write-*")
	if err != nil {
		return Check{ID: "storage", Title: componentTitle("storage"), Status: StatusBlocked, Message: "Папка ассетов недоступна для записи: " + err.Error(), Path: root}
	}
	name := probe.Name()
	closeErr := probe.Close()
	_ = os.Remove(name)
	if closeErr != nil {
		return Check{ID: "storage", Title: componentTitle("storage"), Status: StatusBlocked, Message: closeErr.Error(), Path: root}
	}
	return Check{ID: "storage", Title: componentTitle("storage"), Status: StatusReady, Message: "Папка ассетов доступна для записи", Path: root}
}

func checkFFmpeg(ctx context.Context, cfg Settings) Check {
	ffmpeg, ffprobe, err := ResolveFFmpegTools(cfg.FFmpegPath, cfg.FFprobePath)
	if err != nil {
		if _, supported := ffmpegAssetFor(runtime.GOOS, runtime.GOARCH); supported {
			return Check{ID: "ffmpeg", Title: componentTitle("ffmpeg"), Status: StatusRepairable, Repairable: true, Message: err.Error()}
		}
		return Check{ID: "ffmpeg", Title: componentTitle("ffmpeg"), Status: StatusBlocked, Message: err.Error()}
	}
	if err := runVersion(ctx, ffmpeg, "-version"); err != nil {
		return Check{ID: "ffmpeg", Title: componentTitle("ffmpeg"), Status: StatusRepairable, Repairable: true, Message: "ffmpeg не запускается: " + err.Error(), Path: ffmpeg}
	}
	if err := runVersion(ctx, ffprobe, "-version"); err != nil {
		return Check{ID: "ffmpeg", Title: componentTitle("ffmpeg"), Status: StatusRepairable, Repairable: true, Message: "ffprobe не запускается: " + err.Error(), Path: ffmpeg}
	}
	return Check{ID: "ffmpeg", Title: componentTitle("ffmpeg"), Status: StatusReady, Message: "ffmpeg и ffprobe доступны", Path: ffmpeg}
}

func checkONNXRuntime(cfg Settings) Check {
	path, err := rayonnx.ResolveRuntimeLibraryPath(strings.TrimSpace(cfg.ONNXRuntimePath))
	if err != nil {
		if _, supported := OnnxRuntimeAsset(runtime.GOOS, runtime.GOARCH); supported {
			return Check{ID: "onnxruntime", Title: componentTitle("onnxruntime"), Status: StatusRepairable, Repairable: true, Message: err.Error()}
		}
		return Check{ID: "onnxruntime", Title: componentTitle("onnxruntime"), Status: StatusBlocked, Message: err.Error()}
	}
	if err := rayonnx.TestRuntime(path); err != nil {
		return Check{ID: "onnxruntime", Title: componentTitle("onnxruntime"), Status: StatusRepairable, Repairable: true, Message: "ONNX Runtime не прошёл smoke-test: " + err.Error(), Path: path}
	}
	return Check{ID: "onnxruntime", Title: componentTitle("onnxruntime"), Status: StatusReady, Message: "ONNX Runtime загружается и отвечает", Path: path}
}

func checkMiniLM(ctx context.Context, cfg Settings) Check {
	dir, err := rayonnx.ResolveMiniLMModelDir(strings.TrimSpace(cfg.MiniLMModelDir))
	if err != nil {
		return Check{ID: "minilm", Title: componentTitle("minilm"), Status: StatusRepairable, Repairable: true, Message: err.Error()}
	}
	runtimePath, runtimeErr := rayonnx.ResolveRuntimeLibraryPath(strings.TrimSpace(cfg.ONNXRuntimePath))
	if runtimeErr != nil {
		return Check{ID: "minilm", Title: componentTitle("minilm"), Status: StatusBlocked, Repairable: false, Message: "Сначала исправьте ONNX Runtime: " + runtimeErr.Error(), Path: dir}
	}
	engine, err := rayonnx.New(runtimePath, dir)
	if err != nil {
		return Check{ID: "minilm", Title: componentTitle("minilm"), Status: StatusRepairable, Repairable: true, Message: "MiniLM не загружается: " + err.Error(), Path: dir}
	}
	defer engine.Close()
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	vec, err := engine.Encode(probeCtx, "ray player doctor validation")
	if err != nil || len(vec) == 0 {
		if err == nil {
			err = errors.New("empty embedding")
		}
		return Check{ID: "minilm", Title: componentTitle("minilm"), Status: StatusRepairable, Repairable: true, Message: "MiniLM inference failed: " + err.Error(), Path: dir}
	}
	return Check{ID: "minilm", Title: componentTitle("minilm"), Status: StatusReady, Message: fmt.Sprintf("MiniLM готов, embedding=%d", len(vec)), Path: dir}
}

func checkEssentia(cfg Settings) Check {
	dir, err := rayonnx.ResolveEssentiaModelDir(strings.TrimSpace(cfg.EssentiaModelDir))
	if err != nil {
		return Check{ID: "essentia", Title: componentTitle("essentia"), Status: StatusRepairable, Repairable: true, Message: err.Error()}
	}
	runtimePath, err := rayonnx.ResolveRuntimeLibraryPath(strings.TrimSpace(cfg.ONNXRuntimePath))
	if err != nil {
		return Check{ID: "essentia", Title: componentTitle("essentia"), Status: StatusBlocked, Repairable: false, Message: "Сначала исправьте ONNX Runtime: " + err.Error(), Path: dir}
	}
	probe, probeErr := rayonnx.ProbeEssentia(runtimePath, dir)
	if probeErr != nil || !probe.Ready || !probe.Base.Loaded || !probe.Genre.Loaded {
		msg := probe.Message
		if probeErr != nil {
			msg = probeErr.Error()
		}
		if msg == "" {
			msg = "Essentia models не прошли smoke-test"
		}
		return Check{ID: "essentia", Title: componentTitle("essentia"), Status: StatusRepairable, Repairable: true, Message: msg, Path: dir}
	}
	return Check{ID: "essentia", Title: componentTitle("essentia"), Status: StatusReady, Message: probe.Message, Path: dir}
}

func ResolveFFmpegTools(configuredFFmpeg, configuredFFprobe string) (string, string, error) {
	ffmpegCfg := strings.TrimSpace(configuredFFmpeg)
	ffprobeCfg := strings.TrimSpace(configuredFFprobe)

	if ffmpegCfg != "" && !isCommandDefault(ffmpegCfg, "ffmpeg") {
		ffmpeg, err := resolveExecutable(ffmpegCfg)
		if err != nil {
			return "", "", fmt.Errorf("configured ffmpeg: %w", err)
		}
		ffprobe := ffprobeCfg
		if ffprobe == "" || isCommandDefault(ffprobe, "ffprobe") {
			ffprobe = siblingTool(ffmpeg, "ffprobe")
		}
		ffprobe, err = resolveExecutable(ffprobe)
		if err != nil {
			return "", "", fmt.Errorf("configured ffprobe: %w", err)
		}
		return ffmpeg, ffprobe, nil
	}

	if ffmpeg, err := exec.LookPath("ffmpeg"); err == nil {
		ffprobe := ffprobeCfg
		if ffprobe == "" || isCommandDefault(ffprobe, "ffprobe") {
			if found, probeErr := exec.LookPath("ffprobe"); probeErr == nil {
				return ffmpeg, found, nil
			}
			ffprobe = siblingTool(ffmpeg, "ffprobe")
		}
		if resolved, probeErr := resolveExecutable(ffprobe); probeErr == nil {
			return ffmpeg, resolved, nil
		}
	}

	for _, dir := range ffmpegCandidateDirs() {
		ffmpeg := filepath.Join(dir, executableName("ffmpeg"))
		ffprobe := filepath.Join(dir, executableName("ffprobe"))
		if regularFile(ffmpeg) && regularFile(ffprobe) {
			return ffmpeg, ffprobe, nil
		}
	}
	return "", "", errors.New("ffmpeg/ffprobe не найдены ни в системе, ни в управляемых ассетах")
}

func ffmpegCandidateDirs() []string {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	var dirs []string
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		dirs = append(dirs,
			filepath.Join(exeDir, "assets", "bin", platform),
			filepath.Join(exeDir, "..", "Resources", "assets", "bin", platform),
		)
	}
	if managed, err := appdirs.ManagedFFmpegDir(); err == nil {
		dirs = append(dirs, managed)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		clean := filepath.Clean(dir)
		if clean == "." || seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

func EnsureFFmpeg(ctx context.Context) (string, string, error) {
	if ffmpeg, ffprobe, err := ResolveFFmpegTools("", ""); err == nil {
		if runVersion(ctx, ffmpeg, "-version") == nil && runVersion(ctx, ffprobe, "-version") == nil {
			return ffmpeg, ffprobe, nil
		}
	}
	asset, ok := ffmpegAssetFor(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return "", "", fmt.Errorf("автоматическая загрузка ffmpeg не поддерживается на %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	dir, err := appdirs.ManagedFFmpegDir()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	ffmpeg := filepath.Join(dir, executableName("ffmpeg"))
	ffprobe := filepath.Join(dir, executableName("ffprobe"))
	if asset.ZipURL != "" {
		archive := filepath.Join(dir, "ffmpeg.download.zip")
		if err := downloadFile(ctx, asset.ZipURL, archive); err != nil {
			return "", "", err
		}
		defer os.Remove(archive)
		if err := extractZipMember(archive, "/bin/ffmpeg.exe", ffmpeg); err != nil {
			return "", "", err
		}
		if err := extractZipMember(archive, "/bin/ffprobe.exe", ffprobe); err != nil {
			return "", "", err
		}
	} else {
		if err := downloadGzipBinary(ctx, asset.FFmpegURL, ffmpeg); err != nil {
			return "", "", err
		}
		if err := downloadGzipBinary(ctx, asset.FFprobeURL, ffprobe); err != nil {
			return "", "", err
		}
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(ffmpeg, 0o755)
		_ = os.Chmod(ffprobe, 0o755)
	}
	if err := runVersion(ctx, ffmpeg, "-version"); err != nil {
		return "", "", fmt.Errorf("downloaded ffmpeg failed smoke-test: %w", err)
	}
	if err := runVersion(ctx, ffprobe, "-version"); err != nil {
		return "", "", fmt.Errorf("downloaded ffprobe failed smoke-test: %w", err)
	}
	return ffmpeg, ffprobe, nil
}

func EnsureONNXRuntime(ctx context.Context) (string, error) {
	if path, err := rayonnx.ResolveRuntimeLibrary(); err == nil && rayonnx.TestRuntime(path) == nil {
		return path, nil
	}
	return EnsureManagedONNXRuntime(ctx)
}

// EnsureManagedONNXRuntime guarantees that the compatible runtime exists in
// the per-user managed assets directory, even when a system runtime is usable.
// Portable builds use this path; the application/Doctor still prefer a valid
// configured or system runtime and only repair into managed assets when needed.
func EnsureManagedONNXRuntime(ctx context.Context) (string, error) {
	asset, ok := OnnxRuntimeAsset(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return "", fmt.Errorf("managed ONNX Runtime %s unsupported for %s/%s", ONNXRuntimeVersion, runtime.GOOS, runtime.GOARCH)
	}
	dir, err := appdirs.ManagedONNXRuntimeDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, asset.Library)
	if regularFile(dst) {
		if err := rayonnx.TestRuntime(dst); err == nil {
			return dst, nil
		}
		_ = os.Remove(dst)
	}
	archivePath := filepath.Join(dir, asset.Archive+".download")
	url := fmt.Sprintf("https://github.com/microsoft/onnxruntime/releases/download/v%s/%s", ONNXRuntimeVersion, asset.Archive)
	if err := downloadFile(ctx, url, archivePath); err != nil {
		return "", err
	}
	defer os.Remove(archivePath)
	if asset.Zip {
		err = extractZipMember(archivePath, asset.Member, dst)
	} else {
		err = extractTarGzipMember(archivePath, asset.Member, dst)
	}
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dst, 0o755)
	}
	if err := rayonnx.TestRuntime(dst); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("downloaded ONNX Runtime failed smoke-test: %w", err)
	}
	return dst, nil
}

func ManagedONNXRuntimePath() (string, error) {
	asset, ok := OnnxRuntimeAsset(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return "", fmt.Errorf("managed ONNX Runtime %s unsupported for %s/%s", ONNXRuntimeVersion, runtime.GOOS, runtime.GOARCH)
	}
	dir, err := appdirs.ManagedONNXRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, asset.Library), nil
}

func EnsureMiniLM(ctx context.Context, configured string) (string, error) {
	dir := strings.TrimSpace(configured)
	if dir == "" {
		var err error
		dir, err = appdirs.ManagedMiniLMDir()
		if err != nil {
			return "", err
		}
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if info, statErr := os.Stat(absolute); statErr == nil && !info.IsDir() {
		return "", fmt.Errorf("MiniLM path is not a directory: %s", absolute)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", err
	}
	files := []struct{ Name, URL string }{
		{"model.onnx", huggingFaceURL("onnx/model.onnx")},
		{"tokenizer.json", huggingFaceURL("tokenizer.json")},
	}
	for _, file := range files {
		dst := filepath.Join(absolute, file.Name)
		if regularFile(dst) && (file.Name != "tokenizer.json" || validateJSONFile(dst) == nil) {
			continue
		}
		_ = os.Remove(dst)
		if err := downloadFile(ctx, file.URL, dst); err != nil {
			return "", fmt.Errorf("download %s: %w", file.Name, err)
		}
	}
	if err := validateJSONFile(filepath.Join(absolute, "tokenizer.json")); err != nil {
		return "", err
	}
	if _, _, err := rayonnx.ResolveModelFiles(absolute); err != nil {
		return "", fmt.Errorf("validate MiniLM bundle: %w", err)
	}
	return absolute, nil
}

// EnsureEssentia downloads the complete runtime model set into the managed
// application assets directory when no valid configured bundle is available.
func EnsureEssentia(ctx context.Context, configured string) (string, error) {
	return ensureEssentia(ctx, configured, false)
}

func ensureEssentia(ctx context.Context, configured string, force bool) (string, error) {
	if !force {
		if dir, err := rayonnx.ResolveEssentiaModelDir(configured); err == nil {
			return dir, nil
		}
	}
	dir, err := appdirs.ManagedEssentiaDir()
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", err
	}
	if !force {
		if ready, resolveErr := rayonnx.ResolveEssentiaModelDir(absolute); resolveErr == nil {
			return ready, nil
		}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(essentiaModelBaseURLEnv)), "/")
	if baseURL == "" {
		baseURL = EssentiaModelBaseURL
	}
	for _, name := range rayonnx.RequiredEssentiaFiles() {
		dst := filepath.Join(absolute, name)
		if !force && regularFile(dst) {
			if filepath.Ext(name) != ".json" || validateJSONFile(dst) == nil {
				continue
			}
		}
		_ = os.Remove(dst)
		if err := downloadFile(ctx, baseURL+"/"+name, dst); err != nil {
			return "", fmt.Errorf("download Essentia model %s: %w", name, err)
		}
		if filepath.Ext(name) == ".json" {
			if err := validateJSONFile(dst); err != nil {
				_ = os.Remove(dst)
				return "", err
			}
		}
	}
	ready, err := rayonnx.ResolveEssentiaModelDir(absolute)
	if err != nil {
		return "", fmt.Errorf("validate Essentia bundle: %w", err)
	}
	return ready, nil
}

func ffmpegAssetFor(goos, goarch string) (ffmpegAsset, bool) {
	base := "https://github.com/eugeneware/ffmpeg-static/releases/download/" + FFmpegStaticTag + "/"
	name := ""
	switch goos + "/" + goarch {
	case "darwin/arm64":
		name = "darwin-arm64"
	case "darwin/amd64":
		name = "darwin-x64"
	case "linux/arm64":
		name = "linux-arm64"
	case "linux/amd64":
		name = "linux-x64"
	case "windows/amd64":
		name = "win32-x64"
	case "windows/arm64":
		return ffmpegAsset{ZipURL: "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-winarm64-gpl.zip"}, true
	default:
		return ffmpegAsset{}, false
	}
	return ffmpegAsset{
		FFmpegURL:  base + "ffmpeg-" + name + ".gz",
		FFprobeURL: base + "ffprobe-" + name + ".gz",
	}, true
}

func OnnxRuntimeAsset(goos, goarch string) (runtimeAsset, bool) {
	prefix := "onnxruntime-"
	switch goos + "/" + goarch {
	case "darwin/arm64":
		archive := prefix + "osx-arm64-" + ONNXRuntimeVersion + ".tgz"
		return runtimeAsset{Archive: archive, Member: "/lib/libonnxruntime." + ONNXRuntimeVersion + ".dylib", Library: "libonnxruntime." + ONNXRuntimeVersion + ".dylib"}, true
	case "darwin/amd64":
		// Microsoft stopped publishing macOS x86_64 binaries starting with
		// ONNX Runtime 1.24. Ray can still use a compatible user/system-built
		// library when configured, but Doctor must not offer a broken download.
		return runtimeAsset{}, false
	case "linux/amd64":
		archive := prefix + "linux-x64-" + ONNXRuntimeVersion + ".tgz"
		return runtimeAsset{Archive: archive, Member: "/lib/libonnxruntime.so." + ONNXRuntimeVersion, Library: "libonnxruntime.so." + ONNXRuntimeVersion}, true
	case "linux/arm64":
		archive := prefix + "linux-aarch64-" + ONNXRuntimeVersion + ".tgz"
		return runtimeAsset{Archive: archive, Member: "/lib/libonnxruntime.so." + ONNXRuntimeVersion, Library: "libonnxruntime.so." + ONNXRuntimeVersion}, true
	case "windows/amd64":
		archive := prefix + "win-x64-" + ONNXRuntimeVersion + ".zip"
		return runtimeAsset{Archive: archive, Member: "/lib/onnxruntime.dll", Library: "onnxruntime.dll", Zip: true}, true
	case "windows/arm64":
		archive := prefix + "win-arm64-" + ONNXRuntimeVersion + ".zip"
		return runtimeAsset{Archive: archive, Member: "/lib/onnxruntime.dll", Library: "onnxruntime.dll", Zip: true}, true
	default:
		return runtimeAsset{}, false
	}
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isCommandDefault(value, name string) bool {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, `/\\`) {
		return false
	}
	base := strings.ToLower(value)
	return base == name || base == name+".exe"
}

func siblingTool(ffmpeg, tool string) string {
	return filepath.Join(filepath.Dir(ffmpeg), executableName(tool))
}

func resolveExecutable(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("empty executable path")
	}
	if filepath.IsAbs(value) || strings.ContainsAny(value, `/\`) {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return "", err
		}
		if !regularFile(absolute) {
			return "", fmt.Errorf("executable missing: %s", absolute)
		}
		return absolute, nil
	}
	path, err := exec.LookPath(value)
	if err != nil {
		return "", err
	}
	return path, nil
}

func runVersion(ctx context.Context, path string, args ...string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(probeCtx, path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func huggingFaceURL(path string) string {
	return fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s?download=true", MiniLMRepository, path)
}

func downloadGzipBinary(ctx context.Context, url, dst string) error {
	tmp := dst + ".download.gz"
	if err := downloadFile(ctx, url, tmp); err != nil {
		return err
	}
	defer os.Remove(tmp)
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return writeReaderAtomically(dst, gz, 0o755)
}

func downloadFile(ctx context.Context, url, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	part := dst + ".part"
	_ = os.Remove(part)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ray-player-doctor/1")
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
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(part)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if err := replaceFile(part, dst); err != nil {
		_ = os.Remove(part)
		return err
	}
	return nil
}

func copyWithLimit(dst io.Writer, src io.Reader, max int64) (int64, error) {
	limited := &io.LimitedReader{R: src, N: max + 1}
	n, err := io.Copy(dst, limited)
	if err != nil {
		return n, err
	}
	if n > max {
		return n, fmt.Errorf("download exceeded %d bytes", max)
	}
	return n, nil
}

func writeReaderAtomically(dst string, reader io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	part := dst + ".part"
	_ = os.Remove(part)
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := copyWithLimit(f, reader, maxDownloadBytes)
	syncErr := f.Sync()
	closeErr := f.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(part)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(part, mode)
	}
	if err := replaceFile(part, dst); err != nil {
		_ = os.Remove(part)
		return err
	}
	return nil
}

func replaceFile(src, dst string) error {
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(src, dst)
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
	return fmt.Errorf("archive member %q not found", memberSuffix)
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
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = writeReaderAtomically(dst, rc, 0o755)
		closeErr := rc.Close()
		return errors.Join(err, closeErr)
	}
	return fmt.Errorf("archive member %q not found", memberSuffix)
}

func validateJSONFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("invalid JSON %s: %w", path, err)
	}
	return nil
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
