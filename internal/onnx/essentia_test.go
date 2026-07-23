package onnx

import (
	"math"
	"testing"
)

func TestResolveBaseInputShapeSupportsCurrentDiscogsLayout(t *testing.T) {
	shape, err := resolveBaseInputShape([]interface{}{"n", float64(128), float64(96)}, 5, 5*128*96)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shape) != 3 || shape[0] != 5 || shape[1] != 128 || shape[2] != 96 {
		t.Fatalf("unexpected shape: %#v", shape)
	}
}

func TestResolveBaseInputShapeRejectsMismatchedMelSize(t *testing.T) {
	_, err := resolveBaseInputShape([]interface{}{"n", float64(128), float64(96)}, 2, 100)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestResolveBaseInputShapeSupportsFixedBatch64(t *testing.T) {
	shape, err := resolveBaseInputShape([]interface{}{float64(64), float64(128), float64(96)}, 29, 29*128*96)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shape) != 3 || shape[0] != 64 || shape[1] != 128 || shape[2] != 96 {
		t.Fatalf("unexpected shape: %#v", shape)
	}
}

func TestPrepareBaseMelInputPadsToFixedBatch(t *testing.T) {
	mel := make([]float32, 29*128*96)
	for i := range mel {
		mel[i] = 1
	}
	out, valid, shape, err := prepareBaseMelInput([]interface{}{float64(64), float64(128), float64(96)}, mel, 29)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid != 29 {
		t.Fatalf("unexpected valid patches: %d", valid)
	}
	if len(shape) != 3 || shape[0] != 64 {
		t.Fatalf("unexpected shape: %#v", shape)
	}
	if got, want := len(out), 64*128*96; got != want {
		t.Fatalf("unexpected output len: got %d want %d", got, want)
	}
	if out[len(mel)] != 0 {
		t.Fatal("expected zero padding after copied mel")
	}
}

func TestAverageHeadPredictionsAveragesPerPatch(t *testing.T) {
	avg := averageHeadPredictions([]float32{0.1, 0.9, 0.3, 0.7, 0.5, 0.5}, 3)
	if len(avg) != 2 {
		t.Fatalf("unexpected avg len: %d", len(avg))
	}
	if avg[0] != 0.3 || avg[1] != 0.7 {
		t.Fatalf("unexpected avg values: %#v", avg)
	}
}

func TestRegressionHeadValueClamps(t *testing.T) {
	if got := regressionHeadValue([]float32{1.2}); got != 1 {
		t.Fatalf("expected clamp to 1, got %f", got)
	}
	if got := regressionHeadValue([]float32{-0.2}); got != 0 {
		t.Fatalf("expected clamp to 0, got %f", got)
	}
}

func TestPositiveClassProbabilityUsesClassName(t *testing.T) {
	head := &essentiaHead{name: "mood_sad-discogs-effnet-1", classes: []string{"non_sad", "sad"}}
	values := []float32{0.91, 0.09}
	if got := headClassProbability(head, values, "sad"); got > 0.10 {
		t.Fatalf("expected sad from index 1, got %.3f", got)
	}
	if got := headClassProbability(head, values, "non_sad"); got < 0.90 {
		t.Fatalf("expected non_sad from index 0, got %.3f", got)
	}
}

func TestAggressiveUsesIndexZero(t *testing.T) {
	head := &essentiaHead{name: "mood_aggressive-discogs-effnet-1", classes: []string{"aggressive", "not_aggressive"}}
	values := []float32{0.82, 0.18}
	if got := headClassProbability(head, values, "aggressive"); got < 0.80 {
		t.Fatalf("expected aggressive from index 0, got %.3f", got)
	}
}

func TestPickHeadPredictionOutputPrefersPredictionOverActivations(t *testing.T) {
	got := pickHeadPredictionOutput("mood_sad-discogs-effnet-1", []string{"activations", "softmax"}, []string{"non_sad", "sad"})
	if got != "softmax" {
		t.Fatalf("expected softmax output, got %q", got)
	}
}

func TestTopGenreGroupsAggregatesParents(t *testing.T) {
	classes := []string{"Rock---Black Metal", "Rock---Alternative Rock", "Electronic---Experimental", "Hip Hop---DJ Battle Tool"}
	probs := []float32{0.083, 0.02, 0.01, 0.005}
	groups := topGenreGroups(probs, classes, 3)
	if len(groups) == 0 {
		t.Fatal("expected grouped genres")
	}
	if groups[0].Label != "Rock" {
		t.Fatalf("unexpected top group: %#v", groups[0])
	}
	primary, detail, score, margin := choosePrimaryGenre(groups, topKGenres(probs, classes, 4))
	if primary != "Rock" {
		t.Fatalf("unexpected primary genre: %q", primary)
	}
	if detail != "Rock / Black Metal" {
		t.Fatalf("unexpected detail genre: %q", detail)
	}
	if score <= 0 {
		t.Fatalf("expected positive score, got %f", score)
	}
	if margin < 0 {
		t.Fatalf("unexpected negative margin: %f", margin)
	}
}

func TestBuildHeadProbeUsesPerRowSeries(t *testing.T) {
	head := &essentiaHead{name: "mood_relaxed-discogs-effnet-1", inputName: "embeddings", outputName: "activations", classes: []string{"non_relaxed", "relaxed"}}
	probs := []float32{0.10, 0.90, 0.20, 0.80, 0.30, 0.70, 0.40, 0.60}
	report := buildHeadProbe("mood_relaxed-discogs-effnet-1", head, []float32{0.25, 0.75}, probs, ProbeOptions{IncludePatchRows: true, MaxPatchRows: 4})
	if report.PositiveStats.Count != 4 {
		t.Fatalf("expected per-row series count 4, got %d", report.PositiveStats.Count)
	}
	if report.Shape[0] != 4 || report.Shape[1] != 2 {
		t.Fatalf("unexpected shape: %#v", report.Shape)
	}
	if len(report.FirstRows) != 4 {
		t.Fatalf("expected first rows debug, got %#v", report.FirstRows)
	}
	if got := report.StatsByClass["relaxed"].Count; got != 4 {
		t.Fatalf("expected stats per class to include all rows, got %d", got)
	}
}

func TestRegressionHeadProbeAvoidsBinaryWarnings(t *testing.T) {
	head := &essentiaHead{name: "approachability_regression-discogs-effnet-1", inputName: "embeddings", outputName: "activations", classes: []string{"approachability"}}
	probs := []float32{4.2, 3.8, 4.4, 3.9}
	report := buildHeadProbe("approachability_regression-discogs-effnet-1", head, []float32{4.1}, probs, ProbeOptions{IncludePatchRows: true})
	if report.PositiveStats.Count != 4 {
		t.Fatalf("expected regression series count 4, got %d", report.PositiveStats.Count)
	}
	for _, w := range report.Warnings {
		if w == "near_zero_saturated" || w == "near_one_saturated" || w == "binary_like" {
			t.Fatalf("regression head must not use binary warnings, got %v", report.Warnings)
		}
	}
}

func TestMeanEmbeddingKeeps1280Dimensions(
	t *testing.T,
) {
	const (
		rows    = 3
		columns = 1280
	)

	values := make(
		[]float32,
		rows*columns,
	)
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			values[row*columns+column] =
				float32(row + 1)
		}
	}

	result := meanEmbedding(
		values,
		rows,
		columns,
	)
	if len(result) != columns {
		t.Fatalf(
			"embedding size=%d want=%d",
			len(result),
			columns,
		)
	}
	for index, value := range result {
		if value != 2 {
			t.Fatalf(
				"embedding[%d]=%v want=2",
				index,
				value,
			)
		}
	}
}

func TestDecodeTempoOutputAndMajorityVoting(t *testing.T) {
	probs := make([]float32, 256)
	probs[98] = 0.8
	bpm, conf, idx := DecodeTempoOutput(probs)
	if bpm != 128 || math.Abs(conf-0.8) > 0.00001 || idx != 98 {
		t.Fatalf("unexpected decode result bpm=%v conf=%v idx=%d", bpm, conf, idx)
	}
	result := AggregateTempoMajorityVoting([]tempoPrediction{{BPM: 64, Confidence: 0.7}, {BPM: 128, Confidence: 0.8}, {BPM: 127, Confidence: 0.6}})
	if result.BPM != 128 {
		t.Fatalf("expected perceived majority bucket 128, got %f", result.BPM)
	}
}
