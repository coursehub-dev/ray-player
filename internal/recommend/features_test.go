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

func TestNormalizeTrackFeaturesKeepsCalibratedSemanticProbabilities(t *testing.T) {
	tracks := []library.Track{
		{Energy: 0.1, Danceability: 0.2, Valence: 0.3, AnalyzedLevel: 2},
		{Energy: 0.5, Danceability: 0.6, Valence: 0.7, AnalyzedLevel: 2},
		{Energy: 0.9, Danceability: 1.0, Valence: 0.8, AnalyzedLevel: 2},
	}
	normalizer := BuildFeatureNormalizer(tracks)
	track := library.Track{Energy: 0.7, Danceability: 0.8, Valence: 0.9, AnalyzedLevel: 2}
	normalized := normalizeTrackFeatures(track, normalizer)
	if math.Abs(normalized.Energy-0.7) > 1e-9 {
		t.Fatalf("energy changed from calibrated value: got %.6f", normalized.Energy)
	}
	if math.Abs(normalized.Danceability-0.8) > 1e-9 {
		t.Fatalf("danceability changed from calibrated value: got %.6f", normalized.Danceability)
	}
	if math.Abs(normalized.Valence-0.9) > 1e-9 {
		t.Fatalf("valence changed from calibrated value: got %.6f", normalized.Valence)
	}
}

func TestSemanticNormalizationIsLibraryInvariant(t *testing.T) {
	base := []library.Track{{Energy: 0.2}, {Energy: 0.8}}
	expanded := append([]library.Track{}, base...)
	for i := 0; i < 50; i++ {
		expanded = append(expanded, library.Track{Energy: float64(i) / 49.0})
	}
	baseNorm := BuildFeatureNormalizer(base).NormWeighted("Energy", 0.73)
	expandedNorm := BuildFeatureNormalizer(expanded).NormWeighted("Energy", 0.73)
	if math.Abs(baseNorm-expandedNorm) > 1e-9 {
		t.Fatalf("semantic value depends on library composition: base=%.6f expanded=%.6f", baseNorm, expandedNorm)
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

func TestSaturatedEvidenceFeatureDoesNotBecomeFakeEvidence(t *testing.T) {
	tracks := make([]library.Track, 30)
	for i := range tracks {
		tracks[i].Sad = 0
	}

	n := BuildFeatureNormalizer(tracks)
	got := n.NormWeighted("Sad", 0)
	if math.Abs(got) > 0.001 {
		t.Fatalf("absent sadness became synthetic evidence: got %.3f", got)
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

func TestWeakAuxiliaryEvidenceIsReducedTowardAbsence(t *testing.T) {
	n := BuildFeatureNormalizer(nil)
	got := n.NormWeighted("Dreaminess", 0.10)
	if got >= 0.10 || got <= 0 {
		t.Fatalf("weak auxiliary evidence should be reduced but remain positive, got %.3f", got)
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
