package analysis

import "testing"

func TestNormalizePerceivedBPM(t *testing.T) {
	cases := map[float64]float64{64: 128, 190: 95, 140: 140, 0: 0}
	for in, want := range cases {
		if got := NormalizePerceivedBPM(in); got != want {
			t.Fatalf("NormalizePerceivedBPM(%v)=%v want %v", in, got, want)
		}
	}
}

func TestTempoDistanceNormalizedHandlesHalfDouble(t *testing.T) {
	if TempoDistanceNormalized(80, 160) >= TempoDistanceNormalized(80, 121) {
		t.Fatal("expected half/double tempo to be closer")
	}
}

func TestTempoConfidenceMedianAndStability(t *testing.T) {
	if got := TempoConfidenceMedian([]float64{0.2, 0.8, 0.7}); got != 0.7 {
		t.Fatalf("unexpected median %f", got)
	}
	if got := TempoStability([]float64{128, 129, 64, 128}, 128); got < 0.75 {
		t.Fatalf("expected stability >= 0.75, got %f", got)
	}
}
