package recommend

import (
	"math"
	"testing"

	"ray-player1/internal/library"
)

func TestBuildFeatureNormalizerHandlesEmptySlice(t *testing.T) {
	tracks := []library.Track{}
	normalizer := BuildFeatureNormalizer(tracks)
	if len(normalizer.Stats) != 0 {
		t.Fatalf("expected empty stats, got %d", len(normalizer.Stats))
	}
}

func TestBuildFeatureNormalizerComputesStats(t *testing.T) {
	tracks := []library.Track{
		{Energy: 0.5, Danceability: 0.6, Valence: 0.7, AnalyzedLevel: 2},
		{Energy: 0.6, Danceability: 0.7, Valence: 0.8, AnalyzedLevel: 2},
		{Energy: 0.7, Danceability: 0.8, Valence: 0.9, AnalyzedLevel: 2},
	}
	normalizer := BuildFeatureNormalizer(tracks)
	energyStats, ok := normalizer.Stats["Energy"]
	if !ok {
		t.Fatal("expected Energy stats to exist")
	}
	if energyStats.Count != 3 {
		t.Fatalf("expected count 3, got %d", energyStats.Count)
	}
	if energyStats.Mean != 0.6 {
		t.Fatalf("expected energy mean 0.6, got %f", energyStats.Mean)
	}
}

func TestNormalizeTrackFeaturesAppliesPercentile(t *testing.T) {
	tracks := []library.Track{
		{Energy: 0.5, Danceability: 0.6, Valence: 0.7, AnalyzedLevel: 2},
		{Energy: 0.6, Danceability: 0.7, Valence: 0.8, AnalyzedLevel: 2},
		{Energy: 0.7, Danceability: 0.8, Valence: 0.9, AnalyzedLevel: 2},
	}
	normalizer := BuildFeatureNormalizer(tracks)
	track := library.Track{Energy: 0.7, Danceability: 0.8, Valence: 0.9, AnalyzedLevel: 2}
	normalized := normalizeTrackFeatures(track, normalizer)
	// With only 3 tracks, reliability is low and the value is only partially moved.
	if normalized.Energy <= 0.5 || normalized.Energy >= 0.75 {
		t.Fatalf("expected partially damped energy in (0.5,0.75), got %f", normalized.Energy)
	}
}

func TestFeatureValueHandlesMissingFields(t *testing.T) {
	track := library.Track{Energy: 0.5}
	val, ok := featureValue(track, "Energy")
	if !ok {
		t.Fatal("expected true for existing field")
	}
	if val != 0.5 {
		t.Fatalf("expected 0.5 for existing field, got %f", val)
	}
}

func TestIsUsableFeatureFiltersUnreliable(t *testing.T) {
	if isUsableFeature("Danceability", -0.5) {
		t.Fatal("expected false for negative value")
	}
	if isUsableFeature("Danceability", 1.5) {
		t.Fatal("expected false for value > 1")
	}
	if !isUsableFeature("Danceability", 0.5) {
		t.Fatal("expected true for valid value")
	}
	if !isUsableFeature("Loudness", -42) {
		t.Fatal("expected loudness range to accept negative audio values")
	}
}

func TestNormalizerSaturatedFeatureBecomesNeutral(t *testing.T) {
	tracks := make([]library.Track, 30)
	for i := range tracks {
		tracks[i].Sad = 0
		tracks[i].Danceability = float64(i) / 29.0
	}

	n := BuildFeatureNormalizer(tracks)
	if n.Reliability("Sad") != 0 {
		t.Fatalf("sad reliability should be zero for saturated feature, got %.3f", n.Reliability("Sad"))
	}
	got := n.NormWeighted("Sad", 0)
	if math.Abs(got-0.5) > 0.001 {
		t.Fatalf("saturated feature should normalize to neutral 0.5, got %.3f", got)
	}
	if n.Reliability("Danceability") <= 0.5 {
		t.Fatalf("danceability should have useful reliability, got %.3f", n.Reliability("Danceability"))
	}
}

func TestNormalizerDoesNotInvalidateIndividualZeroOne(t *testing.T) {
	tracks := make([]library.Track, 100)
	for i := range tracks {
		tracks[i].Party = float64(i) / 99.0
	}

	n := BuildFeatureNormalizer(tracks)
	if n.Reliability("Party") <= 0.5 {
		t.Fatalf("party should be reliable with broad spread, got %.3f", n.Reliability("Party"))
	}
	low := n.NormWeighted("Party", 0)
	high := n.NormWeighted("Party", 1)
	if low > 0.15 {
		t.Fatalf("individual 0 should remain low when feature is reliable, got %.3f", low)
	}
	if high < 0.85 {
		t.Fatalf("individual 1 should remain high when feature is reliable, got %.3f", high)
	}
}

func TestNormalizerNoSpreadReturnsNeutral(t *testing.T) {
	tracks := make([]library.Track, 30)
	for i := range tracks {
		tracks[i].Aggressive = 0.001
	}

	n := BuildFeatureNormalizer(tracks)
	got := n.Norm("Aggressive", 0.001)
	if math.Abs(got-0.5) > 0.001 {
		t.Fatalf("no-spread feature should return neutral, got %.3f", got)
	}
}

func TestRawAudioFeaturesKeepNaturalScale(t *testing.T) {
	tracks := make([]library.Track, 40)
	for i := range tracks {
		tracks[i].SpectralCentroid = 700 + float64(i)*80
		tracks[i].Loudness = -44 + float64(i)*0.35
		tracks[i].ZeroCrossingRate = 0.06 + float64(i)*0.003
		tracks[i].RMS = 0.20 + float64(i)*0.006
	}
	n := BuildFeatureNormalizer(tracks)
	if n.Reliability("SpectralCentroid") <= 0 {
		t.Fatalf("raw centroid should not be invalidated: %+v", n.Stats["SpectralCentroid"])
	}
	if n.Reliability("Loudness") <= 0 {
		t.Fatalf("negative loudness should not be invalidated: %+v", n.Stats["Loudness"])
	}
	if got := n.NormWeighted("SpectralCentroid", 3000); got <= 0.50 {
		t.Fatalf("high centroid should remain above neutral, got %.3f", got)
	}
}
