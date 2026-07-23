package analysis

import "testing"

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
