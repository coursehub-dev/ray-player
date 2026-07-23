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
// This path must remain compatible with the official Essentia/Python preprocessing
// verified experimentally for discogs-effnet-bs64-1 and classifier heads.
// Do not add log1p/db/unit normalization here.
// Experimental transforms belong only to ExtractMelSpectrogramMode and cmd/model_probe.
func ExtractMelSpectrogram(path string) ([]float32, int, error) {
	return ExtractMelSpectrogramMode(path, string(MelModeOfficial))
}

func ExtractMelSpectrogramMode(path, mode string) ([]float32, int, error) {
	frames, err := ExtractMelFramesMode(path, mode)
	if err != nil {
		return nil, 0, err
	}
	frameCount := len(frames) / EssentiaMelBands
	if frameCount < EssentiaPatchFrames {
		return nil, 0, errors.New("too few mel frames")
	}
	patchHop := 62
	patches := 1 + (frameCount-EssentiaPatchFrames)/patchHop
	if patches <= 0 {
		return nil, 0, errors.New("too few mel frames")
	}
	flat := make([]float32, 0, patches*EssentiaMelBands*EssentiaPatchFrames)
	for p := 0; p < patches; p++ {
		startFrame := p * patchHop
		endFrame := startFrame + EssentiaPatchFrames
		if endFrame > frameCount {
			break
		}
		chunk := frames[startFrame*EssentiaMelBands : endFrame*EssentiaMelBands]
		for frame := 0; frame < EssentiaPatchFrames; frame++ {
			for mel := 0; mel < EssentiaMelBands; mel++ {
				flat = append(flat, chunk[frame*EssentiaMelBands+mel])
			}
		}
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

func transformMelFrame(frame []float32, mode string) []float32 {
	out := make([]float32, len(frame))
	switch mode {
	case string(MelModeOfficial):
		copy(out, frame)
	case string(MelModeCurrent):
		copy(out, frame)
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
	cfg := DefaultFFmpegConfig()
	cfg.Start = 0
	cfg.Duration = dur
	raw, sr, _, err := DecodeMonoFloat32WithFFmpeg(context.Background(), path, cfg)
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
		out[i] = make([]float32, len(energy[i]))
		for j, v := range energy[i] {
			out[i][j] = float32(math.Log1p(float64(v)))
		}
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
		out[i] = math.Sqrt(re*re+im*im) + 1e-9
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
			energy += spec[i] * filters[m][i]
		}
		out[m] = float32(energy)
	}
	return out
}

func buildMelFilterbank(melBands, fftSize, sampleRate int) [][]float64 {
	half := fftSize/2 + 1
	lowMel := hzToMel(0)
	highMel := hzToMel(float64(sampleRate) / 2)
	melPoints := make([]float64, melBands+2)
	for i := range melPoints {
		melPoints[i] = lowMel + (highMel-lowMel)*float64(i)/float64(len(melPoints)-1)
	}
	bins := make([]int, len(melPoints))
	for i, mel := range melPoints {
		hz := melToHz(mel)
		bins[i] = int(math.Floor((float64(fftSize) + 1) * hz / float64(sampleRate)))
		if bins[i] > half-1 {
			bins[i] = half - 1
		}
	}
	filters := make([][]float64, melBands)
	for m := 1; m <= melBands; m++ {
		filters[m-1] = make([]float64, half)
		left, center, right := bins[m-1], bins[m], bins[m+1]
		if center <= left {
			center = left + 1
		}
		if right <= center {
			right = center + 1
		}
		for k := left; k < center && k < half; k++ {
			filters[m-1][k] = float64(k-left) / float64(center-left)
		}
		for k := center; k < right && k < half; k++ {
			filters[m-1][k] = float64(right-k) / float64(right-center)
		}
	}
	return filters
}

func hzToMel(hz float64) float64  { return 2595 * math.Log10(1+hz/700) }
func melToHz(mel float64) float64 { return 700 * (math.Pow(10, mel/2595) - 1) }
