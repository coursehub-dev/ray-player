package externalmedia

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var progressPattern = regexp.MustCompile(
	`\[download\]\s+([0-9]+(?:\.[0-9]+)?)%`,
)

type DownloadRequest struct {
	URL            string
	OutputDir      string
	OutputTemplate string
	Bitrate        int
	FFmpegPath     string
}

type DownloadCallbacks struct {
	OnLine       func(string)
	OnProgress   func(float64)
	OnConverting func()
}

type Client struct {
	path string
}

func NewClient(path string) *Client {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "yt-dlp"
	}
	return &Client{path: path}
}

func (c *Client) Test(
	ctx context.Context,
) ToolCheckResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	output, err := exec.CommandContext(
		ctx,
		c.path,
		"--version",
	).CombinedOutput()

	result := ToolCheckResult{
		Output: strings.TrimSpace(string(output)),
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.OK = true
	result.Version = strings.TrimSpace(string(output))
	return result
}

func (c *Client) FetchMetadata(
	parent context.Context,
	rawURL string,
) (Metadata, error) {
	ctx, cancel := context.WithTimeout(
		parent,
		45*time.Second,
	)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(
		ctx,
		c.path,
		"--dump-single-json",
		"--no-playlist",
		"--no-warnings",
		rawURL,
	)
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if ctx.Err() != nil {
		return Metadata{}, fmt.Errorf(
			"yt-dlp metadata timeout: %w",
			ctx.Err(),
		)
	}
	if err != nil {
		return Metadata{}, fmt.Errorf(
			"yt-dlp metadata failed: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	var metadata Metadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return Metadata{}, fmt.Errorf(
			"parse yt-dlp metadata: %w",
			err,
		)
	}

	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.Extractor = strings.TrimSpace(metadata.Extractor)
	metadata.ID = strings.TrimSpace(metadata.ID)
	if metadata.Title == "" {
		return Metadata{}, errors.New(
			"yt-dlp returned metadata without title",
		)
	}
	if metadata.WebpageURL == "" {
		metadata.WebpageURL = rawURL
	}
	return metadata, nil
}

func (c *Client) BuildDownloadArgs(
	request DownloadRequest,
) []string {
	args := []string{
		"--no-playlist",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality",
		fmt.Sprintf("%dK", request.Bitrate),
		"--embed-metadata",
		"--restrict-filenames",
		"--newline",
		"--no-part",
		"-o", request.OutputTemplate,
	}

	if ffmpeg := strings.TrimSpace(request.FFmpegPath); ffmpeg != "" {
		args = append(
			args,
			"--ffmpeg-location",
			ffmpeg,
		)
	}

	args = append(args, request.URL)
	return args
}

func (c *Client) Download(
	parent context.Context,
	request DownloadRequest,
	callbacks DownloadCallbacks,
) error {
	ctx, cancel := context.WithTimeout(
		parent,
		30*time.Minute,
	)
	defer cancel()

	if err := ensureDirectory(request.OutputDir); err != nil {
		return err
	}

	args := c.BuildDownloadArgs(request)
	cmd := exec.CommandContext(ctx, c.path, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start yt-dlp: %w", err)
	}

	var scanWG sync.WaitGroup
	scanWG.Add(2)
	go func() {
		defer scanWG.Done()
		scanOutput(stdout, callbacks)
	}()
	go func() {
		defer scanWG.Done()
		scanOutput(stderr, callbacks)
	}()

	waitErr := cmd.Wait()
	scanWG.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr != nil {
		return fmt.Errorf("yt-dlp download failed: %w", waitErr)
	}
	return nil
}

func scanOutput(
	reader io.Reader,
	callbacks DownloadCallbacks,
) {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if callbacks.OnLine != nil {
			callbacks.OnLine(line)
		}

		if progress := ParseProgress(line); progress >= 0 &&
			callbacks.OnProgress != nil {
			callbacks.OnProgress(progress)
		}

		lower := strings.ToLower(line)
		if strings.Contains(lower, "[extractaudio]") ||
			strings.Contains(lower, "[ffmpeg]") ||
			strings.Contains(lower, "destination:") &&
				strings.Contains(lower, ".mp3") {
			if callbacks.OnConverting != nil {
				callbacks.OnConverting()
			}
		}
	}
}

func ParseProgress(line string) float64 {
	match := progressPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return -1
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return -1
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value / 100
}

func FindOutputMP3(
	outputDir string,
	metadata Metadata,
) (string, error) {
	expected := filepath.Join(
		outputDir,
		OutputFilename(metadata),
	)
	if fileExists(expected) {
		return expected, nil
	}

	matches, err := filepath.Glob(filepath.Join(
		outputDir,
		"*"+sanitizeFilenamePart(metadata.ID)+"*.mp3",
	))
	if err != nil {
		return "", err
	}
	for _, match := range matches {
		if fileExists(match) {
			return match, nil
		}
	}
	return "", fmt.Errorf(
		"yt-dlp output mp3 not found in %q",
		outputDir,
	)
}
