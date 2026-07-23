package main

import (
	"testing"

	"ray-player1/internal/emotion"
)

func TestToUltraShortTrackKeepsRequiredFields(t *testing.T) {
	tr := ShortTrackProbeReport{
		File:            "test.mp3",
		Expected:        "joy_party",
		Match:           true,
		CompatibleMatch: true,
		F: ShortFeatureValues{Dance: 0.91, Val: 0.63, Happy: 0.21, Relax: 0.67, Party: 0.42},
		Audio2: ShortAudioTexture2{Flatness: 0.02, Rolloff85: 3300, Flux: 0.21, OnsetRate: 1.3},
		Basis3: ShortBasis3{Label: "joy_party", Motion: 0.71, Joy: 0.64, Combat: 0.18, SprintClean: 0.69},
		Basis3Debug: Basis3Debug{CleanParty: 0.72, TopLabels: []LabelScore{{Label: "joy_party", Score: 0.72, Passed: true}}},
	}

	got := toUltraShortTrack(tr)
	if got.Got != "joy_party" || !got.Match || !got.Compatible {
		t.Fatalf("unexpected labels in ultrashort track: %+v", got)
	}
	if got.Feeling.Joy != tr.Basis3.Joy || got.Feeling.Sprint != tr.Basis3.SprintClean {
		t.Fatalf("ultrashort must reuse basis3 values")
	}
	if got.ML.Dance != tr.F.Dance || got.ML.Valence != tr.F.Val || got.ML.BPM != 0 {
		t.Fatalf("ultrashort ML block mismatch: %+v", got.ML)
	}
	if len(got.Top) == 0 || got.Top[0].Label != "joy_party" {
		t.Fatalf("ultrashort must expose top labels")
	}
}

func TestToUltraShortReportMarksMode(t *testing.T) {
	r := ShortBatchAudioProbeReport{Tool: "audio_probe_batch", Version: "1", Mode: "short", FormulaVersion: emotion.DefaultTuning().Version}
	got := toUltraShortReport(r)
	if got.Mode != "ultrashort" {
		t.Fatalf("expected ultrashort mode, got %s", got.Mode)
	}
}
