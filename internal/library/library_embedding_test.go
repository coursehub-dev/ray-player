package library

import (
	"testing"

	"ray-player1/internal/db"
	"ray-player1/internal/modelcontract"
)

func TestToRowFromRowPreservesSeparateEmbeddings(t *testing.T) {
	validEmb := make([]float32, modelcontract.DiscogsEmbeddingSize)
	for i := range validEmb {
		validEmb[i] = float32(i)
	}
	track := Track{
		ID:            "t1",
		Title:         "Song",
		Artist:        "Artist",
		Embedding:     validEmb,
		TextEmbedding: []float32{4, 5},
	}
	row, err := toRow(track)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.Embedding) != modelcontract.DiscogsEmbeddingSize || len(row.TextEmbedding) != 2 {
		t.Fatalf("unexpected row embeddings: %#v", row)
	}
	restored := fromRow(db.TrackRow{ID: row.ID, Title: row.Title, Artist: row.Artist, Embedding: row.Embedding, TextEmbedding: row.TextEmbedding})
	if len(restored.Embedding) != modelcontract.DiscogsEmbeddingSize || len(restored.TextEmbedding) != 2 {
		t.Fatalf("unexpected restored embeddings: %#v", restored)
	}
}

func TestTrackFromRowFiltersInvalidEmbedding(t *testing.T) {
	restored := fromRow(db.TrackRow{ID: "t2", Title: "Song2", Embedding: []float32{1, 2, 3}})
	if len(restored.Embedding) != 0 {
		t.Fatalf("expected nil embedding for invalid size, got len=%d", len(restored.Embedding))
	}
}
