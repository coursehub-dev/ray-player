package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ray-player1/internal/audio"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: audio_probe <file-or-directory>\n")
		os.Exit(2)
	}
	root := os.Args[1]
	info, err := os.Stat(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat failed: %v\n", err)
		os.Exit(1)
	}

	var files []string
	if info.IsDir() {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus", ".wma":
				files = append(files, path)
			}
			return nil
		})
	} else {
		files = []string{root}
	}

	if len(files) == 0 {
		fmt.Println("no audio files found")
		return
	}

	okCount, failCount := 0, 0
	for _, path := range files {
		stream, format, closer, backend, err := audio.DecodeForPlayback(path)
		if err != nil {
			failCount++
			fmt.Printf("FAIL path=%q err=%v\n", path, err)
			continue
		}
		if probeErr := probe(stream); probeErr != nil {
			failCount++
			fmt.Printf("FAIL path=%q backend=%s probe=%v\n", path, backend, probeErr)
			_ = closer.Close()
			continue
		}
		okCount++
		fmt.Printf("OK path=%q backend=%s sr=%d ch=%d len=%d\n", path, backend, format.SampleRate, format.NumChannels, stream.Len())
		_ = closer.Close()
	}
	fmt.Printf("summary ok=%d fail=%d total=%d\n", okCount, failCount, len(files))
	if failCount > 0 {
		os.Exit(1)
	}
}

func probe(stream interface {
	Stream([][2]float64) (int, bool)
	Err() error
	Seek(int) error
}) error {
	buf := make([][2]float64, 512)
	n, ok := stream.Stream(buf)
	if err := stream.Err(); err != nil {
		return err
	}
	if n == 0 && !ok {
		return fmt.Errorf("empty stream")
	}
	if err := stream.Seek(0); err != nil {
		if n > 0 {
			return nil
		}
		return err
	}
	return nil
}
