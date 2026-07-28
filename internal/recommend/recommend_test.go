package recommend

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"ray-player1/internal/emotion"
	"ray-player1/internal/library"
	"ray-player1/internal/rays"
)

func TestBuildRayKeepsSeedFirstAndUnique(t *testing.T) {
	svc := &Service{}
	seed := testTrack("seed", "Seed Artist", 0, 120, 0.7, []float32{1, 0, 0})
	tracks := []library.Track{
		seed,
		testTrack("a1", "Artist A", 0, 121, 0.69, []float32{0.99, 0.01, 0}),
		testTrack("a2", "Artist A", 0, 122, 0.68, []float32{0.98, 0.02, 0}),
		testTrack("b1", "Artist B", 1, 118, 0.72, []float32{0.88, 0.12, 0}),
		testTrack("c1", "Artist C", 2, 114, 0.51, []float32{0.76, 0.24, 0}),
		testTrack("d1", "Artist D", 3, 110, 0.44, []float32{0.61, 0.39, 0}),
		testTrack("e1", "Artist E", 4, 108, 0.40, []float32{0.55, 0.45, 0}),
	}
	queue := svc.BuildRay(seed, tracks, "")
	if len(queue) < 2 {
		t.Fatalf("expected queue to include seed and recommendations, got %d", len(queue))
	}
	if queue[0].TrackID != seed.ID || !queue[0].IsCurrent {
		t.Fatalf("expected seed to be first/current, got %+v", queue[0])
	}
	seen := map[string]bool{}
	for _, item := range queue {
		if seen[item.TrackID] {
			t.Fatalf("duplicate track in queue: %s", item.TrackID)
		}
		seen[item.TrackID] = true
	}
}

func TestFeedbackScoringPenalizesEarlySkipsAndRecentRepeats(t *testing.T) {
	recent := feedbackItem{lastPlayedAt: 10, avgCompletion: 0.9, playEvents: 3}
	earlySkip := feedbackItem{lastSkippedAt: 20, lastEventType: "early_skip", skipEvents: 3, avgCompletion: 0.1}
	stable := feedbackItem{lastPlayedAt: 5, avgCompletion: 0.8, completeEvents: 3, affinity: 0.4}

	if notRecentlyPlayedScore(testTrack("recent", "Artist A", 0, 120, 0.7, []float32{1}), recent, true) >= notRecentlyPlayedScore(testTrack("stable", "Artist C", 0, 120, 0.7, []float32{1}), stable, false) {
		t.Fatal("expected recent track to receive lower novelty allowance")
	}
	if skipRiskScore(testTrack("skipme", "Artist B", 0, 120, 0.7, []float32{1}), earlySkip) <= skipRiskScore(testTrack("stable", "Artist C", 0, 120, 0.7, []float32{1}), stable) {
		t.Fatal("expected early skip feedback to increase skip risk")
	}
	if userAffinityScore(testTrack("stable", "Artist C", 0, 120, 0.7, []float32{1}), stable) <= userAffinityScore(testTrack("skipme", "Artist B", 0, 120, 0.7, []float32{1}), earlySkip) {
		t.Fatal("expected stable completed track to have stronger user affinity")
	}
}

func TestBucketTargetsForCompactRay(t *testing.T) {
	core, adjacent, discovery := bucketTargets(20)
	if core != 11 || adjacent != 5 || discovery != 3 {
		t.Fatalf("unexpected 20-track mix: core=%d adjacent=%d discovery=%d", core, adjacent, discovery)
	}
	core, adjacent, discovery = bucketTargets(30)
	if core != 17 || adjacent != 7 || discovery != 5 {
		t.Fatalf("unexpected 30-track mix: core=%d adjacent=%d discovery=%d", core, adjacent, discovery)
	}
}

func TestBuildRaySuppressesSameArtistFlood(t *testing.T) {
	svc := &Service{}
	seed := testTrack("seed", "Seed Artist", 0, 120, 0.7, []float32{1, 0, 0})
	tracks := []library.Track{seed}
	for i := 0; i < 6; i++ {
		tracks = append(tracks, testTrack(idNum("same", i), "Seed Artist", 0, 120, 0.7, []float32{0.99, 0.01, 0}))
	}
	tracks = append(tracks,
		testTrack("other1", "Artist A", 0, 121, 0.68, []float32{0.98, 0.02, 0}),
		testTrack("other2", "Artist B", 1, 118, 0.64, []float32{0.90, 0.10, 0}),
		testTrack("other3", "Artist C", 2, 114, 0.58, []float32{0.84, 0.16, 0}),
	)
	queue := svc.BuildRay(seed, tracks, "")
	for i := 1; i < minInt(len(queue), 4); i++ {
		if queue[i].Strategy == "same_artist" {
			t.Fatalf("expected first recommendation block to avoid same-artist flood: %+v", queue)
		}
	}
}

func TestBuildRayProducesBucketMetadata(t *testing.T) {
	svc := &Service{}
	seed := testTrack("seed", "Seed Artist", 0, 120, 0.7, []float32{1, 0, 0})
	tracks := []library.Track{
		seed,
		testTrack("core", "Artist A", 0, 120, 0.7, []float32{0.99, 0.01, 0}),
		testTrack("adj", "Artist B", 1, 116, 0.62, []float32{0.8, 0.2, 0}),
		testTrack("disc", "Artist C", 3, 112, 0.45, []float32{0.7, 0.3, 0}),
	}
	queue := svc.BuildRay(seed, tracks, "")
	if len(queue) < 3 {
		t.Fatalf("expected at least 3 queue items, got %d", len(queue))
	}
	foundBucket := false
	for _, item := range queue[1:] {
		if item.Bucket != "" {
			foundBucket = true
		}
		if item.Strategy == "" {
			t.Fatalf("expected strategy for item %+v", item)
		}
	}
	if !foundBucket {
		t.Fatal("expected at least one non-seed bucket marker")
	}
}

func TestEmoflowCandidateAffinityPrefersBridge(t *testing.T) {
	seed := withMood(testTrack("seed", "Seed", 0, 118, 0.40, []float32{1, 0, 0}), 0.44, 0.32, 0.62, 0.28, 0.72, 0.28, 0.60, 0.34, 0.22)
	bridge := withMood(testTrack("bridge", "Bridge", 1, 120, 0.43, []float32{0.82, 0.18, 0}), 0.46, 0.34, 0.58, 0.30, 0.70, 0.30, 0.58, 0.36, 0.24)
	jump := withMood(testTrack("jump", "Jump", 2, 148, 0.88, []float32{0.96, 0.04, 0}), 0.90, 0.82, 0.08, 0.88, 0.18, 0.84, 0.14, 0.90, 0.86)
	if emoflowCandidateAffinity(seed, bridge) <= emoflowCandidateAffinity(seed, jump) {
		t.Fatal("expected bridge affinity to exceed hard jump")
	}
}

func TestBuildRayAvoidsHardMoodJumpWhenSmoothOptionExists(t *testing.T) {
	svc := &Service{}
	seed := withMood(testTrack("seed", "Seed Artist", 0, 118, 0.28, []float32{1, 0, 0}), 0.30, 0.20, 0.82, 0.05, 0.82, 0.10, 0.78, 0.10, 0.15)
	smooth := withMood(testTrack("smooth", "Artist A", 0, 119, 0.31, []float32{0.94, 0.06, 0}), 0.33, 0.24, 0.78, 0.08, 0.76, 0.14, 0.72, 0.12, 0.18)
	jump := withMood(testTrack("jump", "Artist B", 0, 120, 0.90, []float32{0.995, 0.005, 0}), 0.92, 0.95, 0.04, 0.88, 0.05, 0.92, 0.08, 0.95, 0.85)
	tracks := []library.Track{seed, jump, smooth}
	queue := svc.BuildRay(seed, tracks, "")
	if len(queue) < 2 {
		t.Fatalf("expected recommendations, got %d", len(queue))
	}
	if queue[1].TrackID != "smooth" {
		t.Fatalf("expected smooth transition first, got %+v", queue[1])
	}
}

func TestExplainTransitionProducesHumanReason(t *testing.T) {
	prev := withMood(testTrack("prev", "Artist A", 0, 118, 0.35, []float32{1, 0, 0}), 0.35, 0.30, 0.70, 0.10, 0.68, 0.12, 0.62, 0.18, 0.20)
	next := withMood(testTrack("next", "Artist B", 0, 120, 0.48, []float32{0.96, 0.04, 0}), 0.52, 0.34, 0.64, 0.12, 0.60, 0.18, 0.58, 0.22, 0.24)
	reason := explainTransition(prev, next, bucketCore)
	if reason == "" || reason == "подходит к текущему лучу" {
		t.Fatalf("expected more specific reason, got %q", reason)
	}
}

func TestInferRayModeKeepsAggressiveSeedInContinueMood(t *testing.T) {
	seed := withMood(testTrack("seed", "Rob Zombie", 0, 115, 0.55, []float32{1, 0, 0}), 0.55, 0.35, 0.35, 0.05, 0.30, 0.08, 0.16, 0.35, 0.02)
	seed.SpectralFlatness = 0.08
	seed.SpectralFlux = 0.44
	seed.ZeroCrossingRate = 0.20
	seed.OnsetRate = 1.8
	seed.RMS = 0.45
	seed.Loudness = -32

	mode := inferRayMode(seed, []library.Track{seed}, nil, FeatureNormalizer{})
	if mode != TrajectoryContinueMood {
		t.Fatalf("aggressive seed should continue mood by default, got %s", mode)
	}
}

func TestExplainEmotionTransitionDoesNotBrightenHeavyTrack(t *testing.T) {
	ctx := rankingContext{Normalizer: FeatureNormalizer{}}
	prev := withMood(testTrack("prev", "Seed", 0, 115, 0.48, []float32{1, 0, 0}), 0.48, 0.45, 0.52, 0.02, 0.40, 0.04, 0.20, 0.18, 0.00)
	next := withMood(testTrack("next", "Du Hast", 0, 115, 0.62, []float32{0.98, 0.02, 0}), 0.62, 0.42, 0.35, 0.02, 0.25, 0.03, 0.12, 0.42, 0.01)
	next.SpectralFlatness = 0.10
	next.SpectralFlux = 0.55
	next.ZeroCrossingRate = 0.22
	next.OnsetRate = 2.2
	got := explainEmotionTransition(ctx, prev, next)
	if strings.Contains(got, "светлее") || strings.Contains(got, "радостнее") {
		t.Fatalf("heavy transition must not be described as brighten/joy: %q", got)
	}
}

func TestExplainEmotionTransitionRejectsFalseCloseVibe(t *testing.T) {
	ctx := rankingContext{Normalizer: FeatureNormalizer{}}
	hard := withMood(testTrack("hard", "Rob Zombie", 0, 115, 0.49, []float32{1, 0, 0}), 0.49, 0.57, 0.70, 0.00, 0.21, 0.00, 0.14, 0.46, 0.00)
	hard.ZeroCrossingRate = 0.184
	hard.SpectralCentroid = 3040
	hard.RMS = 0.360
	hard.Loudness = -38.42
	hard.Electronicness = 0.46
	soft := withMood(testTrack("soft", "Артём Качер", 0, 133, 0.61, []float32{1, 0, 0}), 0.61, 0.47, 0.65, 0.07, 0.40, 0.00, 0.13, 0.25, 0.00)
	soft.ZeroCrossingRate = 0.087
	soft.SpectralCentroid = 185
	soft.RMS = 0.438
	soft.Loudness = -33.69
	soft.Electronicness = 0.25
	if got := explainEmotionTransition(ctx, hard, soft); strings.Contains(got, "близкий эмоциональный вайб") {
		t.Fatalf("opposite-feel tracks must not be close vibe: %q", got)
	}
}

func TestBuildEnergyCurveByMode(t *testing.T) {
	seed := withMood(testTrack("seed", "Seed", 0, 120, 0.30, []float32{1, 0, 0}), 0.30, 0.3, 0.7, 0.2, 0.7, 0.2, 0.7, 0.1, 0.1)
	stable := buildEnergyCurve(seed, TrajectoryContinueMood, 6)
	warm := buildEnergyCurve(seed, TrajectoryWarmUp, 6)
	intense := buildEnergyCurve(seed, TrajectoryIntensify, 6)
	cool := buildEnergyCurve(seed, TrajectoryCoolDown, 6)
	deepen := buildEnergyCurve(seed, TrajectoryDeepen, 6)
	explore := buildEnergyCurve(seed, TrajectoryExplore, 6)

	if warm[len(warm)-1] <= warm[0] {
		t.Fatalf("expected warm_up to increase energy: %+v", warm)
	}
	if intense[len(intense)-1] <= warm[len(warm)-1] {
		t.Fatalf("expected intensify to be stronger than warm_up: warm=%+v intensify=%+v", warm, intense)
	}
	if cool[len(cool)-1] >= cool[0] {
		t.Fatalf("expected cool_down to decrease energy: %+v", cool)
	}
	if deepen[len(deepen)-1] >= deepen[0] {
		t.Fatalf("expected deepen to lower activation while increasing emotional depth: %+v", deepen)
	}
	if math.Abs(stable[len(stable)-1]-stable[0]) > 0.05 {
		t.Fatalf("expected stable to stay near seed energy: %+v", stable)
	}
	if explore[1] == explore[2] || explore[2] == explore[3] {
		t.Fatalf("expected explore to alternate around the seed: %+v", explore)
	}
}

func TestEmotionTargetProgramsRemainSemanticallyDistinct(t *testing.T) {
	seed := emotion.Basis{
		Joy:        0.50,
		Melancholy: 0.40,
		Serenity:   0.45,
		Combat:     0.30,
		Pressure:   0.35,
		Roughness:  0.30,
		Swagger:    0.35,
		Brightness: 0.45,
		Motion:     0.45,
	}

	warm := emotionTargetFromMode(seed, TrajectoryWarmUp, 10, 10)
	intense := emotionTargetFromMode(seed, TrajectoryIntensify, 10, 10)
	cool := emotionTargetFromMode(seed, TrajectoryCoolDown, 10, 10)
	deep := emotionTargetFromMode(seed, TrajectoryDeepen, 10, 10)
	explore := emotionTargetFromMode(seed, TrajectoryExplore, 10, 10)
	stable := emotionTargetFromMode(seed, TrajectoryContinueMood, 10, 10)

	if warm.Motion <= seed.Motion || warm.Joy <= seed.Joy {
		t.Fatalf("warm_up target does not warm up: %+v", warm)
	}
	if intense.Pressure <= warm.Pressure || intense.Combat <= seed.Combat {
		t.Fatalf("intensify target must add more pressure/combat than warm_up: warm=%+v intense=%+v", warm, intense)
	}
	if cool.Serenity <= seed.Serenity || cool.Pressure >= seed.Pressure || cool.Combat >= seed.Combat {
		t.Fatalf("cool_down target is not calmer: %+v", cool)
	}
	if deep.Melancholy <= seed.Melancholy || deep.Serenity <= seed.Serenity || deep.Joy >= seed.Joy {
		t.Fatalf("deepen target is not deeper: %+v", deep)
	}
	if explore.Swagger <= seed.Swagger || explore.Brightness <= seed.Brightness {
		t.Fatalf("explore target does not open adjacent colour: %+v", explore)
	}
	if stable != seed {
		t.Fatalf("stable target changed seed: got=%+v want=%+v", stable, seed)
	}
}

func TestNormalizeRayModeKeepsSixUserModesDistinct(t *testing.T) {
	tests := []struct {
		requested string
		want      RayTrajectoryMode
	}{
		{requested: "stable", want: TrajectoryContinueMood},
		{requested: "warm_up", want: TrajectoryWarmUp},
		{requested: "cool_down", want: TrajectoryCoolDown},
		{requested: "intensify", want: TrajectoryIntensify},
		{requested: "deepen", want: TrajectoryDeepen},
		{requested: "explore", want: TrajectoryExplore},
	}

	for _, test := range tests {
		t.Run(test.requested, func(t *testing.T) {
			if got := normalizeRayMode(test.requested, TrajectoryExplore); got != test.want {
				t.Fatalf("normalizeRayMode(%q)=%q want %q", test.requested, got, test.want)
			}
		})
	}
}

func TestTransitionFeedbackKeyChangesForHardJump(t *testing.T) {
	prev := withMood(testTrack("prev", "Artist A", 0, 110, 0.25, []float32{1, 0, 0}), 0.22, 0.20, 0.82, 0.05, 0.80, 0.10, 0.82, 0.08, 0.10)
	next := withMood(testTrack("next", "Artist B", 0, 130, 0.88, []float32{0.98, 0.02, 0}), 0.90, 0.94, 0.05, 0.88, 0.10, 0.92, 0.08, 0.94, 0.86)
	key := TransitionRewardKey(prev, next)
	if key == "" || key[:15] != "transition:jump" {
		t.Fatalf("expected jump transition key, got %q", key)
	}
}

func TestRawSensoryDistanceSeparatesHardRockFromSoftPop(t *testing.T) {
	hard := withMood(testTrack("hard", "Rob Zombie", 0, 115, 0.49, []float32{1, 0, 0}), 0.49, 0.57, 0.70, 0.00, 0.21, 0.00, 0.14, 0.46, 0.00)
	hard.ZeroCrossingRate = 0.184
	hard.SpectralCentroid = 3040
	hard.RMS = 0.360
	hard.Loudness = -38.42
	hard.Electronicness = 0.46
	soft := withMood(testTrack("soft", "Артём Качер", 0, 133, 0.61, []float32{1, 0, 0}), 0.61, 0.47, 0.65, 0.07, 0.40, 0.00, 0.13, 0.25, 0.00)
	soft.ZeroCrossingRate = 0.087
	soft.SpectralCentroid = 185
	soft.RMS = 0.438
	soft.Loudness = -33.69
	soft.Electronicness = 0.25
	if got := rawSensoryDistance(hard, soft); got < 0.26 {
		t.Fatalf("hard rock vs soft pop should not be near transition, raw sensory distance %.3f", got)
	}
	ctx := rankingContext{Normalizer: FeatureNormalizer{}}
	if got := explainEmotionTransition(ctx, hard, soft); strings.Contains(got, "близкий эмоциональный вайб") {
		t.Fatalf("opposite-feel tracks must not be close vibe: %q", got)
	}
}

func TestTransitionFeedbackScoreRewardsKnownGoodTransition(t *testing.T) {
	prev := withMood(testTrack("prev", "Artist A", 0, 118, 0.35, []float32{1, 0, 0}), 0.35, 0.32, 0.70, 0.12, 0.72, 0.14, 0.60, 0.18, 0.20)
	next := withMood(testTrack("next", "Artist B", 0, 120, 0.42, []float32{0.96, 0.04, 0}), 0.44, 0.36, 0.62, 0.16, 0.68, 0.20, 0.55, 0.26, 0.24)
	stats := map[string]strategyStat{TransitionRewardKey(prev, next): {plays: 4, reward: 2.0}}
	if transitionFeedbackScore(stats, prev, next) <= 0 {
		t.Fatal("expected positive transition feedback score")
	}
}

func TestInferRayModeDeepen(t *testing.T) {
	seed := withMood(testTrack("seed", "Seed", 0, 102, 0.42, []float32{1, 0, 0}), 0.46, 0.18, 0.68, 0.10, 0.70, 0.10, 0.40, 0.50, 0.12)
	mode := inferRayMode(seed, []library.Track{seed}, nil, FeatureNormalizer{})
	if mode != TrajectoryDeepen {
		t.Fatalf("expected deepen, got %s", mode)
	}
}

func TestMoodDistanceUsesApproachabilityAndEngagement(t *testing.T) {
	base := withMood(testTrack("a", "Artist A", 0, 110, 0.40, []float32{1, 0, 0}), 0.70, 0.60, 0.30, 0.20, 0.60, 0.65, 0.55, 0.35, 0.20)
	near := withMood(testTrack("b", "Artist B", 0, 112, 0.42, []float32{0.98, 0.02, 0}), 0.72, 0.58, 0.28, 0.22, 0.58, 0.63, 0.52, 0.34, 0.22)
	far := withMood(testTrack("c", "Artist C", 0, 112, 0.42, []float32{0.98, 0.02, 0}), 0.72, 0.58, 0.28, 0.22, 0.58, 0.63, 0.52, 0.34, 0.22)
	a := calibrateMood(withExtendedMood(base, 0.65, 0.55, 0.20, 0.40, 0.45, 0.60, 0.85, 0.82, 0.18))
	b := calibrateMood(withExtendedMood(near, 0.63, 0.52, 0.22, 0.38, 0.42, 0.58, 0.82, 0.78, 0.22))
	c := calibrateMood(withExtendedMood(far, 0.63, 0.52, 0.22, 0.38, 0.42, 0.15, 0.20, 0.18, 0.85))
	if moodDistance(a, c) <= moodDistance(a, b) {
		t.Fatal("expected distant approachability/engagement to increase mood distance")
	}
}

func TestBuildRayWithModeIncludesInsight(t *testing.T) {
	svc := &Service{}
	seed := testTrack("seed", "Seed", 0, 120, 0.50, []float32{1, 0, 0})
	tracks := []library.Track{seed, testTrack("a", "A", 0, 121, 0.52, []float32{0.99, 0.01, 0}), testTrack("b", "B", 1, 118, 0.48, []float32{0.92, 0.08, 0})}
	queue := svc.BuildRayWithMode(seed, tracks, "", "deepen")
	if len(queue) < 2 {
		t.Fatalf("expected queue, got %d", len(queue))
	}
	if queue[1].Insight.Mode != "deepen" {
		t.Fatalf("expected insight mode deepen, got %+v", queue[1].Insight)
	}
}

func TestAuditRayReturnsRows(t *testing.T) {
	svc := &Service{}
	seed := testTrack("seed", "Seed", 0, 120, 0.50, []float32{1, 0, 0})
	tracks := []library.Track{seed, testTrack("a", "A", 0, 121, 0.52, []float32{0.99, 0.01, 0}), testTrack("b", "B", 1, 118, 0.48, []float32{0.92, 0.08, 0})}
	audit := svc.AuditRay(seed, tracks, "explore", 5)
	if len(audit.Rows) == 0 {
		t.Fatal("expected audit rows")
	}
	if audit.Mode == "" {
		t.Fatal("expected audit mode")
	}
}

func TestTempoDistanceHandlesHalfDoubleTempo(t *testing.T) {
	if tempoDistance(80, 160) >= tempoDistance(80, 121) {
		t.Fatal("expected half/double tempo compatibility to be treated as closer")
	}
}

func TestTempoCompatibilityUsesConfidenceAndStability(t *testing.T) {
	a := testTrack("a", "A", 0, 80, 0.5, []float32{1, 0})
	b := testTrack("b", "B", 0, 160, 0.5, []float32{0.9, 0.1})
	a.BPMPerceived, b.BPMPerceived = 80, 160
	a.TempoConfidence, b.TempoConfidence = 0.9, 0.85
	a.TempoStability, b.TempoStability = 0.8, 0.75
	if tempoCompatibility(a, b) < 0.7 {
		t.Fatalf("expected strong compatibility, got %f", tempoCompatibility(a, b))
	}
	b.TempoConfidence = 0.2
	if tempoCompatibility(a, b) != 0.5 {
		t.Fatalf("expected neutral fallback for low confidence, got %f", tempoCompatibility(a, b))
	}
}

func TestBuildRayUsesSeedBucketForCurrentItem(t *testing.T) {
	svc := &Service{}
	seed := testTrack("seed", "Seed Artist", 0, 120, 0.7, []float32{1, 0, 0})
	tracks := []library.Track{seed, testTrack("a", "Artist A", 0, 121, 0.69, []float32{0.99, 0.01, 0}), testTrack("b", "Artist B", 1, 118, 0.72, []float32{0.88, 0.12, 0})}
	queue := svc.BuildRay(seed, tracks, "")
	if len(queue) == 0 {
		t.Fatal("expected queue")
	}
	if queue[0].Bucket != "seed" || queue[0].Strategy != "seed" {
		t.Fatalf("expected seed bucket/strategy on first item, got %+v", queue[0])
	}
}

func TestConfidenceCappedWhenWarningsExist(t *testing.T) {
	tk := library.Track{
		Energy:          0.9,
		Danceability:    0.9,
		Sad:             1,
		Tonality:        1,
		Approachability: 1,
		Engagement:      1,
		TempoSource:     "tempocnn",
		TempoConfidence: 1,
		TempoStability:  1,
		AnalyzedLevel:   2,
		AnalysisStatus:  "done",
		Embedding:       []float32{0.1, 0.2},
	}
	conf := insightConfidence(tk)
	w := insightWarning(rays.QueueInsight{Confidence: conf, TempoUnknown: false, Novelty: 1, JumpPenalty: 0.2, MoodDistance: 0.1}, tk)
	capped := capConfidenceByWarning(conf, w)
	if capped >= 0.9 {
		t.Fatalf("confidence should be capped for saturated heads: conf=%.3f warning=%s capped=%.3f", conf, w, capped)
	}
}

func idNum(prefix string, n int) string {
	return prefix + string(rune('a'+n))
}

func testTrack(id, artist string, cluster int, tempo float64, energy float64, embedding []float32) library.Track {
	return library.Track{
		ID:               id,
		Title:            id,
		Artist:           artist,
		DurationLabel:    "3:00",
		ClusterID:        cluster,
		Tempo:            tempo,
		BPMPerceived:     tempo,
		TempoConfidence:  0.8,
		TempoStability:   0.8,
		TempoSource:      "tempocnn",
		Energy:           energy,
		Danceability:     0.6,
		Valence:          0.6,
		GenrePrimary:     "electronic",
		Acousticness:     0.3,
		Electronicness:   0.7,
		Instrumentalness: 0.2,
		Vocalness:        0.8,
		Happy:            0.5,
		Sad:              0.2,
		Relaxed:          0.4,
		Party:            energy,
		Aggressive:       energy * 0.35,
		Embedding:        embedding,
		AnalyzedLevel:    2,
		PlayCount:        0,
	}
}

func withMood(t library.Track, dance, valence, acoustic, electronic, vocal, happy, relaxed, party, aggressive float64) library.Track {
	t.Danceability = dance
	t.Valence = valence
	t.Acousticness = acoustic
	t.Electronicness = electronic
	t.Vocalness = vocal
	t.Instrumentalness = 1 - vocal
	t.Happy = happy
	t.Sad = 1 - valence
	t.Relaxed = relaxed
	t.Party = party
	t.Aggressive = aggressive
	t.TimbreBrightness = happy
	t.Tonality = dance
	t.Approachability = 0.55
	t.Engagement = party
	t.Melodicness = dance
	t.Softness = relaxed
	t.Heaviness = aggressive
	t.Dreaminess = (relaxed + valence) * 0.5
	t.Emotionality = (happy + t.Sad) * 0.5
	return t
}

func withExtendedMood(t library.Track, melodic, soft, heavy, dreamy, emotional, timbre, tonality, approachability, engagement float64) library.Track {
	t.Melodicness = melodic
	t.Softness = soft
	t.Heaviness = heavy
	t.Dreaminess = dreamy
	t.Emotionality = emotional
	t.TimbreBrightness = timbre
	t.Tonality = tonality
	t.Approachability = approachability
	t.Engagement = engagement
	return t
}

func TestBridgeAffinityRejectsHardJump(t *testing.T) {
	seed := withMood(testTrack("seed", "Seed", 0, 118, 0.40, []float32{1, 0, 0}), 0.44, 0.32, 0.62, 0.28, 0.72, 0.28, 0.60, 0.34, 0.22)
	bridge := withMood(testTrack("bridge", "Bridge", 1, 120, 0.43, []float32{0.82, 0.18, 0}), 0.46, 0.34, 0.58, 0.30, 0.70, 0.30, 0.58, 0.36, 0.24)
	jump := withMood(testTrack("jump", "Jump", 2, 148, 0.88, []float32{0.96, 0.04, 0}), 0.90, 0.82, 0.08, 0.88, 0.18, 0.84, 0.14, 0.90, 0.86)
	bridgeScore := bridgeAffinity(seed, bridge)
	jumpScore := bridgeAffinity(seed, jump)
	if bridgeScore <= jumpScore {
		t.Fatalf("expected bridge affinity (%f) to exceed hard jump (%f)", bridgeScore, jumpScore)
	}
}

func TestSessionVolatilityIncreasesOnUnstableWindow(t *testing.T) {
	history := []library.Track{
		withMood(testTrack("h1", "H1", 0, 100, 0.3, []float32{1, 0, 0}), 0.3, 0.3, 0.7, 0.2, 0.7, 0.2, 0.7, 0.1, 0.1),
		withMood(testTrack("h2", "H2", 0, 140, 0.8, []float32{0.9, 0.1, 0}), 0.8, 0.8, 0.2, 0.8, 0.2, 0.8, 0.8, 0.8, 0.8),
		withMood(testTrack("h3", "H3", 0, 100, 0.3, []float32{1, 0, 0}), 0.3, 0.3, 0.7, 0.2, 0.7, 0.2, 0.7, 0.1, 0.1),
	}
	next := withMood(testTrack("next", "Next", 0, 120, 0.5, []float32{0.8, 0.2, 0}), 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5)
	volatility := sessionVolatility(history, next)
	if volatility <= 0.3 {
		t.Fatalf("expected high volatility for unstable window, got %f", volatility)
	}
}

func TestRidersToDemonoidIsNotStableMood(t *testing.T) {
	riders := calibrateMood(library.Track{
		Danceability:     0.87,
		Energy:           0.62,
		Valence:          0.52,
		Relaxed:          0.59,
		Party:            0.42,
		Aggressive:       0.02,
		Acousticness:     0.02,
		Electronicness:   0.18,
		Instrumentalness: 0.52,
		Vocalness:        0.48,
		Melodicness:      0.13,
		Softness:         0.12,
		Heaviness:        0.00,
		TimbreBrightness: 0.14,
		BPMPerceived:     115,
		TempoConfidence:  0.71,
		SpectralCentroid: 1607,
		ZeroCrossingRate: 0.122,
	})
	demonoid := calibrateMood(library.Track{
		Danceability:     0.91,
		Energy:           0.53,
		Valence:          0.51,
		Relaxed:          0.73,
		Party:            0.28,
		Aggressive:       0.02,
		Acousticness:     0.05,
		Electronicness:   0.30,
		Instrumentalness: 0.47,
		Vocalness:        0.53,
		Melodicness:      0.02,
		Softness:         0.15,
		Heaviness:        0.01,
		TimbreBrightness: 0.26,
		BPMPerceived:     115,
		TempoConfidence:  1.0,
		SpectralCentroid: 1569,
		ZeroCrossingRate: 0.198,
	})

	dist := moodDistance(riders, demonoid)
	risk := jumpPenaltyFromMood(riders, demonoid)

	if demonoid.Edge-riders.Edge < 0.10 {
		t.Fatalf("expected Demonoid to have meaningfully higher edge: riders=%.3f demonoid=%.3f", riders.Edge, demonoid.Edge)
	}
	if dist < 0.10 {
		t.Fatalf("Riders -> Demonoid mood distance too low: dist=%.3f riders=%+v demonoid=%+v", dist, riders, demonoid)
	}
	if risk < 0.18 {
		t.Fatalf("Riders -> Demonoid jump risk too low: risk=%.3f", risk)
	}
}

func TestPerceptualHardJumpRiskHigherForAggressiveToSerene(t *testing.T) {
	aggr := withMood(testTrack("aggr", "Aggressive", 0, 145, 0.88, []float32{1, 0, 0}), 0.90, 0.82, 0.08, 0.90, 0.12, 0.84, 0.12, 0.88, 0.84)
	serene := withMood(testTrack("serene", "Serene", 0, 92, 0.22, []float32{0.2, 0.8, 0}), 0.26, 0.22, 0.82, 0.12, 0.78, 0.18, 0.86, 0.08, 0.10)
	ctx := rankingContext{Normalizer: FeatureNormalizer{}}
	risk := perceptualHardJumpRisk(ctx, aggr, serene, TrajectoryContinueMood)
	if risk < 0.08 {
		t.Fatalf("expected a noticeable perceptual hard jump, got %f", risk)
	}
}

func TestControlledDiscoveryScoresSmoothCandidateHigher(t *testing.T) {
	seed := withMood(testTrack("seed", "Seed", 0, 120, 0.48, []float32{1, 0, 0}), 0.48, 0.42, 0.55, 0.35, 0.62, 0.40, 0.52, 0.35, 0.30)
	smooth := withMood(testTrack("smooth", "Smooth", 1, 122, 0.50, []float32{0.95, 0.05, 0}), 0.50, 0.44, 0.56, 0.36, 0.60, 0.42, 0.54, 0.36, 0.31)
	jump := withMood(testTrack("jump", "Jump", 2, 160, 0.90, []float32{0.98, 0.02, 0}), 0.88, 0.84, 0.08, 0.90, 0.16, 0.86, 0.10, 0.88, 0.82)
	ctx := rankingContext{Seed: seed, Mode: TrajectoryExplore, Position: 4, QueueLength: 12, Normalizer: FeatureNormalizer{}}
	smoothScore := controlledDiscoveryScore(scored{track: smooth, bucket: bucketDiscovery}, []library.Track{seed}, ctx)
	jumpScore := controlledDiscoveryScore(scored{track: jump, bucket: bucketDiscovery}, []library.Track{seed}, ctx)
	if smoothScore <= jumpScore {
		t.Fatalf("expected smooth discovery to outscore jump: smooth=%f jump=%f", smoothScore, jumpScore)
	}
}

func TestQueueInsightIncludesEmotionBasis(t *testing.T) {
	seed := withMood(testTrack("seed", "Seed", 0, 120, 0.48, []float32{1, 0, 0}), 0.48, 0.42, 0.55, 0.35, 0.62, 0.40, 0.52, 0.35, 0.30)
	prev := withMood(testTrack("prev", "Prev", 0, 118, 0.46, []float32{0.98, 0.02, 0}), 0.46, 0.40, 0.58, 0.34, 0.60, 0.38, 0.50, 0.34, 0.28)
	cand := withMood(testTrack("cand", "Cand", 1, 121, 0.50, []float32{0.92, 0.08, 0}), 0.50, 0.44, 0.54, 0.36, 0.62, 0.42, 0.54, 0.36, 0.31)
	ctx := rankingContext{Seed: seed, Mode: TrajectoryContinueMood, Normalizer: FeatureNormalizer{}}
	ins := queueInsight(ctx, seed, prev, []library.Track{prev}, cand)
	if ins.Emotion.Label == "" {
		t.Fatal("expected emotion label in queue insight")
	}
	if ins.Emotion.Distance <= 0 || ins.Emotion.HardJump <= 0 {
		t.Fatalf("expected positive emotion distance and hard jump, got %+v", ins.Emotion)
	}
	if ins.Emotion.BridgeScore <= 0 {
		t.Fatalf("expected bridge score in queue insight, got %+v", ins.Emotion)
	}
}

func TestEmotionTransitionReasonIsSpecific(t *testing.T) {
	prev := withMood(testTrack("prev2", "Prev2", 0, 118, 0.36, []float32{1, 0, 0}), 0.36, 0.28, 0.68, 0.12, 0.70, 0.20, 0.60, 0.18, 0.24)
	next := withMood(testTrack("next2", "Next2", 0, 122, 0.52, []float32{0.95, 0.05, 0}), 0.54, 0.34, 0.60, 0.18, 0.72, 0.24, 0.58, 0.22, 0.28)
	reason := explainEmotionTransition(rankingContext{Normalizer: FeatureNormalizer{}}, prev, next)
	if reason == "" || reason == "мягкий мост между состояниями" {
		t.Fatalf("expected a specific emotion transition reason, got %q", reason)
	}
}

func TestFamilyTransitionPenaltyCalmToDanger(t *testing.T) {
	pen := familyTransitionPenalty("serene_calm", "combat_force")
	if pen < 0.70 {
		t.Fatalf("calm->danger should have high penalty, got %f", pen)
	}
}

func TestFamilyTransitionPenaltySameFamily(t *testing.T) {
	pen := familyTransitionPenalty("serene_calm", "serene_bright")
	if pen != 0 {
		t.Fatalf("same family should have zero penalty, got %f", pen)
	}
}

func TestFamilyTransitionPenaltyCalmToMixed(t *testing.T) {
	pen := familyTransitionPenalty("serene_calm", "dramatic_arc")
	if pen > 0.35 {
		t.Fatalf("calm->mixed should have moderate penalty, got %f", pen)
	}
}

func TestPerceptualHardJumpRiskUsesFamilyPenalty(t *testing.T) {
	pen := familyTransitionPenalty("joy_party", "combat_force")
	if pen < 0.60 {
		t.Fatalf("joy->combat family penalty should be high, got %f", pen)
	}
	pen2 := familyTransitionPenalty("serene_calm", "serene_bright")
	if pen2 != 0 {
		t.Fatalf("same family penalty should be zero, got %f", pen2)
	}
}

func TestEmotionFamilyLabelMapping(t *testing.T) {
	tests := []struct {
		label  string
		family string
	}{
		{"serene_calm", "calm"},
		{"serene_bright", "calm"},
		{"combat_force", "danger"},
		{"dirty_electro_combat", "danger"},
		{"joy_party", "positive"},
		{"melancholy_calm", "melancholy"},
		{"dramatic_arc", "mixed"},
		{"tense_pressure", "pressure"},
		{"night_smooth", "warm"},
	}
	for _, tt := range tests {
		got := emotionFamily(tt.label)
		if got != tt.family {
			t.Errorf("emotionFamily(%q) = %q, want %q", tt.label, got, tt.family)
		}
	}
}

func TestMergeCandidatePoolsDeduplicatesTracks(t *testing.T) {
	track := library.Track{
		ID:    "same-track",
		Title: "Same Track",
	}

	merged := mergeCandidatePools(
		24,
		[]scored{
			{
				track:  track,
				score:  0.62,
				bucket: bucketAdjacent,
			},
		},
		[]scored{
			{
				track:  track,
				score:  0.74,
				bucket: bucketBridge,
			},
		},
		[]scored{
			{
				track:  track,
				score:  0.68,
				bucket: bucketDiscovery,
			},
		},
	)

	if len(merged) != 1 {
		t.Fatalf(
			"expected one deduplicated candidate, got %d",
			len(merged),
		)
	}
	if merged[0].bucket != bucketBridge {
		t.Fatalf(
			"expected strongest bridge candidate, got %q",
			merged[0].bucket,
		)
	}
}

func TestMergeCandidatePoolsCapsSequentialRanking(t *testing.T) {
	items := make([]scored, 0, 500)
	for i := 0; i < 500; i++ {
		items = append(items, scored{
			track: library.Track{
				ID: fmt.Sprintf("track-%03d", i),
			},
			score: float64(500-i) / 500,
		})
	}

	merged := mergeCandidatePools(20, items)
	want := candidatePoolLimit(20)
	if len(merged) != want {
		t.Fatalf(
			"candidate count = %d, want %d",
			len(merged),
			want,
		)
	}
}

func TestBuildRayWithModeContextCanBeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := &Service{}
	_, err := service.BuildRayWithModeContext(
		ctx,
		library.Track{ID: "seed"},
		[]library.Track{{ID: "seed"}},
		"",
		"",
	)

	if !errors.Is(err, ErrRayBuildCanceled) {
		t.Fatalf(
			"expected ErrRayBuildCanceled, got %v",
			err,
		)
	}
}

func BenchmarkMergeCandidatePools200(b *testing.B) {
	benchmarkMergeCandidatePools(b, 200)
}

func BenchmarkMergeCandidatePools2000(b *testing.B) {
	benchmarkMergeCandidatePools(b, 2000)
}

func benchmarkMergeCandidatePools(
	b *testing.B,
	count int,
) {
	pools := make([][]scored, 5)
	for poolIndex := range pools {
		pools[poolIndex] = make(
			[]scored,
			0,
			count,
		)
		for i := 0; i < count; i++ {
			pools[poolIndex] = append(
				pools[poolIndex],
				scored{
					track: library.Track{
						ID: fmt.Sprintf(
							"track-%d",
							i,
						),
					},
					score: float64(
						count - i + poolIndex,
					),
				},
			)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mergeCandidatePools(24, pools...)
	}
}

func TestWarmUpAndCoolDownPreferOppositeEnergyDirection(
	t *testing.T,
) {
	ctx := rankingContext{
		ContentMode:       rays.ContentWarmUp,
		TargetEnergyDelta: 0.22,
		TargetCalmDelta:   -0.12,
		TargetBrightDelta: 0.08,
	}

	seed := library.Track{
		ID: "seed",
	}
	candidateHigher := library.Track{
		ID: "higher",
	}

	seedResult := emotion.Result{
		Basis: emotion.Basis{
			Joy:        0.50,
			Serenity:   0.40,
			Brightness: 0.30,
		},
	}
	higherResult := emotion.Result{
		Basis: emotion.Basis{
			Joy:        0.72,
			Serenity:   0.28,
			Brightness: 0.38,
		},
	}
	lowerResult := emotion.Result{
		Basis: emotion.Basis{
			Joy:        0.28,
			Serenity:   0.60,
			Brightness: 0.22,
		},
	}

	cache := &emotionCache{m: map[string]emotion.Result{}}
	cache.m[seed.ID] = seedResult
	cache.m[candidateHigher.ID] = higherResult
	ctx.EmotionCache = cache

	scoreHigher := trajectoryModeScore(ctx, seed, candidateHigher)

	cache.m[candidateHigher.ID] = lowerResult
	scoreLower := trajectoryModeScore(ctx, seed, candidateHigher)

	if scoreHigher <= scoreLower {
		t.Fatalf(
			"warm_up: higher-energy score %.3f should exceed lower-energy %.3f",
			scoreHigher,
			scoreLower,
		)
	}
}
