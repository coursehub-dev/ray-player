package onnx

import (
	"math"
	"testing"

	"ray-player1/internal/analysis"
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

func TestPrepareDiscogsPatchInputKeepsPrepatchedTensorExactlyOnce(t *testing.T) {
	const patches = 35
	mel := make([]float32, patches*128*96)
	for i := range mel {
		mel[i] = float32(i%97) / 97
	}
	got, count, err := prepareDiscogsPatchInput(mel, patches, 128, 96)
	if err != nil {
		t.Fatalf("prepareDiscogsPatchInput: %v", err)
	}
	if count != patches {
		t.Fatalf("patches=%d want=%d", count, patches)
	}
	if len(got) != len(mel) {
		t.Fatalf("len=%d want=%d", len(got), len(mel))
	}
	if len(got) > 0 && &got[0] != &mel[0] {
		t.Fatal("prepatched input must not be rebuilt or copied")
	}
}

func TestPrepareDiscogsPatchInputRejectsMalformedTensor(t *testing.T) {
	_, _, err := prepareDiscogsPatchInput(make([]float32, 35*128*96-1), 35, 128, 96)
	if err == nil {
		t.Fatal("expected strict prepatched contract error")
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

func TestApplyHeadPredictionsUsesRawRegressionRows(t *testing.T) {
	head := &essentiaHead{name: "approachability_regression-discogs-effnet-1"}
	result := EssentiaOutput{}
	applyHeadPredictions(
		&result,
		"approachability_regression-discogs-effnet-1",
		head,
		[]float32{4.2, 3.8, 4.4, 3.9},
		4,
	)
	if result.Approachability != 0.5 {
		t.Fatalf("invalid regression rows must use neutral fallback, got %.3f", result.Approachability)
	}

	applyHeadPredictions(
		&result,
		"approachability_regression-discogs-effnet-1",
		head,
		[]float32{0.2, 0.4, 0.6, 0.8},
		4,
	)
	if math.Abs(result.Approachability-0.5) > 1e-6 {
		t.Fatalf("valid regression rows value=%.3f want=0.5", result.Approachability)
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

func TestRegressionHeadProbeUsesProductionFallback(t *testing.T) {
	head := &essentiaHead{name: "approachability_regression-discogs-effnet-1", inputName: "embeddings", outputName: "activations", classes: []string{"approachability"}}
	report := buildHeadProbe(
		"approachability_regression-discogs-effnet-1",
		head,
		[]float32{4},
		[]float32{4.1, 4.0, 4.2, 3.9},
		ProbeOptions{},
	)
	if report.Aggregation.ChosenValue != 0.5 || report.Aggregation.ChosenMode != "neutral_fallback" {
		t.Fatalf("probe must mirror production regression fallback, got %+v", report.Aggregation)
	}
}

func TestBuildGenreProbeUsesActualPatchRows(t *testing.T) {
	classes := []string{"Rock---A", "Pop---B", "Electronic---C"}
	raw := []float32{
		0.9, 0.1, 0.2,
		0.1, 0.8, 0.3,
	}
	avg := []float32{0.5, 0.45, 0.25}
	report := buildGenreProbe(avg, raw, []int64{2, 3}, classes, 2, ProbeOptions{IncludeGenrePatchDebug: true})
	if len(report.PatchTop) != 2 {
		t.Fatalf("patch debug rows=%d want=2", len(report.PatchTop))
	}
	if len(report.PatchTop[0].Top) == 0 || report.PatchTop[0].Top[0].Label != "Rock / A" {
		t.Fatalf("first patch top=%+v", report.PatchTop[0])
	}
	if len(report.PatchTop[1].Top) == 0 || report.PatchTop[1].Top[0].Label != "Pop / B" {
		t.Fatalf("second patch top=%+v", report.PatchTop[1])
	}
}

func TestProbeFinalFeaturesMirrorProductionMoodThemeFusion(t *testing.T) {
	jamendo := HeadProbeReport{
		Name: "mtg_jamendo_moodtheme-discogs-effnet-1",
		StatsByClass: map[string]Stat{
			"melodic":   {Mean: 0.6},
			"soft":      {Mean: 0.5},
			"heavy":     {Mean: 0.4},
			"dream":     {Mean: 0.3},
			"emotional": {Mean: 0.7},
		},
	}
	heads := []HeadProbeReport{
		jamendo,
		{Name: "mood_relaxed-discogs-effnet-1", Aggregation: AggregationReport{ChosenValue: 0.5}},
		{Name: "mood_aggressive-discogs-effnet-1", Aggregation: AggregationReport{ChosenValue: 0.2}},
		{Name: "tonal_atonal-discogs-effnet-1", Aggregation: AggregationReport{ChosenValue: 0.4}},
	}
	features := extractFinalFeatures(EssentiaProbeReport{Heads: heads}, analysis.Features{})
	if features.Melodic < 0.6 || features.Soft < 0.5 || features.Heavy < 0.4 || features.Dream < 0.3 || features.Emotional < 0.7 {
		t.Fatalf("probe moodtheme fusion diverged from production: %+v", features)
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

func TestGenreTagsSuppressWeakDetailsByEvidenceOnly(t *testing.T) {
	groups := []GenreGroupCandidate{
		{
			Label:          "Rock",
			Score:          0.22,
			BestSubLabel:   "Rock / Substyle A",
			BestSubScore:   0.22,
			SecondSubScore: 0.20,
			Support:        1,
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
		t.Fatalf("weak/ambiguous detail must be suppressed, got %q", tags[0].Detail)
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
				BestSubLabel: "Hip Hop / Boom Bap",
				BestSubScore: test.bestSub,
				Support:      1,
			}}
			if tags := buildGenreTagsForUI(groups, 3); len(tags) != test.want {
				t.Fatalf("tags=%#v want len=%d", tags, test.want)
			}
		})
	}
}

func TestGenreTagsDoNotHardcodeLabelDenylist(t *testing.T) {
	groups := []GenreGroupCandidate{
		{Label: "Classical", Score: 0.20, BestSubLabel: "Classical / Modern", BestSubScore: 0.20, Support: 4},
		{Label: "Stage & Screen", Score: 0.12, BestSubLabel: "Stage & Screen / Soundtrack", BestSubScore: 0.12, Support: 3},
	}

	tags := buildGenreTagsForUI(groups, 3)
	if len(tags) != 2 {
		t.Fatalf("model labels must be filtered by numeric evidence, not a text denylist: %#v", tags)
	}
	if tags[0].Label != "Classical" || tags[1].Label != "Stage & Screen" {
		t.Fatalf("unexpected tag order: %#v", tags)
	}
}

func TestGenreTagsUseAcceptedPrimaryForSecondaryThreshold(t *testing.T) {
	groups := []GenreGroupCandidate{
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
	if !genreResultAccepted(true, result.GenreScore, result.GenreMargin) {
		t.Fatalf("near-tied multi-label genres must remain usable: score=%.3f margin=%.3f", result.GenreScore, result.GenreMargin)
	}
}

func TestGenreResultAcceptedRequiresReliableScore(t *testing.T) {
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
		{name: "low margin is ambiguous but usable", reliable: true, score: 1, margin: 0, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := genreResultAccepted(test.reliable, test.score, test.margin); got != test.want {
				t.Fatalf("genreResultAccepted(%t, %.3f, %.3f)=%t want=%t", test.reliable, test.score, test.margin, got, test.want)
			}
		})
	}
}

func TestGenreDetailForUIUsesGenericScoreAndSeparation(t *testing.T) {
	for _, tc := range []GenreGroupCandidate{
		{
			Label:          "Rock",
			BestSubLabel:   "Rock---Substyle A",
			BestSubScore:   genreDetailMinScore - 0.001,
			SecondSubScore: 0.01,
		},
		{
			Label:          "Electronic",
			BestSubLabel:   "Electronic---Substyle A",
			BestSubScore:   0.24,
			SecondSubScore: 0.22,
		},
	} {
		if got := genreDetailForUI(tc); got != "" {
			t.Fatalf("genreDetailForUI(%+v)=%q want empty", tc, got)
		}
	}

	if got := genreDetailForUI(GenreGroupCandidate{
		Label:          "Rock",
		BestSubLabel:   "Rock---Substyle A",
		BestSubScore:   0.28,
		SecondSubScore: 0.10,
	}); got != "Rock / Substyle A" {
		t.Fatalf("clear substyle detail=%q", got)
	}

	if got := genreDetailForUI(GenreGroupCandidate{
		Label:          "Pop",
		BestSubLabel:   "Pop---Substyle A",
		BestSubScore:   genreDetailStrongScore,
		SecondSubScore: genreDetailStrongScore - 0.01,
	}); got != "Pop / Substyle A" {
		t.Fatalf("strong substyle may survive a close runner-up, got %q", got)
	}
}

func TestTopGenreGroupsTracksSecondSubstyleEvidence(t *testing.T) {
	classes := []string{"Pop---A", "Pop---B", "Pop---C"}
	groups := topGenreGroups([]float32{0.31, 0.22, 0.08}, classes, 3)
	if len(groups) != 1 {
		t.Fatalf("groups=%#v", groups)
	}
	if math.Abs(float64(groups[0].BestSubScore-0.31)) > 1e-6 ||
		math.Abs(float64(groups[0].SecondSubScore-0.22)) > 1e-6 {
		t.Fatalf("unexpected substyle evidence: %+v", groups[0])
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
