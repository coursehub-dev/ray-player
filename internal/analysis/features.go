package analysis

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/madelynnblue/go-dsp/fft"
)

type Features struct {
	Tempo             float64
	BPMPerceived      float64
	TempoConfidence   float64
	TempoStability    float64
	BPMHalf           float64
	BPMDouble         float64
	TempoSource       string
	TempoModel        string
	TempoError        string
	TempoAnalyzedAt   int64
	Energy            float64
	Danceability      float64
	Valence           float64
	Acousticness      float64
	Instrumentalness  float64
	Loudness          float64
	SpectralCentroid  float64
	ZeroCrossingRate  float64
	RMS               float64
	SpectralFlatness  float64
	SpectralRolloff85 float64
	SpectralFlux      float64
	OnsetRate         float64
	DynamicRange      float64
	LowBandRatio      float64
	MidBandRatio      float64
	HighBandRatio     float64
	MFCC              []float64
	Embedding         []float32
}

type AudioFeatureOptions struct {
	MaxAnalysisSec     float64
	PreferCenterWindow bool
	ForProbeShort      bool
}

func Extract(path string) (Features, int, error) {
	return ExtractWithContext(context.Background(), path)
}

func ExtractWithContext(ctx context.Context, path string) (Features, int, error) {
	return ExtractWithOptions(ctx, path, AudioFeatureOptions{})
}

func ExtractWithOptions(ctx context.Context, path string, opts AudioFeatureOptions) (Features, int, error) {
	cfg := DefaultFFmpegConfig()
	samples, sampleRate, warnings, err := DecodeMonoFloat32WithFFmpeg(ctx, path, cfg)
	if warnings != "" {
		analysisLog.W("ffmpeg decode warnings path=%q warnings=%s", path, warnings)
	}
	if err != nil {
		return Features{}, 0, err
	}
	if len(samples) == 0 {
		return Features{}, 0, errors.New("empty audio stream")
	}
	mono := make([]float64, len(samples))
	for i, sample := range samples {
		mono[i] = float64(sample)
	}
	mono = limitAnalysisWindow(mono, sampleRate, opts)
	features := ExtractFromPCMWithOptions(mono, sampleRate, opts)
	durMs, durErr := DecodeAudioDuration(ctx, path)
	if durErr != nil {
		durMs = EstimateAudioDuration(path, len(samples), beep.SampleRate(sampleRate))
	}
	return features, durMs, nil
}

func ExtractFromPCM(samples []float64, sampleRate int) Features {
	return ExtractFromPCMWithOptions(samples, sampleRate, AudioFeatureOptions{})
}

func ExtractFromPCMWithOptions(samples []float64, sampleRate int, opts AudioFeatureOptions) Features {
	if len(samples) == 0 {
		return Features{}
	}
	_ = opts
	return analyzeMono(samples, beep.SampleRate(sampleRate))
}

func limitAnalysisWindow(samples []float64, sampleRate int, opts AudioFeatureOptions) []float64 {
	limitSec := opts.MaxAnalysisSec
	if limitSec <= 0 || sampleRate <= 0 {
		return samples
	}
	limit := int(limitSec * float64(sampleRate))
	if limit <= 0 || len(samples) <= limit {
		return samples
	}
	if !opts.PreferCenterWindow {
		return append([]float64{}, samples[:limit]...)
	}
	start := (len(samples) - limit) / 2
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(samples) {
		end = len(samples)
		start = end - limit
	}
	return append([]float64{}, samples[start:end]...)
}

func EstimateAudioDuration(path string, streamerLen int, sampleRate beep.SampleRate) int {
	if streamerLen > 0 && sampleRate > 0 {
		return int((time.Second * time.Duration(streamerLen) / time.Duration(sampleRate)) / time.Millisecond)
	}
	info, err := os.Stat(path)
	if err != nil {
		return int((4 * time.Minute) / time.Millisecond)
	}
	size := info.Size()
	ext := strings.ToLower(filepath.Ext(path))
	bytesPerSecond := 24000.0
	switch ext {
	case ".flac":
		bytesPerSecond = 100000
	case ".wav":
		bytesPerSecond = 176400
	case ".ogg", ".oga":
		bytesPerSecond = 20000
	case ".m4a", ".aac":
		bytesPerSecond = 24000
	}
	seconds := float64(size) / bytesPerSecond
	if seconds < 10 {
		seconds = 240
	}
	return int(seconds * 1000)
}

func analyzeMono(samples []float64, sr beep.SampleRate) Features {
	r := rms(samples)
	zcr := zeroCross(samples)
	centroid, power := spectralCentroidAndPower(samples, int(sr))
	tempo := estimateTempo(samples, int(sr))
	mfcc := computeMFCC(samples, int(sr), 6)
	energy := clamp(r*3.2, 0, 1)
	dance := clamp((tempo-70)/90*0.65+zcr*4.0, 0, 1)
	valence := clamp((centroid/4000)*0.55+energy*0.3, 0, 1)
	acoustic := clamp(1-energy*0.7-zcr*3, 0, 1)
	instr := clamp(0.4+acoustic*0.4-r*0.2, 0, 1)
	loudness := -60 + r*60
	frames := spectrumFrames(samples, int(sr))
	flatness := spectralFlatnessFromFrames(frames)
	rolloff85 := spectralRolloff(power, int(sr), 0.85)
	onset := onsetRateFromSamples(samples, int(sr))
	dyn := dynamicRangeFromFrames(rmsFrames(samples, int(sr)))
	low, mid, high := bandEnergyRatios(power, int(sr))
	spectralFlux := spectralFluxFromFrames(frames)
	emb := []float32{float32(tempo / 200), float32(energy), float32(dance), float32(valence), float32(acoustic), float32(instr), float32(clamp((loudness+60)/60, 0, 1)), float32(clamp(centroid/8000, 0, 1)), float32(clamp(zcr*8, 0, 1)), float32(r)}
	for _, coeff := range mfcc {
		emb = append(emb, float32(clamp((coeff+20)/40, 0, 1)))
	}
	return Features{Tempo: tempo, BPMPerceived: NormalizePerceivedBPM(tempo), BPMHalf: tempo * 0.5, BPMDouble: tempo * 2, Energy: energy, Danceability: dance, Valence: valence, Acousticness: acoustic, Instrumentalness: instr, Loudness: loudness, SpectralCentroid: centroid, ZeroCrossingRate: zcr, RMS: r, SpectralFlatness: flatness, SpectralRolloff85: rolloff85, SpectralFlux: spectralFlux, OnsetRate: onset, DynamicRange: dyn, LowBandRatio: low, MidBandRatio: mid, HighBandRatio: high, MFCC: mfcc, Embedding: emb}
}

func rms(samples []float64) float64 {
	var sum float64
	for _, s := range samples {
		sum += s * s
	}
	return math.Sqrt(sum / math.Max(1, float64(len(samples))))
}
func zeroCross(samples []float64) float64 {
	if len(samples) < 2 {
		return 0
	}
	n := 0
	for i := 1; i < len(samples); i++ {
		if (samples[i] >= 0) != (samples[i-1] >= 0) {
			n++
		}
	}
	return float64(n) / float64(len(samples))
}

func spectralCentroidAndPower(samples []float64, sr int) (float64, []float64) {
	n := 2048
	if len(samples) < n {
		n = len(samples)
	}
	frame := make([]float64, n)
	copy(frame, samples[:n])
	for i := range frame {
		frame[i] *= 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n))
	}
	spec := fft.FFTReal(frame)
	power := make([]float64, len(spec)/2)
	var weighted, total float64
	for i := 0; i < len(power); i++ {
		mag := cmplxAbs(spec[i])
		p := mag * mag
		power[i] = p
		freq := float64(i) * float64(sr) / float64(n)
		weighted += mag * freq
		total += mag
	}
	if total == 0 {
		return 0, power
	}
	return weighted / total, power
}

func spectrumFrames(samples []float64, sr int) [][]float64 {
	window := sr / 20
	if window < 512 {
		window = 512
	}
	if len(samples) < window {
		return nil
	}
	step := window / 2
	if step < 1 {
		step = 1
	}
	frames := make([][]float64, 0, len(samples)/step)
	for i := 0; i+window <= len(samples); i += step {
		frame := make([]float64, window)
		copy(frame, samples[i:i+window])
		for j := range frame {
			frame[j] *= 0.5 - 0.5*math.Cos(2*math.Pi*float64(j)/float64(window))
		}
		frames = append(frames, frame)
	}
	return frames
}

func spectrumPower(frame []float64) []float64 {
	spec := fft.FFTReal(frame)
	power := make([]float64, len(spec)/2)
	for i := 0; i < len(power); i++ {
		mag := cmplxAbs(spec[i])
		power[i] = mag * mag
	}
	return power
}

func normalizeSpectrum(x []float64) []float64 {
	out := make([]float64, len(x))
	sum := 0.0
	for _, v := range x {
		if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			sum += v
		}
	}
	if sum <= 1e-12 {
		return out
	}
	for i, v := range x {
		if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			out[i] = v / sum
		}
	}
	return out
}

func trimmedMeanSorted(xs []float64, trim float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	start := int(float64(len(xs)) * trim)
	end := len(xs) - start
	if start >= end {
		start = 0
		end = len(xs)
	}
	sum := 0.0
	for _, x := range xs[start:end] {
		sum += x
	}
	return sum / float64(end-start)
}

func spectralFluxFromFrames(frames [][]float64) float64 {
	if len(frames) < 2 {
		return 0
	}
	fluxes := make([]float64, 0, len(frames)-1)
	for i := 1; i < len(frames); i++ {
		prev := normalizeSpectrum(spectrumPower(frames[i-1]))
		cur := normalizeSpectrum(spectrumPower(frames[i]))
		n := len(prev)
		if len(cur) < n {
			n = len(cur)
		}
		if n == 0 {
			continue
		}
		sum := 0.0
		for j := 0; j < n; j++ {
			d := cur[j] - prev[j]
			if d > 0 {
				sum += d
			}
		}
		fluxes = append(fluxes, clamp01(sum))
	}
	if len(fluxes) == 0 {
		return 0
	}
	sort.Float64s(fluxes)
	return trimmedMeanSorted(fluxes, 0.10)
}

func spectralFlatnessFromFrames(frames [][]float64) float64 {
	if len(frames) == 0 {
		return 0
	}
	vals := make([]float64, 0, len(frames))
	for _, frame := range frames {
		power := spectrumPower(frame)
		if len(power) == 0 {
			continue
		}
		const eps = 1e-12
		logSum, arith := 0.0, 0.0
		count := 0
		for _, p := range power {
			if p <= 0 || math.IsNaN(p) || math.IsInf(p, 0) {
				continue
			}
			x := p + eps
			logSum += math.Log(x)
			arith += x
			count++
		}
		if count == 0 || arith <= 0 {
			continue
		}
		geo := math.Exp(logSum / float64(count))
		ari := arith / float64(count)
		vals = append(vals, clamp01(geo/ari))
	}
	if len(vals) == 0 {
		return 0
	}
	sort.Float64s(vals)
	return trimmedMeanSorted(vals, 0.10)
}

func bandEnergyRatios(power []float64, sampleRate int) (low, mid, high float64) {
	if len(power) == 0 || sampleRate <= 0 {
		return 0, 0, 0
	}
	lowSum := 0.0
	midSum := 0.0
	highSum := 0.0
	n := len(power)
	nyquist := float64(sampleRate) / 2.0
	for i, p := range power {
		if p <= 0 || math.IsNaN(p) || math.IsInf(p, 0) {
			continue
		}
		freq := float64(i) / float64(maxInt(1, n-1)) * nyquist
		switch {
		case freq >= 20 && freq < 250:
			lowSum += p
		case freq >= 250 && freq < 4000:
			midSum += p
		case freq >= 4000 && freq <= nyquist:
			highSum += p
		}
	}
	sum := lowSum + midSum + highSum
	if sum <= 1e-12 {
		return 0, 0, 0
	}
	return lowSum / sum, midSum / sum, highSum / sum
}

func spectralRolloff(power []float64, sampleRate int, ratio float64) float64 {
	if len(power) == 0 || sampleRate <= 0 {
		return 0
	}
	total := 0.0
	for _, p := range power {
		if p > 0 && !math.IsNaN(p) && !math.IsInf(p, 0) {
			total += p
		}
	}
	if total <= 1e-12 {
		return 0
	}
	target := total * ratio
	acc := 0.0
	n := len(power)
	nyquist := float64(sampleRate) / 2.0
	for i, p := range power {
		if p > 0 && !math.IsNaN(p) && !math.IsInf(p, 0) {
			acc += p
		}
		if acc >= target {
			return float64(i) / float64(maxInt(1, n-1)) * nyquist
		}
	}
	return nyquist
}

func rmsFrames(samples []float64, sr int) []float64 {
	window := sr / 20
	if window < 256 {
		window = 256
	}
	out := make([]float64, 0, len(samples)/window)
	for i := 0; i+window <= len(samples); i += window {
		out = append(out, rms(samples[i:i+window]))
	}
	return out
}

func onsetRateFromSamples(samples []float64, sr int) float64 {
	energy := rmsFrames(samples, sr)
	if len(energy) < 3 || sr <= 0 {
		return 0
	}
	dur := float64(len(samples)) / float64(sr)
	if dur <= 0 {
		return 0
	}
	mean := 0.0
	for _, e := range energy {
		mean += e
	}
	mean /= float64(len(energy))
	variance := 0.0
	for _, e := range energy {
		d := e - mean
		variance += d * d
	}
	std := math.Sqrt(variance / float64(len(energy)))
	threshold := mean + 0.75*std
	onsets := 0
	prevRising := false
	for i := 1; i < len(energy); i++ {
		rise := energy[i]-energy[i-1] > threshold*0.15 && energy[i] > threshold
		if rise && !prevRising {
			onsets++
		}
		prevRising = rise
	}
	return float64(onsets) / dur
}

func dynamicRangeFromFrames(rmsFrames []float64) float64 {
	if len(rmsFrames) == 0 {
		return 0
	}
	xs := append([]float64{}, rmsFrames...)
	sort.Float64s(xs)
	p10 := percentileSorted(xs, 0.10)
	p95 := percentileSorted(xs, 0.95)
	if p95 <= 1e-9 {
		return 0
	}
	return clamp01((p95 - p10) / p95)
}

func estimateTempo(samples []float64, sr int) float64 {
	if len(samples) < sr {
		return 90
	}
	window := sr / 20
	if window < 256 {
		window = 256
	}
	env := make([]float64, 0, len(samples)/window)
	for i := 0; i+window <= len(samples); i += window {
		env = append(env, rms(samples[i:i+window]))
	}
	bestLag, best := 0, 0.0
	minLag := maxInt(1, int(math.Round(float64(60*20)/180.0)))
	maxLag := maxInt(minLag+1, int(math.Round(float64(60*20)/70.0)))
	for lag := minLag; lag <= maxLag && lag < len(env)/2; lag++ {
		var score float64
		for i := lag; i < len(env); i++ {
			score += env[i] * env[i-lag]
		}
		if score > best {
			best, bestLag = score, lag
		}
	}
	if bestLag == 0 {
		return 90
	}
	return 60 * 20 / float64(bestLag)
}
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func cmplxAbs(v complex128) float64 { return math.Hypot(real(v), imag(v)) }
func computeMFCC(samples []float64, sr int, count int) []float64 {
	n := 2048
	if len(samples) < n {
		n = len(samples)
	}
	if n <= 0 || count <= 0 {
		return nil
	}
	frame := make([]float64, n)
	copy(frame, samples[:n])
	for i := range frame {
		frame[i] *= 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(maxInt(2, n-1)))
	}
	spec := fft.FFTReal(frame)
	mags := make([]float64, n/2)
	for i := range mags {
		mags[i] = math.Log1p(cmplxAbs(spec[i]))
	}
	out := make([]float64, count)
	for k := 0; k < count; k++ {
		var sum float64
		for i, v := range mags {
			sum += v * math.Cos(math.Pi*float64(k)*(float64(i)+0.5)/float64(len(mags)))
		}
		out[k] = sum / float64(len(mags))
	}
	_ = sr
	return out
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp01(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func percentileSorted(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	if p <= 0 {
		return xs[0]
	}
	if p >= 1 {
		return xs[len(xs)-1]
	}
	pos := p * float64(len(xs)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return xs[lo]
	}
	f := pos - float64(lo)
	return xs[lo]*(1-f) + xs[hi]*f
}
