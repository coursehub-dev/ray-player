package analysis

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/madelynnblue/go-dsp/fft"
)

const (
	EssentiaSampleRate  = 16000
	EssentiaFFTSize     = 512
	EssentiaHopSize     = 256
	EssentiaMelBands    = 96
	EssentiaPatchFrames = 128
)

type MelMode string

const (
	MelModeOfficial MelMode = "official"
	MelModeCurrent  MelMode = "current" // alias for official
	MelModeLog1P    MelMode = "log1p"
	MelModeDB       MelMode = "db"
	MelModeUnit     MelMode = "unit"
)

func DefaultMelMode() MelMode { return MelModeOfficial }

type MelProcessor struct {
	fftSize    int
	hopSize    int
	melBands   int
	hannWin    []float64
	melFilters [][]float64
}

func NewMelProcessor() *MelProcessor {
	hann := make([]float64, EssentiaFFTSize)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(EssentiaFFTSize-1)))
	}
	return &MelProcessor{
		fftSize:    EssentiaFFTSize,
		hopSize:    EssentiaHopSize,
		melBands:   EssentiaMelBands,
		hannWin:    hann,
		melFilters: buildMelFilterbank(EssentiaMelBands, EssentiaFFTSize, EssentiaSampleRate),
	}
}

// IMPORTANT:
// MusiCNN/EffNet Discogs models are trained on Essentia's TensorflowInputMusiCNN
// frontend: 16 kHz, 512/256 STFT, 96 Slaney mel bands, linear triangles with
// unit-triangle normalization, power spectrum, then log10(1 + 10000 * energy).
// Experimental transforms belong only to ExtractMelSpectrogramMode/model probes.
func ExtractMelSpectrogram(path string) ([]float32, int, error) {
	return ExtractMelSpectrogramWithContext(context.Background(), path)
}

type MelAnalysisWindow struct {
	Start    time.Duration
	Duration time.Duration
}

func ExtractMelSpectrogramWithContext(
	ctx context.Context,
	path string,
) ([]float32, int, error) {
	duration := time.Duration(0)
	if durationMs, err := DecodeAudioDuration(ctx, path); err == nil && durationMs > 0 {
		duration = time.Duration(durationMs) * time.Millisecond
	}

	windows := SelectMelAnalysisWindows(duration)
	analysisLog.I(
		"mel windows path=%q duration=%.2fs windows=%d",
		path,
		duration.Seconds(),
		len(windows),
	)
	all := make([]float32, 0)
	totalPatches := 0
	for index, window := range windows {
		frames, err := extractMelFramesWindow(
			ctx,
			path,
			string(MelModeOfficial),
			window.Start,
			window.Duration,
		)
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, ctx.Err()
			}
			continue
		}
		patched, patches, err := makeMelPatches(frames)
		if err != nil {
			analysisLog.W(
				"mel window skipped path=%q index=%d start=%.2fs duration=%.2fs err=%v",
				path, index, window.Start.Seconds(), window.Duration.Seconds(), err,
			)
			continue
		}
		analysisLog.D(
			"mel window ok path=%q index=%d start=%.2fs duration=%.2fs frames=%d patches=%d",
			path, index, window.Start.Seconds(), window.Duration.Seconds(), len(frames)/EssentiaMelBands, patches,
		)
		all = append(all, patched...)
		totalPatches += patches
	}
	if totalPatches == 0 {
		return nil, 0, errors.New("too few mel frames")
	}
	analysisLog.I("mel ready path=%q patches=%d values=%d", path, totalPatches, len(all))
	return all, totalPatches, nil
}

func SelectMelAnalysisWindows(total time.Duration) []MelAnalysisWindow {
	const (
		singleWindow = 45 * time.Second
		segment      = 9 * time.Second
		windowCount  = 5
	)
	if total <= 0 {
		return []MelAnalysisWindow{{Duration: singleWindow}}
	}
	if total <= singleWindow {
		return []MelAnalysisWindow{{Duration: total}}
	}

	// Keep the same 45-second inference budget, but distribute it across the
	// whole song. Start/middle/end-only sampling systematically misses drops,
	// bridges and late high-energy sections in structurally complex tracks.
	lastStart := total - segment
	windows := make([]MelAnalysisWindow, 0, windowCount)
	for i := 0; i < windowCount; i++ {
		start := time.Duration(int64(lastStart) * int64(i) / int64(windowCount-1))
		windows = append(windows, MelAnalysisWindow{Start: start, Duration: segment})
	}
	return windows
}

func ExtractMelSpectrogramMode(path, mode string) ([]float32, int, error) {
	frames, err := ExtractMelFramesMode(path, mode)
	if err != nil {
		return nil, 0, err
	}
	return makeMelPatches(frames)
}

func makeMelPatches(frames []float32) ([]float32, int, error) {
	frameCount := len(frames) / EssentiaMelBands
	if frameCount < EssentiaPatchFrames {
		return nil, 0, errors.New("too few mel frames")
	}
	const patchHop = 62
	patches := 1 + (frameCount-EssentiaPatchFrames)/patchHop
	flat := make([]float32, 0, patches*EssentiaMelBands*EssentiaPatchFrames)
	for p := 0; p < patches; p++ {
		startFrame := p * patchHop
		endFrame := startFrame + EssentiaPatchFrames
		if endFrame > frameCount {
			break
		}
		chunk := frames[startFrame*EssentiaMelBands : endFrame*EssentiaMelBands]
		flat = append(flat, chunk...)
	}
	return flat, patches, nil
}

func ExtractMelFrames(path string) ([]float32, error) {
	return ExtractMelFramesMode(path, string(MelModeOfficial))
}

func ExtractMelFramesMode(path, mode string) ([]float32, error) {
	frames, err := ExtractMelEnergyFrames(path)
	if err != nil {
		return nil, err
	}
	if mode == "" || mode == string(MelModeCurrent) {
		mode = string(MelModeOfficial)
	}
	flat := make([]float32, 0, len(frames)*EssentiaMelBands)
	for _, frame := range frames {
		flat = append(flat, transformMelFrame(frame, mode)...)
	}
	if len(flat) == 0 {
		return nil, errors.New("too few mel frames")
	}
	return flat, nil
}

func extractMelFramesWindow(
	ctx context.Context,
	path string,
	mode string,
	start time.Duration,
	duration time.Duration,
) ([]float32, error) {
	resampled, err := ExtractResampledMonoWindowAt(ctx, path, start, duration)
	if err != nil {
		return nil, err
	}
	if len(resampled) < EssentiaFFTSize {
		return nil, errors.New("too few samples for mel extraction")
	}
	frames := NewMelProcessor().ProcessEnergy(resampled)
	flat := make([]float32, 0, len(frames)*EssentiaMelBands)
	for _, frame := range frames {
		flat = append(flat, transformMelFrame(frame, mode)...)
	}
	if len(flat) == 0 {
		return nil, errors.New("too few mel frames")
	}
	return flat, nil
}

func transformMelFrame(frame []float32, mode string) []float32 {
	out := make([]float32, len(frame))
	switch mode {
	case string(MelModeOfficial):
		for i, v := range frame {
			out[i] = float32(math.Log10(1.0 + 10000.0*math.Max(0, float64(v))))
		}
	case string(MelModeCurrent):
		for i, v := range frame {
			out[i] = float32(math.Log10(1.0 + 10000.0*math.Max(0, float64(v))))
		}
	case string(MelModeLog1P), "essentia-like":
		for i, v := range frame {
			out[i] = float32(math.Log1p(float64(v)))
		}
	case string(MelModeDB):
		maxV := 0.0
		for _, v := range frame {
			if float64(v) > maxV {
				maxV = float64(v)
			}
		}
		ref := maxV
		if ref <= 0 {
			ref = 1e-9
		}
		for i, v := range frame {
			out[i] = float32(10 * math.Log10(math.Max(1e-9, float64(v))/ref))
		}
	case string(MelModeUnit):
		if len(frame) == 0 {
			return out
		}
		minV, maxV := frame[0], frame[0]
		for _, v := range frame {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
		rangeV := maxV - minV
		if rangeV <= 0 {
			copy(out, frame)
			return out
		}
		for i, v := range frame {
			out[i] = (v - minV) / rangeV
		}
	default:
		copy(out, frame)
	}
	return out
}

func ExtractMelEnergyFrames(path string) ([][]float32, error) {
	resampled, err := ExtractResampledMonoWindow(path, 45*time.Second)
	if err != nil {
		return nil, err
	}
	if len(resampled) < EssentiaFFTSize {
		return nil, errors.New("too few samples for mel extraction")
	}
	processor := NewMelProcessor()
	frames := processor.ProcessEnergy(resampled)
	if len(frames) == 0 {
		return nil, errors.New("too few mel frames")
	}
	return frames, nil
}

func ExtractResampledMonoWindow(path string, dur time.Duration) ([]float64, error) {
	return ExtractResampledMonoWindowAt(context.Background(), path, 0, dur)
}

func ExtractResampledMonoWindowAt(
	ctx context.Context,
	path string,
	start time.Duration,
	dur time.Duration,
) ([]float64, error) {
	cfg := DefaultFFmpegConfig()
	cfg.Start = start
	cfg.Duration = dur
	raw, sr, _, err := DecodeMonoFloat32WithFFmpeg(ctx, path, cfg)
	if err != nil {
		return nil, err
	}
	if sr <= 0 || len(raw) == 0 {
		return nil, errors.New("empty mono window")
	}
	mono := make([]float64, len(raw))
	for i, v := range raw {
		mono[i] = float64(v)
	}
	return mono, nil
}

func (p *MelProcessor) Process(samples []float64) [][]float32 {
	energy := p.ProcessEnergy(samples)
	if len(energy) == 0 {
		return nil
	}
	out := make([][]float32, len(energy))
	for i := range energy {
		out[i] = transformMelFrame(energy[i], string(MelModeOfficial))
	}
	return out
}

func (p *MelProcessor) ProcessEnergy(samples []float64) [][]float32 {
	frameCount := 1 + (len(samples)-p.fftSize)/p.hopSize
	if frameCount <= 0 {
		return nil
	}
	out := make([][]float32, 0, frameCount)
	for start := 0; start+p.fftSize <= len(samples); start += p.hopSize {
		frame := make([]float64, p.fftSize)
		for i := 0; i < p.fftSize; i++ {
			frame[i] = samples[start+i] * p.hannWin[i]
		}
		spec := fftMagnitudes(frame)
		mel := applyMelFiltersEnergy(spec, p.melFilters)
		out = append(out, mel)
	}
	return out
}

func readMonoWindow(streamer beep.Streamer, sr beep.SampleRate, dur time.Duration) []float64 {
	need := sr.N(dur)
	buf := make([][2]float64, 1024)
	mono := make([]float64, 0, need)
	for len(mono) < need {
		n, ok := streamer.Stream(buf)
		for i := 0; i < n && len(mono) < need; i++ {
			mono = append(mono, (buf[i][0]+buf[i][1])*0.5)
		}
		if !ok {
			break
		}
	}
	return mono
}

func resampleLinear(in []float64, srcRate, dstRate int) []float64 {
	if srcRate <= 0 || dstRate <= 0 || len(in) == 0 || srcRate == dstRate {
		return in
	}
	outLen := int(float64(len(in)) * float64(dstRate) / float64(srcRate))
	if outLen <= 1 {
		return in
	}
	out := make([]float64, outLen)
	ratio := float64(srcRate) / float64(dstRate)
	for i := 0; i < outLen; i++ {
		pos := float64(i) * ratio
		idx := int(pos)
		frac := pos - float64(idx)
		if idx >= len(in)-1 {
			out[i] = in[len(in)-1]
			continue
		}
		out[i] = in[idx]*(1-frac) + in[idx+1]*frac
	}
	return out
}

func fftMagnitudes(frame []float64) []float64 {
	spec := fft.FFTReal(frame)
	half := len(frame)/2 + 1
	if len(spec) < half {
		half = len(spec)
	}
	out := make([]float64, half)
	for i := 0; i < half; i++ {
		re := real(spec[i])
		im := imag(spec[i])
		out[i] = math.Sqrt(re*re + im*im)
	}
	return out
}

func applyMelFilters(spec []float64, filters [][]float64) []float32 {
	out := make([]float32, len(filters))
	for m := range filters {
		var energy float64
		for i := range spec {
			energy += spec[i] * filters[m][i]
		}
		out[m] = float32(math.Log1p(energy))
	}
	return out
}

func applyMelFiltersEnergy(spec []float64, filters [][]float64) []float32 {
	out := make([]float32, len(filters))
	for m := range filters {
		var energy float64
		for i := range spec {
			// Essentia MelBands defaults to type=power: the filterbank consumes
			// squared magnitude even though Spectrum itself returns magnitude.
			energy += spec[i] * spec[i] * filters[m][i]
		}
		out[m] = float32(energy)
	}
	return out
}

func buildMelFilterbank(melBands, fftSize, sampleRate int) [][]float64 {
	half := fftSize/2 + 1
	lowMel := hzToSlaneyMel(0)
	highMel := hzToSlaneyMel(float64(sampleRate) / 2)
	melPoints := make([]float64, melBands+2)
	hzPoints := make([]float64, melBands+2)
	for i := range melPoints {
		melPoints[i] = lowMel + (highMel-lowMel)*float64(i)/float64(len(melPoints)-1)
		hzPoints[i] = slaneyMelToHz(melPoints[i])
	}
	filters := make([][]float64, melBands)
	for m := 0; m < melBands; m++ {
		filters[m] = make([]float64, half)
		left := hzPoints[m]
		center := hzPoints[m+1]
		right := hzPoints[m+2]
		if !(left < center && center < right) {
			continue
		}
		// Essentia weighting=linear: interpolate triangle weights in Hz.
		// normalize=unit_tri: normalize continuous triangle area by bandwidth.
		normalize := 2.0 / (right - left)
		for k := 0; k < half; k++ {
			hz := float64(k) * float64(sampleRate) / float64(fftSize)
			var weight float64
			switch {
			case hz >= left && hz < center:
				weight = (hz - left) / (center - left)
			case hz >= center && hz <= right:
				weight = (right - hz) / (right - center)
			}
			if weight > 0 {
				filters[m][k] = weight * normalize
			}
		}
	}
	return filters
}

func hzToSlaneyMel(hz float64) float64 {
	const (
		linearHzPerMel = 200.0 / 3.0
		minLogHz       = 1000.0
		minLogMel      = minLogHz / linearHzPerMel
		logStep        = 0.06875177742094912 // ln(6.4) / 27
	)
	if hz < minLogHz {
		return hz / linearHzPerMel
	}
	return minLogMel + math.Log(hz/minLogHz)/logStep
}

func slaneyMelToHz(mel float64) float64 {
	const (
		linearHzPerMel = 200.0 / 3.0
		minLogHz       = 1000.0
		minLogMel      = minLogHz / linearHzPerMel
		logStep        = 0.06875177742094912 // ln(6.4) / 27
	)
	if mel < minLogMel {
		return mel * linearHzPerMel
	}
	return minLogHz * math.Exp(logStep*(mel-minLogMel))
}
