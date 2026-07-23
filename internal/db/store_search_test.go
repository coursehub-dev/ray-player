package db

import (
	"path/filepath"
	"testing"
)

func TestSearchTracksPrefersExactTitle(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	mustUpsertTrack(t, store, TrackRow{ID: "1", Path: "/music/exact.mp3", Title: "Blue Monday", Artist: "New Order", Album: "Substance", Genre: "Synthpop", FileName: "Blue Monday.mp3", Folder: "/music", DurationLabel: "3:00"})
	mustUpsertTrack(t, store, TrackRow{ID: "2", Path: "/music/fuzzy.mp3", Title: "Monday Blues", Artist: "Someone", Album: "Other", Genre: "Rock", FileName: "Monday Blues.mp3", Folder: "/music", DurationLabel: "3:00"})

	results, err := store.SearchTracks("Blue Monday", []string{"blu", "lue", "mon", "ond", "nda", "day"}, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].ID != "1" {
		t.Fatalf("expected exact title first, got %s", results[0].ID)
	}
}

func TestSearchTracksEmptyQueryUsesBehaviorRanking(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	mustUpsertTrack(t, store, TrackRow{ID: "popular", Path: "/music/popular.mp3", Title: "Popular", Artist: "A", FileName: "popular.mp3", Folder: "/music", DurationLabel: "3:00", PlayCount: 10, CompleteCount: 6})
	mustUpsertTrack(t, store, TrackRow{ID: "skipped", Path: "/music/skipped.mp3", Title: "Skipped", Artist: "B", FileName: "skipped.mp3", Folder: "/music", DurationLabel: "3:00", PlayCount: 10, SkipCount: 9})

	results, err := store.SearchTracks("", nil, 10)
	if err != nil {
		t.Fatalf("empty-query search failed: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].ID != "popular" {
		t.Fatalf("expected popular track first, got %s", results[0].ID)
	}
}

func TestRecordEventUpdatesTrackFeedback(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	mustUpsertTrack(t, store, TrackRow{ID: "fb1", Path: "/music/fb1.mp3", Title: "Feedback", Artist: "A", FileName: "fb1.mp3", Folder: "/music", DurationLabel: "3:00"})
	if err := store.RecordEvent("fb1", "play_start", 0, 180000); err != nil {
		t.Fatalf("play_start: %v", err)
	}
	if err := store.RecordEvent("fb1", "play_half", 90000, 180000); err != nil {
		t.Fatalf("play_half: %v", err)
	}
	if err := store.RecordEvent("fb1", "manual_next", 100000, 180000); err != nil {
		t.Fatalf("manual_next: %v", err)
	}
	feedback, err := store.ListTrackFeedback()
	if err != nil {
		t.Fatalf("list feedback: %v", err)
	}
	item, ok := feedback["fb1"]
	if !ok {
		t.Fatal("expected feedback row")
	}
	if item.PlayEvents < 2 {
		t.Fatalf("expected play events to grow, got %+v", item)
	}
	if item.SkipEvents < 1 || item.LastEventType != "manual_next" {
		t.Fatalf("expected skip/manual_next to persist, got %+v", item)
	}
	if item.AvgCompletion <= 0 {
		t.Fatalf("expected avg completion > 0, got %+v", item)
	}
	if item.LastPlayedAt == 0 {
		t.Fatalf("expected last_played_at, got %+v", item)
	}
	recent, err := store.RecentTrackIDs(10)
	if err != nil {
		t.Fatalf("recent tracks: %v", err)
	}
	if len(recent) == 0 || recent[0] != "fb1" {
		t.Fatalf("expected fb1 in recent list, got %#v", recent)
	}
}

func TestSearchTracksFilenameFallbackFindsTrack(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	mustUpsertTrack(t, store, TrackRow{ID: "f1", Path: "/music/rare-live-cut.mp3", Title: "Unknown", Artist: "Unknown Artist", FileName: "rare-live-cut.mp3", Folder: "/music", DurationLabel: "3:00"})

	results, err := store.SearchTracks("live cut", []string{"liv", "ive", "cut"}, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 || results[0].ID != "f1" {
		t.Fatalf("expected filename fallback hit, got %#v", results)
	}
}

func TestGetTrackByNormalizedPathPrefersNormalizedPath(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	mustUpsertTrack(t, store, TrackRow{
		ID:             "path-hit",
		Path:           "/tmp/ray-player1/path-hit.mp3",
		NormalizedPath: "/tmp/ray-player1/normalized-path.mp3",
		Title:          "Path Hit",
		Artist:         "Test Artist",
		FileName:       "path-hit.mp3",
		Folder:         "/tmp/ray-player1",
		DurationLabel:  "3:00",
	})
	mustUpsertTrack(t, store, TrackRow{
		ID:             "normalized-hit",
		Path:           "/tmp/ray-player1/other-path.mp3",
		NormalizedPath: "/tmp/ray-player1/path-hit.mp3",
		Title:          "Normalized Hit",
		Artist:         "Test Artist",
		FileName:       "other-path.mp3",
		Folder:         "/tmp/ray-player1",
		DurationLabel:  "3:00",
	})

	row, err := store.GetTrackByNormalizedPath("/tmp/ray-player1/path-hit.mp3")
	if err != nil {
		t.Fatalf("get by normalized path: %v", err)
	}
	if row.ID != "normalized-hit" {
		t.Fatalf("expected normalized-path match, got %#v", row.ID)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenAtPath(path)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	return store
}

func mustUpsertTrack(t *testing.T, store *Store, row TrackRow) {
	t.Helper()
	row.DurationMs = 180000
	row.DurationLabel = "3:00"
	row.MetadataSource = "test"
	if err := store.UpsertTrack(row, []string{"tes", "est"}); err != nil {
		t.Fatalf("upsert track %s: %v", row.ID, err)
	}
}
