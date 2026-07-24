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
	restored := fromRow(db.TrackRow{
		ID:                   row.ID,
		Title:                row.Title,
		Artist:               row.Artist,
		Embedding:            row.Embedding,
		TextEmbedding:        row.TextEmbedding,
		AnalysisVersion:      currentAnalysisVersion,
		EssentiaModelVersion: currentEssentiaModelVersion,
	})
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

func TestSanitizeMetadataGenreRejectsDownloadSiteTags(t *testing.T) {
	for _, value := range []string{
		"lmusic.kz",
		"www.lightaudio.ru",
		"https://example.com/music",
		"unknown",
		"n/a",
	} {
		if got := sanitizeMetadataGenre(value); got != "" {
			t.Fatalf("sanitizeMetadataGenre(%q)=%q want empty", value, got)
		}
	}
	if got := sanitizeMetadataGenre("Alternative Rock"); got != "Alternative Rock" {
		t.Fatalf("valid genre changed: %q", got)
	}
}

func TestChooseGenreRequiresStrongMLConfidence(t *testing.T) {
	if got := chooseGenre("Hip-Hop", "Rock", 0.11, 0.10); got != "Hip-Hop" {
		t.Fatalf("weak ML should fall back to metadata, got %q", got)
	}
	if got := chooseGenre("Hip-Hop", "Rock", 0.24, 0.08); got != "Rock" {
		t.Fatalf("strong ML should win, got %q", got)
	}
	if got := chooseGenre("www.lightaudio.ru", "Rock", 0.11, 0.10); got != "Unknown" {
		t.Fatalf("invalid metadata plus weak ML should be Unknown, got %q", got)
	}
}

func TestTrackFromRowSuppressesStaleMLStateUntilReanalysis(t *testing.T) {
	restored := fromRow(db.TrackRow{
		ID:                   "stale",
		Genre:                "Hip-Hop",
		GenrePrimary:         "Rock",
		GenreDetail:          "Rock / Black Metal",
		GenreLabel:           "Rock",
		Happy:                0.9,
		Relaxed:              0.9,
		Aggressive:           0.9,
		Approachability:      1,
		Engagement:           1,
		TempoConfidence:      1,
		TempoStability:       1,
		Embedding:            make([]float32, modelcontract.DiscogsEmbeddingSize),
		AnalysisVersion:      currentAnalysisVersion - 1,
		EssentiaModelVersion: "old-contract",
	})
	if len(restored.Embedding) != 0 {
		t.Fatal("stale semantic embedding must not be used before reanalysis")
	}
	if restored.GenrePrimary != "" || restored.GenreLabel != "Hip-Hop" {
		t.Fatalf("stale ML genre leaked into UI: primary=%q label=%q", restored.GenrePrimary, restored.GenreLabel)
	}
	if restored.Happy != 0 || restored.Relaxed != 0 || restored.Aggressive != 0 {
		t.Fatalf("stale mood heads leaked into EmoFlow: %+v", restored)
	}
	if restored.Approachability != 0.5 || restored.Engagement != 0.5 {
		t.Fatalf("stale regression heads must fall back to neutral values: %+v", restored)
	}
	if restored.TempoConfidence != 0 || restored.TempoStability != 0 {
		t.Fatalf("stale tempo confidence must be reset: %+v", restored)
	}
}
