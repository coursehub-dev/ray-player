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
