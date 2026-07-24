package onnx

import (
	"testing"

	"ray-player1/internal/analysis"
)

func TestTempoInputShapeCandidatesAreRank4(t *testing.T) {
	shapes := tempoInputShapeCandidates()
	if len(shapes) < 2 {
		t.Fatalf("expected several tempo shape candidates")
	}

	wantProduct := int64(analysis.TempoPatchFrames * analysis.TempoMelBands)
	for _, shape := range shapes {
		if len(shape) != 4 {
			t.Fatalf("shape rank=%d want 4: %v", len(shape), shape)
		}
		product := int64(1)
		for _, dim := range shape {
			product *= dim
		}
		if product != wantProduct {
			t.Fatalf("shape product=%d want %d shape=%v", product, wantProduct, shape)
		}
	}
}

func TestTransposeTempoPatchKeepsLength(t *testing.T) {
	in := make([]float32, analysis.TempoPatchFrames*analysis.TempoMelBands)
	out := transposeTempoPatch(in)
	if len(out) != len(in) {
		t.Fatalf("transpose length=%d want %d", len(out), len(in))
	}
}

func TestTempoReliableAllowsStableHighConfidenceTrack(t *testing.T) {
	result := TempoResult{
		BPM:              115,
		Confidence:       0.92,
		RawBPMStd:        0,
		RawConfidenceStd: 0,
	}
	if !tempoResultReliable(result, 57) {
		t.Fatal("stable tempo must not be rejected only because raw BPM std is zero")
	}
}

func TestTempoReliableRejectsLowConfidence(t *testing.T) {
	result := TempoResult{
		BPM:        115,
		Confidence: 0.20,
		RawBPMStd:  12,
	}
	if tempoResultReliable(result, 57) {
		t.Fatal("low-confidence tempo must be rejected")
	}
}

func TestTempoLooksLockedRejectsExactHighConfidenceLock(t *testing.T) {
	result := TempoResult{
		BPM:              115,
		Confidence:       1,
		LocalBPM:         []float64{115, 115, 115, 115},
		RawBPMStd:        0,
		RawConfidenceStd: 0,
	}
	if !tempoLooksLocked(result) {
		t.Fatal("expected suspicious lock diagnostic")
	}
	if !tempoResultReliable(result, len(result.LocalBPM)) {
		t.Fatal("base reliability should remain high before the lock guard")
	}
	if reliable := tempoResultReliable(result, len(result.LocalBPM)) && !tempoLooksLocked(result); reliable {
		t.Fatal("exact high-confidence lock must be rejected by the final guard")
	}
}
