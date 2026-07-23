package podcast

import (
	"path/filepath"
	"testing"
	"time"

	"ray-player1/internal/db"
)

func TestPodcastHistoryLifecycle(t *testing.T) {
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	if err := store.UpsertPodcastItem(db.PodcastItemRow{
		ID:             "podcast-test",
		Path:           "/podcasts/test.mp3",
		Title:          "Test podcast",
		Folder:         "/podcasts",
		Duration:       1000,
		AddedAt:        now,
		UpdatedAt:      now,
		SemanticStatus: "metadata_ready",
		DownloadStatus: "ready",
	}); err != nil {
		t.Fatalf("upsert podcast: %v", err)
	}

	service := NewHistoryService(store)
	item := Item{
		ID:       "podcast-test",
		Title:    "Test podcast",
		Duration: 1000,
	}

	if err := service.Begin(item, "ray-test", "ray", 100); err != nil {
		t.Fatalf("begin: %v", err)
	}

	service.mu.Lock()
	service.active.LastTickAt = time.Now().Add(-5 * time.Second)
	service.mu.Unlock()

	if err := service.Tick(item.ID, 105, item.Duration, true); err != nil {
		t.Fatalf("tick: %v", err)
	}

	if err := service.Finish(item.ID, 250, item.Duration, "switch_item"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	history, err := service.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history size = %d, want 1", len(history))
	}

	entry := history[0]
	if entry.Item.ID != item.ID {
		t.Fatalf("item id = %q, want %q", entry.Item.ID, item.ID)
	}
	if entry.RayID != "ray-test" {
		t.Fatalf("ray id = %q, want ray-test", entry.RayID)
	}
	if entry.Source != "ray" {
		t.Fatalf("source = %q, want ray", entry.Source)
	}
	if entry.EndReason != "switch_item" {
		t.Fatalf("end reason = %q, want switch_item", entry.EndReason)
	}
	if entry.ProgressPercent != 25 {
		t.Fatalf("progress = %d, want 25", entry.ProgressPercent)
	}
	if entry.ListenedSeconds < 4 {
		t.Fatalf("listened seconds = %v, want >= 4", entry.ListenedSeconds)
	}
}
