package analysis

import (
	"testing"
	"time"
)

func TestDefaultMelModeIsOfficial(t *testing.T) {
	if DefaultMelMode() != MelModeOfficial {
		t.Fatalf("default mel mode = %q, want %q", DefaultMelMode(), MelModeOfficial)
	}
}

func TestApplyMelModeOfficialIsIdentity(t *testing.T) {
	in := []float32{0, 0.5, 2, 10, 50}
	out := transformMelFrame(in, string(MelModeOfficial))
	if len(out) != len(in) {
		t.Fatalf("len mismatch: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("official mode changed value at %d: got %v want %v", i, out[i], in[i])
		}
	}
	out[0] = 999
	if in[0] == 999 {
		t.Fatal("apply must return a copy, not alias input")
	}
}

func TestSelectMelAnalysisWindowsUsesThreeSegmentsForLongTrack(t *testing.T) {
	windows := SelectMelAnalysisWindows(3 * time.Minute)
	if len(windows) != 3 {
		t.Fatalf("windows=%d want=3", len(windows))
	}
	if windows[0].Start != 0 || windows[0].Duration != 15*time.Second {
		t.Fatalf("unexpected first window: %+v", windows[0])
	}
	if windows[1].Start != 82*time.Second+500*time.Millisecond {
		t.Fatalf("unexpected middle window start: %v", windows[1].Start)
	}
	if windows[2].Start != 165*time.Second {
		t.Fatalf("unexpected final window start: %v", windows[2].Start)
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
