package audio

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func ProbeDuration(path string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		4*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		FFprobePath(),
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if ctx.Err() != nil {
		return 0, fmt.Errorf(
			"ffprobe duration timeout: %w",
			ctx.Err(),
		)
	}
	if err != nil {
		return 0, fmt.Errorf(
			"ffprobe duration failed: %w; stderr=%s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	raw := strings.TrimSpace(string(output))
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf(
			"invalid ffprobe duration %q",
			raw,
		)
	}

	return time.Duration(seconds * float64(time.Second)), nil
}
