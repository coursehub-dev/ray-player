package emotion

import "testing"

func TestTuningDefaultVersion(t *testing.T) {
	if got := DefaultTuning().Version; got == "" {
		t.Fatal("DefaultTuning.Version must not be empty")
	}
}

func TestComputeFromInputsWithTuningCanChangeJoyPartyGate(t *testing.T) {
	in := Inputs{
		Dance:             0.74,
		Valence:           0.58,
		Happy:             0.48,
		Relaxed:           0.42,
		Party:             0.40,
		BPM:               118,
		BPMPerceived:      118,
		TempoConfidence:   1,
		TempoStability:    1,
		Loudness:          -28,
		RMS:               0.33,
		ZeroCrossingRate:  0.11,
		SpectralCentroid:  1700,
		SpectralFlatness:  0.04,
		SpectralRolloff85: 2800,
		SpectralFlux:      0.12,
		OnsetRate:         1.05,
		DynamicRange:      0.40,
	}

	base := ComputeFromInputs(in, nil).Basis.Label
	if base == "" {
		t.Fatal("base label must not be empty")
	}

	tuned := DefaultTuning()
	tuned.JoyPartyMinClean = 0.95
	tuned.JoyPartyMinJoy = 0.90
	got := ComputeFromInputsWithTuning(in, nil, tuned).Basis.Label
	if got != "neutral" {
		t.Fatalf("expected tuned gate to suppress joy_party, got %s", got)
	}
}
