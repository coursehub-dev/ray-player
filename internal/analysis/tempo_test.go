package analysis

import (
	"math"
	"testing"
)

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

func TestTempoCNNContractMatchesEssentiaDefaults(t *testing.T) {
	if TempoSampleRate != 11025 || TempoFFTSize != 1024 || TempoHopSize != 512 {
		t.Fatalf("unexpected frame contract sr=%d fft=%d hop=%d", TempoSampleRate, TempoFFTSize, TempoHopSize)
	}
	if TempoMelBands != 40 || TempoPatchFrames != 256 || TempoPatchHop != 128 {
		t.Fatalf("unexpected patch contract bands=%d frames=%d hop=%d", TempoMelBands, TempoPatchFrames, TempoPatchHop)
	}
}

func TestStandardizeTempoPatchesMatchesPerInferenceTensorScaling(t *testing.T) {
	patches := [][]float32{{1, 2, 3, 4}, {10, 20, 30, 40}}
	standardizeTempoPatches(patches)
	for index, patch := range patches {
		mean := 0.0
		for _, value := range patch {
			mean += float64(value)
		}
		mean /= float64(len(patch))
		variance := 0.0
		for _, value := range patch {
			d := float64(value) - mean
			variance += d * d
		}
		std := math.Sqrt(variance / float64(len(patch)))
		if math.Abs(mean) > 1e-6 || math.Abs(std-1) > 1e-6 {
			t.Fatalf("patch=%d standardized mean=%v std=%v values=%v", index, mean, std, patch)
		}
	}
}
