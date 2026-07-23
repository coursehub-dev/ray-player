package emotion

import "testing"

func TestComputeFromInputsStableLabels(t *testing.T) {
	abba := Inputs{
		Dance:             0.99,
		Valence:           0.72,
		Relaxed:           0.66,
		Party:             0.55,
		BPM:               115,
		BPMPerceived:      115,
		TempoConfidence:   1,
		TempoStability:    1,
		Loudness:          -42.6,
		RMS:               0.289,
		ZeroCrossingRate:  0.194,
		SpectralCentroid:  2634,
		SpectralFlatness:  0.060,
		SpectralRolloff85: 3175,
		SpectralFlux:      0.44,
		OnsetRate:         1.78,
		DynamicRange:      0.62,
		MidBandRatio:      0.87,
		HighBandRatio:     0.155,
	}
	if got := ComputeFromInputs(abba, nil).Basis.Label; got != "joy_party" && got != "uplift_drive" && got != "joy_funk" {
		t.Fatalf("ABBA-like should resolve to a clean upbeat label, got %s", got)
	}

	adele := Inputs{
		Dance:             1,
		Valence:           0.47,
		Vocalness:         0.77,
		Relaxed:           0.49,
		BPM:               134,
		BPMPerceived:      134,
		TempoConfidence:   0.65,
		TempoStability:    0.53,
		Loudness:          -44,
		RMS:               0.268,
		ZeroCrossingRate:  0.09,
		SpectralCentroid:  1547,
		SpectralFlatness:  0.003,
		SpectralRolloff85: 751,
		SpectralFlux:      0.272,
		OnsetRate:         1.13,
		DynamicRange:      0.743,
	}
	if got := ComputeFromInputs(adele, nil).Basis.Label; got != "melancholy_grief" {
		t.Fatalf("Adele-like should be melancholy_grief, got %s", got)
	}
}

func TestTopLabelScoresUsesDebugGuards(t *testing.T) {
	b := Basis{Joy: 0.8, Swagger: 0.8, Serenity: 0.1, Combat: 0.1, Pressure: 0.1, Roughness: 0.1, Brightness: 0.7, Smoothness: 0.6, Sprint: 0.4, Pulse: 0.5, Motion: 0.4, Melancholy: 0.1, Impact: 0.2, Dreaminess: 0.1}
	d := Debug{CleanBright: 0.2, JoyConfidence: 0.2, WarmGroove: 0.2, SereneBright: 0.2, TensePressure: 0.2, VocalGrief: 0.2, DramaticArc: 0.2}
	for _, it := range TopLabelScoresWithDebug(b, d) {
		if it.Label == "joy_party" && it.Passed {
			t.Fatalf("joy_party should not pass without JoyConfidence")
		}
		if it.Label == "combat_force" && it.Passed {
			t.Fatalf("combat_force should not pass without debug guard")
		}
	}
}
