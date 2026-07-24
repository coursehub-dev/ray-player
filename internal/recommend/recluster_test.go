package recommend

import (
	"fmt"
	"testing"

	"ray-player1/internal/library"
	"ray-player1/internal/modelcontract"
)

func TestInitialEmotionClusterCentroidsAreOrderIndependent(t *testing.T) {
	tracks := []library.Track{
		emotionClusterTestTrack("c", "joy"),
		emotionClusterTestTrack("a", "combat"),
		emotionClusterTestTrack("b", "grief"),
		emotionClusterTestTrack("d", "calm"),
	}
	reversed := []library.Track{tracks[3], tracks[2], tracks[1], tracks[0]}

	got := initialEmotionClusterCentroids(tracks, 3, BuildFeatureNormalizer(tracks))
	want := initialEmotionClusterCentroids(reversed, 3, BuildFeatureNormalizer(reversed))
	if len(got) != len(want) {
		t.Fatalf("centroid count differs: got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		if emotionVectorDistance(got[i], want[i]) > 1e-9 {
			t.Fatalf("centroid %d depends on track order", i)
		}
	}
}

func TestReclusterUsesEmotionInsteadOfEmbeddingOnly(t *testing.T) {
	tracks := make([]library.Track, 0, 9)
	for _, mood := range []string{"joy", "combat", "grief"} {
		for i := 0; i < 3; i++ {
			track := emotionClusterTestTrack(fmt.Sprintf("%s-%d", mood, i), mood)
			track.Embedding = clusterTestEmbedding(1, 0)
			tracks = append(tracks, track)
		}
	}

	service := &Service{}
	service.Recluster(tracks)

	clusters := map[string]int{}
	for _, track := range tracks {
		mood := track.ID[:len(track.ID)-2]
		if existing, ok := clusters[mood]; ok && existing != track.ClusterID {
			t.Fatalf("same emotion split across clusters: mood=%s first=%d got=%d", mood, existing, track.ClusterID)
		}
		clusters[mood] = track.ClusterID
	}
	if len(clusters) != 3 {
		t.Fatalf("moods=%d want=3", len(clusters))
	}
	seen := map[int]bool{}
	for _, clusterID := range clusters {
		seen[clusterID] = true
	}
	if len(seen) != 3 {
		t.Fatalf("opposite emotions collapsed despite identical embeddings: clusters=%v", clusters)
	}
}

func emotionClusterTestTrack(id, mood string) library.Track {
	track := library.Track{
		ID:                id,
		Embedding:         clusterTestEmbedding(1, 0),
		AnalyzedLevel:     2,
		AnalysisStatus:    string(library.AnalysisDone),
		Loudness:          -24,
		RMS:               0.32,
		ZeroCrossingRate:  0.08,
		SpectralCentroid:  1800,
		SpectralFlatness:  0.03,
		SpectralRolloff85: 3600,
		SpectralFlux:      0.18,
		OnsetRate:         1.6,
		DynamicRange:      0.35,
		LowBandRatio:      0.40,
		MidBandRatio:      0.40,
		HighBandRatio:     0.20,
		Tempo:             110,
		BPMPerceived:      110,
		TempoConfidence:   0.80,
		TempoStability:    0.80,
		Tonality:          0.75,
		Approachability:   0.70,
		Engagement:        0.75,
	}
	switch mood {
	case "joy":
		track.Energy = 0.82
		track.Danceability = 0.90
		track.Valence = 0.88
		track.Happy = 0.90
		track.Sad = 0.08
		track.Relaxed = 0.45
		track.Party = 0.82
		track.Aggressive = 0.04
		track.TimbreBrightness = 0.78
	case "combat":
		track.Energy = 0.92
		track.Danceability = 0.66
		track.Valence = 0.18
		track.Happy = 0.08
		track.Sad = 0.18
		track.Relaxed = 0.08
		track.Party = 0.28
		track.Aggressive = 0.96
		track.TimbreBrightness = 0.42
		track.ZeroCrossingRate = 0.18
		track.SpectralFlatness = 0.12
		track.SpectralFlux = 0.42
		track.OnsetRate = 3.8
	case "grief":
		track.Energy = 0.18
		track.Danceability = 0.16
		track.Valence = 0.10
		track.Happy = 0.04
		track.Sad = 0.94
		track.Relaxed = 0.86
		track.Party = 0.02
		track.Aggressive = 0.02
		track.TimbreBrightness = 0.22
		track.Loudness = -38
		track.RMS = 0.12
		track.SpectralCentroid = 900
		track.SpectralFlux = 0.05
		track.OnsetRate = 0.4
	case "calm":
		track.Energy = 0.24
		track.Danceability = 0.34
		track.Valence = 0.72
		track.Happy = 0.62
		track.Sad = 0.12
		track.Relaxed = 0.94
		track.Party = 0.14
		track.Aggressive = 0.01
		track.TimbreBrightness = 0.58
	}
	return track
}

func clusterTestEmbedding(x, y float32) []float32 {
	out := make([]float32, modelcontract.DiscogsEmbeddingSize)
	out[0] = x
	out[1] = y
	return out
}
