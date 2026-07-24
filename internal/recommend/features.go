package recommend

import (
	"math"
	"sort"

	"ray-player1/internal/emotion"
	"ray-player1/internal/library"
)

type FeatureStats struct {
	Name   string
	Count  int
	P05    float64
	P50    float64
	P95    float64
	Min    float64
	Max    float64
	Mean   float64
	Std    float64
	Spread float64

	NearZeroRatio float64
	NearOneRatio  float64
	BinaryRatio   float64

	Reliability float64
	Invalid     bool
	Reason      string
}

type FeatureNormalizer struct {
	Stats map[string]FeatureStats
}

func BuildFeatureNormalizer(tracks []library.Track) FeatureNormalizer {
	names := []string{
		"Energy", "Danceability", "Valence", "Happy", "Sad", "Relaxed",
		"Party", "Aggressive", "Acousticness", "Electronicness",
		"Instrumentalness", "Vocalness", "Melodicness", "Softness",
		"Heaviness", "Dreaminess", "Emotionality", "TimbreBrightness",
		"Tonality", "Approachability", "Engagement",
		"TempoConfidence", "TempoStability",
		"Loudness", "RMS", "ZeroCrossingRate", "SpectralCentroid",
		"SpectralFlatness", "SpectralRolloff85", "SpectralFlux",
		"OnsetRate", "DynamicRange", "LowBandRatio", "MidBandRatio", "HighBandRatio",
	}

	out := FeatureNormalizer{Stats: map[string]FeatureStats{}}
	for _, name := range names {
		values := make([]float64, 0, len(tracks))
		for _, t := range tracks {
			v, ok := featureValue(t, name)
			if ok && isUsableFeature(name, v) {
				values = append(values, v)
			}
		}
		if len(values) == 0 {
			continue
		}
		out.Stats[name] = buildFeatureStats(name, values)
	}
	return out
}

func (n FeatureNormalizer) Norm(name string, value float64) float64 {
	if _, ok := emotion.SemanticFeatureTrust(name); ok {
		// Classifier probabilities and bounded regression outputs already have
		// global meaning. Percentile-remapping them against the current library
		// makes the same track change emotion when unrelated tracks are added.
		return clamp01(value)
	}
	st, ok := n.Stats[name]
	if !ok {
		return fallbackFeatureNorm(name, value)
	}
	if st.Invalid || st.Reliability <= 0 || st.P95-st.P05 < minUsefulSpread(name) {
		return fallbackFeatureNorm(name, value)
	}
	return clamp01((value - st.P05) / (st.P95 - st.P05))
}

func (n FeatureNormalizer) Reliability(name string) float64 {
	if trust, ok := emotion.SemanticFeatureTrust(name); ok {
		return trust
	}
	st, ok := n.Stats[name]
	if !ok {
		return 0.35
	}
	if st.Invalid {
		return 0
	}
	return clamp01(st.Reliability)
}

func (n FeatureNormalizer) NormWeighted(name string, value float64) float64 {
	if normalized, ok := emotion.NormalizeSemanticFeature(name, value); ok {
		return normalized
	}
	norm := n.Norm(name, value)
	rel := n.Reliability(name)
	if rel <= 0 && isRawAudioFeature(name) {
		return fallbackFeatureNorm(name, value)
	}
	return 0.5 + (norm-0.5)*rel
}

func normalizeTrackFeatures(t library.Track, n FeatureNormalizer) library.Track {
	t.Energy = n.NormWeighted("Energy", t.Energy)
	t.Danceability = n.NormWeighted("Danceability", t.Danceability)
	t.Valence = n.NormWeighted("Valence", t.Valence)
	t.Happy = n.NormWeighted("Happy", t.Happy)
	t.Sad = n.NormWeighted("Sad", t.Sad)
	t.Relaxed = n.NormWeighted("Relaxed", t.Relaxed)
	t.Party = n.NormWeighted("Party", t.Party)
	t.Aggressive = n.NormWeighted("Aggressive", t.Aggressive)
	t.Acousticness = n.NormWeighted("Acousticness", t.Acousticness)
	t.Electronicness = n.NormWeighted("Electronicness", t.Electronicness)
	t.Instrumentalness = n.NormWeighted("Instrumentalness", t.Instrumentalness)
	t.Vocalness = n.NormWeighted("Vocalness", t.Vocalness)
	t.Melodicness = n.NormWeighted("Melodicness", t.Melodicness)
	t.Softness = n.NormWeighted("Softness", t.Softness)
	t.Heaviness = n.NormWeighted("Heaviness", t.Heaviness)
	t.Dreaminess = n.NormWeighted("Dreaminess", t.Dreaminess)
	t.Emotionality = n.NormWeighted("Emotionality", t.Emotionality)
	t.TimbreBrightness = n.NormWeighted("TimbreBrightness", t.TimbreBrightness)
	t.Tonality = n.NormWeighted("Tonality", t.Tonality)
	t.Approachability = n.NormWeighted("Approachability", t.Approachability)
	t.Engagement = n.NormWeighted("Engagement", t.Engagement)
	return t
}

func featureValue(t library.Track, name string) (float64, bool) {
	switch name {
	case "Energy":
		return t.Energy, true
	case "Danceability":
		return t.Danceability, true
	case "Valence":
		return t.Valence, true
	case "Happy":
		return t.Happy, true
	case "Sad":
		return t.Sad, true
	case "Relaxed":
		return t.Relaxed, true
	case "Party":
		return t.Party, true
	case "Aggressive":
		return t.Aggressive, true
	case "Acousticness":
		return t.Acousticness, true
	case "Electronicness":
		return t.Electronicness, true
	case "Instrumentalness":
		return t.Instrumentalness, true
	case "Vocalness":
		return t.Vocalness, true
	case "Melodicness":
		return t.Melodicness, true
	case "Softness":
		return t.Softness, true
	case "Heaviness":
		return t.Heaviness, true
	case "Dreaminess":
		return t.Dreaminess, true
	case "Emotionality":
		return t.Emotionality, true
	case "TimbreBrightness":
		return t.TimbreBrightness, true
	case "Tonality":
		return t.Tonality, true
	case "Approachability":
		return t.Approachability, true
	case "Engagement":
		return t.Engagement, true
	case "TempoConfidence":
		return t.TempoConfidence, true
	case "TempoStability":
		return t.TempoStability, true
	case "Loudness":
		return t.Loudness, true
	case "RMS":
		return t.RMS, true
	case "ZeroCrossingRate":
		return t.ZeroCrossingRate, true
	case "SpectralCentroid":
		return t.SpectralCentroid, true
	case "SpectralFlatness":
		return t.SpectralFlatness, true
	case "SpectralRolloff85":
		return t.SpectralRolloff85, true
	case "SpectralFlux":
		return t.SpectralFlux, true
	case "OnsetRate":
		return t.OnsetRate, true
	case "DynamicRange":
		return t.DynamicRange, true
	case "LowBandRatio":
		return t.LowBandRatio, true
	case "MidBandRatio":
		return t.MidBandRatio, true
	case "HighBandRatio":
		return t.HighBandRatio, true
	default:
		return 0, false
	}
}

// Important:
// Do NOT mark a single track value 0.0 or 1.0 as invalid.
// Some classifiers can be legitimately confident.
// We only reduce feature weight when the whole feature distribution
// is saturated or has too little spread across the current library.
func featureReliability(name string, xs []float64, p05, p50, p95 float64) (float64, bool, string) {
	if len(xs) < 10 {
		return 0.35, false, "small_sample"
	}

	spread := p95 - p05
	nearZero := 0
	nearOne := 0
	binary := 0

	if isRawAudioFeature(name) {
		minSpread := minUsefulSpread(name)
		if spread < minSpread {
			return 0, true, "no_spread"
		}
		reliability := clamp01(spread / usefulSpreadScale(name))
		if reliability < 0.10 {
			return 0, true, "low_reliability"
		}
		return reliability, false, ""
	}
	for _, x := range xs {
		if x <= 0.001 {
			nearZero++
		}
		if x >= 0.999 {
			nearOne++
		}
		if x <= 0.001 || x >= 0.999 {
			binary++
		}
	}
	n := float64(len(xs))
	nearZeroRatio := float64(nearZero) / n
	nearOneRatio := float64(nearOne) / n
	binaryRatio := float64(binary) / n

	if spread < 0.03 {
		return 0, true, "no_spread"
	}
	if nearZeroRatio > 0.85 {
		return 0, true, "near_zero_saturated"
	}
	if nearOneRatio > 0.85 {
		return 0, true, "near_one_saturated"
	}

	reliability := clamp01(spread / 0.35)
	if binaryRatio > 0.75 {
		reliability *= 0.45
	}
	if binaryRatio > 0.50 {
		reliability *= 0.70
	}
	if p50 < 0.03 || p50 > 0.97 {
		reliability *= 0.55
	}
	if reliability < 0.10 {
		return 0, true, "low_reliability"
	}
	return clamp01(reliability), false, ""
}

func buildFeatureStats(name string, values []float64) FeatureStats {
	sort.Float64s(values)
	mean := meanFloat64(values)
	std := stdFloat64(values, mean)
	p05 := percentile(values, 0.05)
	p50 := percentile(values, 0.50)
	p95 := percentile(values, 0.95)
	reliability, invalid, reason := featureReliability(name, values, p05, p50, p95)
	return FeatureStats{
		Name:          name,
		Count:         len(values),
		P05:           p05,
		P50:           p50,
		P95:           p95,
		Min:           values[0],
		Max:           values[len(values)-1],
		Mean:          mean,
		Std:           std,
		Spread:        p95 - p05,
		NearZeroRatio: ratioNear(values, 0.001, true),
		NearOneRatio:  ratioNear(values, 0.999, false),
		BinaryRatio:   ratioBinary(values),
		Reliability:   reliability,
		Invalid:       invalid,
		Reason:        reason,
	}
}

func meanFloat64(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func stdFloat64(xs []float64, mean float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		d := x - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(xs)))
}

func ratioNear(xs []float64, threshold float64, zero bool) float64 {
	if len(xs) == 0 {
		return 0
	}
	n := 0
	for _, x := range xs {
		if zero {
			if x <= threshold {
				n++
			}
		} else {
			if x >= threshold {
				n++
			}
		}
	}
	return float64(n) / float64(len(xs))
}

func ratioBinary(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	n := 0
	for _, x := range xs {
		if x <= 0.001 || x >= 0.999 {
			n++
		}
	}
	return float64(n) / float64(len(xs))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := clamp01(p) * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

func fallbackFeatureNorm(name string, v float64) float64 {
	switch name {
	case "Loudness":
		return clamp01((v + 60.0) / 35.0)
	case "RMS":
		return clamp01((v - 0.05) / 0.45)
	case "ZeroCrossingRate":
		return clamp01(v / 0.22)
	case "SpectralCentroid":
		return clamp01((v - 200.0) / 5000.0)
	case "SpectralRolloff85":
		return clamp01((v - 100.0) / 6000.0)
	case "SpectralFlatness":
		return clamp01(v / 0.08)
	case "SpectralFlux":
		return clamp01(v / 0.45)
	case "OnsetRate":
		return clamp01(v / 3.0)
	case "DynamicRange", "LowBandRatio", "MidBandRatio", "HighBandRatio":
		return clamp01(v)
	default:
		return clamp01(v)
	}
}

func isRawAudioFeature(name string) bool {
	switch name {
	case "Loudness", "RMS", "ZeroCrossingRate", "SpectralCentroid", "SpectralFlatness", "SpectralRolloff85", "SpectralFlux", "OnsetRate", "DynamicRange", "LowBandRatio", "MidBandRatio", "HighBandRatio":
		return true
	default:
		return false
	}
}

func minUsefulSpread(name string) float64 {
	switch name {
	case "Loudness":
		return 2.0
	case "SpectralCentroid":
		return 180
	case "SpectralRolloff85":
		return 250
	case "RMS":
		return 0.025
	case "ZeroCrossingRate", "SpectralFlatness", "SpectralFlux", "OnsetRate":
		return 0.01
	case "DynamicRange", "LowBandRatio", "MidBandRatio", "HighBandRatio":
		return 0.02
	default:
		return 0.03
	}
}

func usefulSpreadScale(name string) float64 {
	switch name {
	case "Loudness":
		return 14.0
	case "SpectralCentroid":
		return 2800.0
	case "SpectralRolloff85":
		return 4200.0
	case "RMS":
		return 0.35
	case "ZeroCrossingRate":
		return 0.16
	case "SpectralFlatness":
		return 0.08
	case "SpectralFlux":
		return 0.45
	case "OnsetRate":
		return 3.0
	default:
		return 0.35
	}
}

func isUsableFeature(name string, v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	switch name {
	case "Loudness":
		return v >= -90 && v <= 5
	case "RMS":
		return v >= 0 && v <= 2
	case "ZeroCrossingRate":
		return v >= 0 && v <= 1
	case "SpectralCentroid":
		return v >= 20 && v <= 12000
	case "SpectralRolloff85":
		return v >= 50 && v <= 22050
	case "SpectralFlatness":
		return v >= 0 && v <= 1
	case "SpectralFlux":
		return v >= 0 && v <= 10
	case "OnsetRate":
		return v >= 0 && v <= 20
	case "DynamicRange":
		return v >= 0 && v <= 1.5
	case "LowBandRatio", "MidBandRatio", "HighBandRatio":
		return v >= 0 && v <= 1
	default:
		return v >= 0 && v <= 1
	}
}
