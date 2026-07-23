package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/onnx"
)

func main() {
	audioPath := flag.String("audio", "", "path to audio file")
	runtimePath := flag.String("runtime", "", "path to libonnxruntime.dylib")
	modelsDir := flag.String("models", "", "path to essentia models dir")
	melMode := flag.String("mel-mode", string(analysis.DefaultMelMode()), "official|current|log1p|db|unit|essentia-like")
	outPath := flag.String("out", "/tmp/ray_model_probe.json", "output json path")
	ffmpegPath := flag.String("ffmpeg", "", "path to ffmpeg binary")
	flag.Parse()

	if *audioPath == "" || *runtimePath == "" || *modelsDir == "" {
		log.Fatal("usage: model_probe -audio FILE -runtime LIB -models DIR [-mel-mode MODE] [-out FILE] [-ffmpeg BIN]")
	}
	if *ffmpegPath != "" {
		analysis.SetFFmpegPath(*ffmpegPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	report, err := onnx.ProbeEssentiaDetailed(ctx, *runtimePath, *modelsDir, *audioPath, *melMode)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*outPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := onnx.WriteProbeReport(f.Name(), report); err != nil {
		log.Fatal(err)
	}
	fmt.Println(f.Name())
}
