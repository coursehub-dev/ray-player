package emotion

import "math"

// NormalizeSemanticFeature applies a model-level trust prior without using
// the current library distribution. The boolean is false for raw audio
// texture features, which need their own physical/range normalization.
func NormalizeSemanticFeature(name string, value float64) (float64, bool) {
	trust, ok := SemanticFeatureTrust(name)
	if !ok {
		return 0, false
	}
	raw := clampSemantic01(value)
	neutral := semanticFeatureNeutral(name)
	return clampSemantic01(neutral + (raw-neutral)*trust), true
}

// SemanticFeatureTrust reports a fixed prior for globally bounded model
// outputs. It is intentionally independent of library composition.
func SemanticFeatureTrust(name string) (float64, bool) {
	switch name {
	case "Melodicness", "Softness", "Heaviness", "Dreaminess", "Emotionality":
		// These axes are projections of a broad 56-label mood/theme head, not
		// dedicated calibrated binary classifiers.
		return 0.45, true
	case "TimbreBrightness", "Tonality":
		return 0.65, true
	case "Approachability", "Engagement":
		return 0.80, true
	case "Energy", "Danceability", "Valence", "Happy", "Sad", "Relaxed",
		"Party", "Aggressive", "Acousticness", "Electronicness",
		"Instrumentalness", "Vocalness":
		return 1.0, true
	default:
		return 0, false
	}
}

func semanticFeatureNeutral(name string) float64 {
	switch name {
	case "Valence", "TimbreBrightness", "Tonality", "Approachability", "Engagement":
		return 0.5
	default:
		// Happy/sad/aggressive/etc. are one-sided evidence. Low trust must
		// reduce evidence toward absence, not invent a neutral 0.5 signal.
		return 0
	}
}

func clampSemantic01(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
