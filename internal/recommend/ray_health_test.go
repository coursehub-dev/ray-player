package recommend

import (
	"strings"
	"testing"

	"ray-player1/internal/library"
	"ray-player1/internal/rays"
)

func TestInsightConfidenceDropsWhenTempoIsBroken(t *testing.T) {
	tk := library.Track{
		Energy:          0.8,
		Danceability:    0.8,
		Valence:         0.2,
		Sad:             1.0,
		Tonality:        1.0,
		Approachability: 1.0,
		Engagement:      1.0,
		TempoSource:     "error",
		TempoError:      "Invalid rank for input",
		TempoConfidence: 0,
		TempoStability:  0,
		AnalyzedLevel:   2,
		AnalysisStatus:  "done",
		Embedding:       []float32{0.1, 0.2},
	}

	conf := insightConfidence(tk)
	if conf >= 0.9 {
		t.Fatalf("confidence should not be high with broken tempo and saturated heads: %.3f", conf)
	}
}

func TestInsightWarningDetectsBrokenSensors(t *testing.T) {
	tk := library.Track{
		Sad:             1.0,
		Tonality:        1.0,
		Approachability: 1.0,
		Engagement:      1.0,
		TempoSource:     "error",
		TempoError:      "Invalid rank for input",
	}

	ins := rays.QueueInsight{
		TempoUnknown: true,
		Novelty:      1.0,
		Confidence:   1.0,
	}

	w := insightWarning(ins, tk)
	for _, want := range []string{"tempo_unknown", "genre_weak", "nov_saturated", "sad_saturated", "tonal_saturated", "conf_suspect"} {
		if !strings.Contains(w, want) {
			t.Fatalf("warning %q missing %q", w, want)
		}
	}
}

func TestCanPlaceDiscoveryLimitsWarmUp(t *testing.T) {
	queue := []rays.QueueItem{
		{Insight: rays.QueueInsight{Bucket: bucketCore}},
		{Insight: rays.QueueInsight{Bucket: bucketAdjacent}},
		{Insight: rays.QueueInsight{Bucket: bucketDiscovery, Strategy: "discovery_safe"}},
		{Insight: rays.QueueInsight{Bucket: bucketDiscovery, Strategy: "discovery_safe"}},
	}

	cand := scored{bucket: bucketDiscovery, strategy: "wildcard"}
	if canPlaceDiscovery(queue, cand, TrajectoryWarmUp) {
		t.Fatalf("warm_up should not allow more discovery/wildcard")
	}
}

func TestLogRayHealth(t *testing.T) {
	queue := []rays.QueueItem{{
		TrackID: "1",
		Track: library.Track{
			Sad:             1,
			Tonality:        1,
			TempoSource:     "error",
			TempoError:      "bad",
			TempoConfidence: 0,
			TempoStability:  0,
		},
		Insight: rays.QueueInsight{TempoUnknown: true, Novelty: 1, JumpPenalty: 0, Bucket: bucketDiscovery, Strategy: "wildcard"},
	}}
	logRayHealth(queue)
}
