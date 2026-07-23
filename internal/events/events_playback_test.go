package events

import (
	"errors"
	"path/filepath"
	"testing"

	"ray-player1/internal/db"
	"ray-player1/internal/library"
)

func openTestStore(t *testing.T) *db.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events-test.db")
	store, err := db.OpenAtPath(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func TestTechnicalSkipDoesNotAffectAffinity(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	svc := NewService(store, library.NewService(store, nil, nil, nil))
	track := library.Track{ID: "tech1", Title: "Tech", DurationMs: 180000}

	if err := store.UpsertTrack(db.TrackRow{ID: track.ID, Path: "/music/tech1.mp3", Title: track.Title, FileName: "tech1.mp3", Folder: "/music", DurationMs: track.DurationMs, DurationLabel: "3:00"}, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := svc.MarkPlay(track); err != nil {
		t.Fatalf("mark play: %v", err)
	}
	before, _ := store.ListTrackFeedback()
	affinityBefore := before[track.ID].Affinity

	if err := svc.MarkTechnicalSkip(track, "stream_error", 1200); err != nil {
		t.Fatalf("technical skip: %v", err)
	}
	if err := svc.MarkPlaybackFailed(track, "stream_error", errors.New("boom"), 1200); err != nil {
		t.Fatalf("playback failed: %v", err)
	}

	after, _ := store.ListTrackFeedback()
	item := after[track.ID]
	if item.Affinity != affinityBefore {
		t.Fatalf("affinity changed: before=%.3f after=%.3f", affinityBefore, item.Affinity)
	}
	if item.SkipEvents != 0 {
		t.Fatalf("expected no skip events, got %d", item.SkipEvents)
	}
}
