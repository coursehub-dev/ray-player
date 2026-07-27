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

func TestExtractTempoFramesUsesMagnitudeMelBands(t *testing.T) {
	samples := make([]float64, TempoFFTSize)
	samples[0] = 2
	frames := extractTempoFrames(samples)
	if len(frames) != 1 || len(frames[0]) != TempoMelBands {
		t.Fatalf("unexpected tempo frames shape=%dx%d", len(frames), func() int {
			if len(frames) == 0 {
				return 0
			}
			return len(frames[0])
		}())
	}
	for i, value := range frames[0] {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || value < 0 {
			t.Fatalf("band=%d invalid magnitude value=%v", i, value)
		}
	}
}

func TestStandardizeTempoPatchesMatchesEssentiaAxisZero(t *testing.T) {
	patches := [][]float32{
		{1, 10, 5},
		{2, 20, 5},
		{3, 30, 5},
	}
	standardizeTempoPatches(patches)

	want := math.Sqrt(1.5)
	if math.Abs(float64(patches[0][0])+want) > 1e-6 ||
		math.Abs(float64(patches[1][0])) > 1e-6 ||
		math.Abs(float64(patches[2][0])-want) > 1e-6 {
		t.Fatalf("axis-0 normalization mismatch for coordinate 0: %#v", patches)
	}
	if math.Abs(float64(patches[0][1])+want) > 1e-6 ||
		math.Abs(float64(patches[1][1])) > 1e-6 ||
		math.Abs(float64(patches[2][1])-want) > 1e-6 {
		t.Fatalf("axis-0 normalization mismatch for coordinate 1: %#v", patches)
	}
	for row := range patches {
		if patches[row][2] != 0 {
			t.Fatalf("constant batch coordinate must normalize to zero: %#v", patches)
		}
	}
}
