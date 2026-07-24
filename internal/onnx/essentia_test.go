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

func TestAggregateRegressionHeadRejectsOutOfRangeSaturation(t *testing.T) {
	d := aggregateRegressionHead([]float32{4.2, 3.8, 4.4, 3.9}, 4)
	if d.Reliable {
		t.Fatalf("out-of-range regression output must be rejected: %+v", d)
	}
	if d.Value != 0.5 {
		t.Fatalf("invalid regression must use neutral fallback, got %.3f", d.Value)
	}
	if d.OutOfRange != 4 {
		t.Fatalf("outOfRange=%d want=4", d.OutOfRange)
	}
}

func TestAggregateRegressionHeadRejectsExactBoundaryLock(t *testing.T) {
	d := aggregateRegressionHead([]float32{1, 1, 1, 1, 1, 1}, 6)
	if d.Reliable || !d.Saturated {
		t.Fatalf("exact regression boundary lock must be rejected: %+v", d)
	}
	if d.Value != 0.5 {
		t.Fatalf("saturated regression must use neutral fallback, got %.3f", d.Value)
	}
}

func TestAggregateRegressionHeadUsesTrimmedMean(t *testing.T) {
	d := aggregateRegressionHead([]float32{0.2, 0.4, 0.6, 0.8}, 4)
	if !d.Reliable {
		t.Fatalf("expected reliable regression output: %+v", d)
	}
	if math.Abs(d.Value-0.5) > 0.0001 {
		t.Fatalf("value=%.4f want=0.5", d.Value)
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
	probs := []float32{0.42, 0.38, 0.44, 0.39}
	report := buildHeadProbe("approachability_regression-discogs-effnet-1", head, []float32{0.41}, probs, ProbeOptions{IncludePatchRows: true})
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

func TestGenreTagsSuppressNoisyWeakDetails(t *testing.T) {
	groups := []GenreGroupCandidate{
		{
			Label:        "Rock",
			Score:        0.22,
			BestSubLabel: "Rock / Black Metal",
			BestSubScore: 0.22,
			Support:      1,
		},
	}
	tags := buildGenreTagsForUI(groups, 3)
	if len(tags) != 1 {
		t.Fatalf("tags=%#v", tags)
	}
	if tags[0].Label != "Rock" {
		t.Fatalf("label=%q want Rock", tags[0].Label)
	}
	if tags[0].Detail != "" {
		t.Fatalf("weak noisy detail must be suppressed, got %q", tags[0].Detail)
	}
}

func TestGenreTagsRequireConfidentPrimary(t *testing.T) {
	tests := []struct {
		name    string
		score   float32
		bestSub float32
		want    int
	}{
		{
			name:    "below grouped score threshold",
			score:   genrePrimaryMinScore - 0.001,
			bestSub: 0.99,
			want:    0,
		},
		{
			name:    "at grouped score threshold",
			score:   genrePrimaryMinScore,
			bestSub: 0.01,
			want:    1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := []GenreGroupCandidate{{
				Label:        "Hip Hop",
				Score:        test.score,
				BestSubLabel: "Hip Hop / DJ Battle Tool",
				BestSubScore: test.bestSub,
				Support:      1,
			}}
			if tags := buildGenreTagsForUI(groups, 3); len(tags) != test.want {
				t.Fatalf("tags=%#v want len=%d", tags, test.want)
			}
		})
	}
}

func TestGenreTagsUseAcceptedPrimaryForSecondaryThreshold(t *testing.T) {
	groups := []GenreGroupCandidate{
		{Label: "Non-Music", Score: 0.90, BestSubScore: 0.90, Support: 4},
		{Label: "Rock", Score: 0.10, BestSubScore: 0.11, Support: 3},
		{Label: "Electronic", Score: 0.09, BestSubScore: 0.10, Support: 3},
	}

	tags := buildGenreTagsForUI(groups, 3)
	if len(tags) != 2 {
		t.Fatalf("expected Rock and Electronic tags, got %#v", tags)
	}
	if tags[0].Label != "Rock" || tags[1].Label != "Electronic" {
		t.Fatalf("unexpected tag order: %#v", tags)
	}

	result := EssentiaOutput{GenreTags: tags}
	finalizeGenreResult(&result)
	if genreResultAccepted(true, result.GenreScore, result.GenreMargin) {
		t.Fatalf("near-tied genres must fail the confidence gate: score=%.3f margin=%.3f", result.GenreScore, result.GenreMargin)
	}
}

func TestGenreResultAcceptedRequiresReliableScoreAndMargin(t *testing.T) {
	tests := []struct {
		name     string
		reliable bool
		score    float64
		margin   float64
		want     bool
	}{
		{name: "accepted", reliable: true, score: float64(genrePrimaryMinScore), margin: float64(genrePrimaryMinMargin), want: true},
		{name: "unreliable", reliable: false, score: 1, margin: 1, want: false},
		{name: "score below threshold", reliable: true, score: float64(genrePrimaryMinScore) - 0.001, margin: 1, want: false},
		{name: "margin below threshold", reliable: true, score: 1, margin: float64(genrePrimaryMinMargin) - 0.001, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := genreResultAccepted(test.reliable, test.score, test.margin); got != test.want {
				t.Fatalf("genreResultAccepted(%t, %.3f, %.3f)=%t want=%t", test.reliable, test.score, test.margin, got, test.want)
			}
		})
	}
}

func TestGenreDetailForUISuppressesKnownWeakCollapseLabels(t *testing.T) {
	for _, tc := range []GenreGroupCandidate{
		{Label: "Rock", BestSubLabel: "Rock---Black Metal", BestSubScore: 0.22, Support: 4},
		{Label: "Hip Hop", BestSubLabel: "Hip Hop---DJ Battle Tool", BestSubScore: 0.20, Support: 3},
		{Label: "Rock", BestSubLabel: "Rock---Pop Rock", BestSubScore: 0.15, Support: 5},
	} {
		if got := genreDetailForUI(tc); got != "" {
			t.Fatalf("genreDetailForUI(%+v)=%q want empty", tc, got)
		}
	}
	if got := genreDetailForUI(GenreGroupCandidate{
		Label: "Rock", BestSubLabel: "Rock---Black Metal", BestSubScore: 0.31, Support: 4,
	}); got != "Rock / Black Metal" {
		t.Fatalf("strong collapse-prone detail=%q", got)
	}
	if got := genreDetailForUI(GenreGroupCandidate{
		Label: "Electronic", BestSubLabel: "Electronic---Deep House", BestSubScore: 0.19, Support: 3,
	}); got != "Electronic / Deep House" {
		t.Fatalf("ordinary detail=%q", got)
	}
}

func TestFlattenPatchPredictionsPreservesAllRows(t *testing.T) {
	rows := [][]float32{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}}
	got := flattenPatchPredictions(rows)
	if len(got) != 6 {
		t.Fatalf("len=%d want=6", len(got))
	}
	if got[0] != 0.1 || got[5] != 0.6 {
		t.Fatalf("unexpected flattened values: %#v", got)
	}
}
