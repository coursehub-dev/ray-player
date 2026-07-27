package analysis

import (
	"math"
	"testing"
	"time"
)

func TestDefaultMelModeIsOfficial(t *testing.T) {
	if DefaultMelMode() != MelModeOfficial {
		t.Fatalf("default mel mode = %q, want %q", DefaultMelMode(), MelModeOfficial)
	}
}

func TestApplyMelModeOfficialMatchesMusiCNNLogCompression(t *testing.T) {
	in := []float32{0, 0.0001, 0.001}
	out := transformMelFrame(in, string(MelModeOfficial))
	if len(out) != len(in) {
		t.Fatalf("len mismatch: got %d want %d", len(out), len(in))
	}
	want := []float64{0, math.Log10(2), math.Log10(11)}
	for i := range out {
		if math.Abs(float64(out[i])-want[i]) > 1e-6 {
			t.Fatalf("official[%d]=%v want=%v", i, out[i], want[i])
		}
	}
	out[0] = 999
	if in[0] == 999 {
		t.Fatal("apply must return a copy, not alias input")
	}
}

func TestSlaneyMelRoundTrip(t *testing.T) {
	if got := hzToSlaneyMel(1000); math.Abs(got-15) > 1e-9 {
		t.Fatalf("hzToSlaneyMel(1000)=%v want=15", got)
	}
	for _, hz := range []float64{0, 100, 1000, 4000, 8000} {
		got := slaneyMelToHz(hzToSlaneyMel(hz))
		if math.Abs(got-hz) > 1e-6*math.Max(1, hz) {
			t.Fatalf("Slaney round trip %v -> %v", hz, got)
		}
	}
}

func TestMusiCNNHannWindowIsSymmetricAndNotAreaNormalized(t *testing.T) {
	p := NewMelProcessor()
	if len(p.hannWin) != EssentiaFFTSize {
		t.Fatalf("hann size=%d want=%d", len(p.hannWin), EssentiaFFTSize)
	}
	sum := 0.0
	for i, value := range p.hannWin {
		sum += value
		if math.Abs(value-p.hannWin[len(p.hannWin)-1-i]) > 1e-12 {
			t.Fatalf("hann window is not symmetric at %d", i)
		}
	}
	want := float64(EssentiaFFTSize-1) / 2
	if math.Abs(sum-want) > 1e-9 {
		t.Fatalf("hann sum=%v want=%v; window must match Essentia normalized=false", sum, want)
	}
}

func TestApplyMelFiltersEnergyUsesPowerSpectrum(t *testing.T) {
	got := applyMelFiltersEnergy([]float64{2}, [][]float64{{0.5}})
	if len(got) != 1 || math.Abs(float64(got[0])-2) > 1e-9 {
		t.Fatalf("power energy=%v want=2", got)
	}
}

func TestApplyMelFiltersMagnitudeUsesMagnitudeSpectrum(t *testing.T) {
	got := applyMelFiltersMagnitude([]float64{2}, [][]float64{{0.5}})
	if len(got) != 1 || math.Abs(float64(got[0])-1) > 1e-9 {
		t.Fatalf("magnitude bands=%v want=1", got)
	}
}

func TestProcessMagnitudeAndEnergyUseDifferentSpectralContracts(t *testing.T) {
	processor := &MelProcessor{
		fftSize:    4,
		hopSize:    4,
		melBands:   1,
		hannWin:    []float64{1, 1, 1, 1},
		melFilters: [][]float64{{1, 1, 1}},
	}
	samples := []float64{2, 0, 0, 0}
	magnitude := processor.ProcessMagnitude(samples)
	power := processor.ProcessEnergy(samples)
	if len(magnitude) != 1 || len(power) != 1 {
		t.Fatalf("unexpected frame counts magnitude=%d power=%d", len(magnitude), len(power))
	}
	if math.Abs(float64(magnitude[0][0])-6) > 1e-9 {
		t.Fatalf("magnitude=%v want=6", magnitude[0][0])
	}
	if math.Abs(float64(power[0][0])-12) > 1e-9 {
		t.Fatalf("power=%v want=12", power[0][0])
	}
}

func TestMusiCNNFilterbankIsFiniteAndPopulated(t *testing.T) {
	filters := buildMelFilterbank(EssentiaMelBands, EssentiaFFTSize, EssentiaSampleRate)
	if len(filters) != EssentiaMelBands {
		t.Fatalf("filters=%d", len(filters))
	}
	for band, weights := range filters {
		nonZero := 0
		for _, weight := range weights {
			if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
				t.Fatalf("band=%d invalid weight=%v", band, weight)
			}
			if weight > 0 {
				nonZero++
			}
		}
		if nonZero == 0 {
			t.Fatalf("band=%d is empty", band)
		}
	}
}

func TestMusiCNNUnitTriangleFiltersHaveApproximatelyUnitArea(t *testing.T) {
	filters := buildMelFilterbank(EssentiaMelBands, EssentiaFFTSize, EssentiaSampleRate)
	binHz := float64(EssentiaSampleRate) / float64(EssentiaFFTSize)
	for band, weights := range filters {
		area := 0.0
		for _, weight := range weights {
			area += weight * binHz
		}
		// The triangles are sampled at FFT-bin centres, so discretization is
		// coarsest in the narrow low-frequency bands. We only need to catch a
		// regression back to unit-max / unnormalized triangles here.
		if area < 0.55 || area > 1.45 {
			t.Fatalf("band=%d unit_tri area=%v", band, area)
		}
	}
}

func TestSelectMelAnalysisWindowsDistributesFiveSegmentsForLongTrack(t *testing.T) {
	windows := SelectMelAnalysisWindows(3 * time.Minute)
	if len(windows) != 5 {
		t.Fatalf("windows=%d want=5", len(windows))
	}
	wantStarts := []time.Duration{
		0,
		42*time.Second + 750*time.Millisecond,
		85*time.Second + 500*time.Millisecond,
		128*time.Second + 250*time.Millisecond,
		171 * time.Second,
	}
	for i, window := range windows {
		if window.Start != wantStarts[i] || window.Duration != 9*time.Second {
			t.Fatalf("window[%d]=%+v want start=%v duration=9s", i, window, wantStarts[i])
		}
	}
}

func TestSelectMelAnalysisWindowsKeepsFortyFiveSecondBudget(t *testing.T) {
	windows := SelectMelAnalysisWindows(6 * time.Minute)
	total := time.Duration(0)
	for i, window := range windows {
		total += window.Duration
		if i > 0 && window.Start <= windows[i-1].Start {
			t.Fatalf("windows are not ordered: previous=%+v current=%+v", windows[i-1], window)
		}
	}
	if total != 45*time.Second {
		t.Fatalf("analysis budget=%v want=45s", total)
	}
}

func TestSelectMelAnalysisWindowsKeepsShortTrackSingle(t *testing.T) {
	windows := SelectMelAnalysisWindows(30 * time.Second)
	if len(windows) != 1 {
		t.Fatalf("windows=%d want=1", len(windows))
	}
	if windows[0].Start != 0 || windows[0].Duration != 30*time.Second {
		t.Fatalf("unexpected short-track window: %+v", windows[0])
	}
}

func TestMakeMelPatchesDoesNotRepatchPrepatchedData(t *testing.T) {
	frameCount := EssentiaPatchFrames + 62*2
	frames := make([]float32, frameCount*EssentiaMelBands)
	patched, patches, err := makeMelPatches(frames)
	if err != nil {
		t.Fatalf("makeMelPatches: %v", err)
	}
	if patches != 3 {
		t.Fatalf("patches=%d want=3", patches)
	}
	want := patches * EssentiaPatchFrames * EssentiaMelBands
	if len(patched) != want {
		t.Fatalf("len=%d want=%d", len(patched), want)
	}
}
