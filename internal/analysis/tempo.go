package analysis

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"
)

const (
	TempoSampleRate  = 11025
	TempoFFTSize     = 1024
	TempoHopSize     = 256
	TempoMelBands    = 40
	TempoPatchFrames = 256
	TempoPatchHop    = 64
)

func ExtractTempoPatches(path string) ([][]float32, error) {
	cfg := DefaultFFmpegConfig()
	cfg.SampleRate = TempoSampleRate
	cfg.Duration = 90 * time.Second
	cfg.Timeout = 90 * time.Second
	raw, sr, _, err := DecodeMonoFloat32WithFFmpeg(context.Background(), path, cfg)
	if err != nil {
		return nil, err
	}
	if sr != TempoSampleRate {
		return nil, errors.New("tempo decode sample rate mismatch")
	}
	samples := make([]float64, len(raw))
	for i, v := range raw {
		samples[i] = float64(v)
	}
	frames := extractTempoFrames(samples)
	if len(frames) < TempoPatchFrames {
		return nil, errors.New("too few tempo frames")
	}
	patches := make([][]float32, 0, 1+(len(frames)-TempoPatchFrames)/TempoPatchHop)
	for start := 0; start+TempoPatchFrames <= len(frames); start += TempoPatchHop {
		patch := make([]float32, 0, TempoPatchFrames*TempoMelBands)
		for i := 0; i < TempoPatchFrames; i++ {
			patch = append(patch, frames[start+i]...)
		}
		patches = append(patches, patch)
	}
	if len(patches) == 0 {
		return nil, errors.New("too few tempo patches")
	}
	return patches, nil
}

func extractTempoFrames(samples []float64) [][]float32 {
	if len(samples) < TempoFFTSize {
		return nil
	}
	processor := &MelProcessor{
		fftSize:    TempoFFTSize,
		hopSize:    TempoHopSize,
		melBands:   TempoMelBands,
		hannWin:    buildHannWindow(TempoFFTSize),
		melFilters: buildMelFilterbank(TempoMelBands, TempoFFTSize, TempoSampleRate),
	}
	frames := processor.ProcessEnergy(samples)
	if len(frames) == 0 {
		return nil
	}
	normalizeTempoFrames(frames)
	return frames
}

func normalizeTempoFrames(frames [][]float32) {
	if len(frames) == 0 {
		return
	}
	for band := 0; band < len(frames[0]); band++ {
		mean := 0.0
		for i := range frames {
			mean += float64(frames[i][band])
		}
		mean /= float64(len(frames))
		variance := 0.0
		for i := range frames {
			d := float64(frames[i][band]) - mean
			variance += d * d
		}
		std := math.Sqrt(variance/float64(len(frames)) + 1e-9)
		for i := range frames {
			v := (math.Log1p(float64(frames[i][band])) - mean) / std
			frames[i][band] = float32(v)
		}
	}
}

func buildHannWindow(size int) []float64 {
	if size <= 0 {
		return nil
	}
	window := make([]float64, size)
	for i := range window {
		window[i] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(size-1)))
	}
	return window
}

func NormalizePerceivedBPM(bpm float64) float64 {
	if bpm <= 0 {
		return 0
	}
	for bpm < 70 {
		bpm *= 2
	}
	for bpm > 180 {
		bpm /= 2
	}
	return bpm
}

func TempoDistanceNormalized(a, b float64) float64 {
	if a <= 0 || b <= 0 {
		return 1
	}
	candidates := []float64{
		math.Abs(a-b) / math.Max(a, b),
		math.Abs(a*2-b) / math.Max(a*2, b),
		math.Abs(a*0.5-b) / math.Max(a*0.5, b),
		math.Abs(a-b*2) / math.Max(a, b*2),
		math.Abs(a-b*0.5) / math.Max(a, b*0.5),
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate < best {
			best = candidate
		}
	}
	return best
}

func TempoStability(local []float64, global float64) float64 {
	if len(local) == 0 || global <= 0 {
		return 0
	}
	matched := 0
	for _, bpm := range local {
		if TempoDistanceNormalized(bpm, global) <= 0.04 {
			matched++
		}
	}
	return float64(matched) / float64(len(local))
}

func TempoConfidenceMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cloned := append([]float64{}, values...)
	sort.Float64s(cloned)
	mid := len(cloned) / 2
	if len(cloned)%2 == 1 {
		return cloned[mid]
	}
	return (cloned[mid-1] + cloned[mid]) * 0.5
}
