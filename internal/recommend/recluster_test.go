package recommend

import (
	"testing"

	"ray-player1/internal/library"
	"ray-player1/internal/modelcontract"
)

func TestInitialClusterCentroidsAreOrderIndependent(t *testing.T) {
	tracks := []library.Track{
		{ID: "c", Embedding: clusterTestEmbedding(0, 1)},
		{ID: "a", Embedding: clusterTestEmbedding(1, 0)},
		{ID: "b", Embedding: clusterTestEmbedding(-1, 0)},
		{ID: "d", Embedding: clusterTestEmbedding(0, -1)},
	}
	reversed := []library.Track{tracks[3], tracks[2], tracks[1], tracks[0]}

	got := initialClusterCentroids(tracks, 3)
	want := initialClusterCentroids(reversed, 3)
	if len(got) != len(want) {
		t.Fatalf("centroid count differs: got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		if vectorSim(got[i], want[i]) < 0.999999 {
			t.Fatalf("centroid %d depends on track order", i)
		}
	}
}

func TestInitialClusterCentroidsSpreadSeeds(t *testing.T) {
	tracks := []library.Track{
		{ID: "a", Embedding: clusterTestEmbedding(1, 0)},
		{ID: "b", Embedding: clusterTestEmbedding(0.99, 0.01)},
		{ID: "c", Embedding: clusterTestEmbedding(0, 1)},
	}
	centroids := initialClusterCentroids(tracks, 2)
	if len(centroids) != 2 {
		t.Fatalf("expected two centroids, got %d", len(centroids))
	}
	if sim := vectorSim(centroids[0], centroids[1]); sim > 0.2 {
		t.Fatalf("expected farthest-first seeds, similarity=%.4f", sim)
	}
}

func clusterTestEmbedding(x, y float32) []float32 {
	out := make([]float32, modelcontract.DiscogsEmbeddingSize)
	out[0] = x
	out[1] = y
	return out
}
