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
	TempoHopSize     = 512
	TempoMelBands    = 40
	TempoPatchFrames = 256
	TempoPatchHop    = 128
)

func ExtractTempoPatches(path string) ([][]float32, error) {
	return ExtractTempoPatchesWithContext(context.Background(), path)
}

func ExtractTempoPatchesWithContext(ctx context.Context, path string) ([][]float32, error) {
	cfg := DefaultFFmpegConfig()
	cfg.SampleRate = TempoSampleRate
	cfg.Duration = 90 * time.Second
	cfg.Timeout = 90 * time.Second
	raw, sr, _, err := DecodeMonoFloat32WithFFmpeg(ctx, path, cfg)
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
	standardizeTempoPatches(patches)
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
		melFilters: buildTempoCNNMelFilterbank(),
	}
	// Essentia TensorflowInputTempoCNN hardcodes MelBands type=magnitude.
	// Do not reuse the EffNet/MusiCNN power-spectrum frontend here.
	return processor.ProcessMagnitude(samples)
}

func buildTempoCNNMelFilterbank() [][]float64 {
	const (
		lowHz  = 20.0
		highHz = 5000.0
	)
	half := TempoFFTSize/2 + 1
	lowMel := hzToSlaneyMel(lowHz)
	highMel := hzToSlaneyMel(highHz)
	freqs := make([]float64, TempoMelBands+2)
	for i := range freqs {
		mel := lowMel + (highMel-lowMel)*float64(i)/float64(len(freqs)-1)
		freqs[i] = slaneyMelToHz(mel)
	}

	filters := make([][]float64, TempoMelBands)
	freqScale := (float64(TempoSampleRate) / 2) / float64(half-1)
	for band := 0; band < TempoMelBands; band++ {
		left, center, right := freqs[band], freqs[band+1], freqs[band+2]
		filters[band] = make([]float64, half)
		unitTriArea := (center - left + right - center) / 2
		if unitTriArea <= 0 {
			continue
		}
		start := int(math.Ceil(left / freqScale))
		end := int(math.Floor(right / freqScale))
		if start < 0 {
			start = 0
		}
		if end >= half {
			end = half - 1
		}
		for bin := start; bin <= end; bin++ {
			freq := float64(bin) * freqScale
			weight := 0.0
			if freq < center {
				weight = (freq - left) / (center - left)
			} else {
				weight = (right - freq) / (right - center)
			}
			if weight > 0 {
				filters[band][bin] = weight / unitTriArea
			}
		}
	}
	return filters
}

func standardizeTempoPatches(patches [][]float32) {
	if len(patches) == 0 {
		return
	}
	width := len(patches[0])
	if width == 0 {
		return
	}
	for _, patch := range patches {
		if len(patch) != width {
			return
		}
	}

	// Essentia TensorflowPredictTempoCNN first builds [B,1,256,40] and then
	// applies TensorNormalize(scaler="standard") with the algorithm's
	// default axis=0. Therefore each time/mel coordinate is standardized over
	// the patch batch, not over the values inside one patch.
	for i := 0; i < width; i++ {
		mean := 0.0
		for _, patch := range patches {
			mean += float64(patch[i])
		}
		mean /= float64(len(patches))

		variance := 0.0
		for _, patch := range patches {
			delta := float64(patch[i]) - mean
			variance += delta * delta
		}
		std := math.Sqrt(variance / float64(len(patches)))
		if std <= 1e-12 {
			for _, patch := range patches {
				patch[i] = 0
			}
			continue
		}
		for _, patch := range patches {
			patch[i] = float32((float64(patch[i]) - mean) / std)
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
