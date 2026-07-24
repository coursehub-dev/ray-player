package db

import (
	"path/filepath"
	"testing"
)

func TestMigrateInvalidEmbeddingResetsAnalysis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.db")
	store, err := OpenAtPath(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	row := TrackRow{
		ID:              "legacy",
		Path:            "/music/legacy.mp3",
		Title:           "Legacy",
		Genre:           "Rock",
		GenrePrimary:    "Rock",
		GenreDetail:     "Rock / Black Metal",
		GenreTagsJSON:   `[{"label":"Rock","detail":"Rock / Black Metal","score":0.1}]`,
		GenreLabel:      "Rock",
		AnalysisStatus:  "done",
		AnalysisVersion: 13,
		Embedding:       []float32{1, 2, 3, 4},
	}
	if err := store.UpsertTrack(row, nil); err != nil {
		_ = store.Close()
		t.Fatalf("upsert legacy row: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	store, err = OpenAtPath(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store.Close()

	got, err := store.GetTrack("legacy")
	if err != nil {
		t.Fatalf("get migrated track: %v", err)
	}
	if len(got.Embedding) != 0 {
		t.Fatalf("embedding len=%d want=0", len(got.Embedding))
	}
	if got.AnalysisStatus != "pending" || got.AnalysisVersion != 0 {
		t.Fatalf("analysis state=%q version=%d want pending/0", got.AnalysisStatus, got.AnalysisVersion)
	}
	if got.Genre != "Rock" {
		t.Fatalf("metadata genre=%q want Rock", got.Genre)
	}
	if got.GenrePrimary != "" || got.GenreDetail != "" || got.GenreTagsJSON != "" || got.GenreLabel != "" {
		t.Fatalf("stale ML genre fields were not cleared: primary=%q detail=%q tags=%q label=%q", got.GenrePrimary, got.GenreDetail, got.GenreTagsJSON, got.GenreLabel)
	}
}
