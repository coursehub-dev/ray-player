package appstate

import (
	"path/filepath"
	"testing"

	"ray-player1/internal/db"
	"ray-player1/internal/rays"
)

func TestReplacePersistsSynchronouslyAndTransientDoesNot(t *testing.T) {
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	state := NewStore(store)
	persistent := state.Get()
	persistent.CurrentTrackID = "track-persisted"
	persistent.PositionMs = 1234
	state.Replace(persistent)

	row, err := store.GetAppState()
	if err != nil {
		t.Fatal(err)
	}
	if row.CurrentTrackID != "track-persisted" || row.PositionMs != 1234 {
		t.Fatalf("unexpected persisted state: %+v", row)
	}

	preview := state.Get()
	preview.CurrentTrackID = "track-preview"
	preview.PositionMs = 5678
	state.ReplaceTransient(preview)

	row, err = store.GetAppState()
	if err != nil {
		t.Fatal(err)
	}
	if row.CurrentTrackID != "track-persisted" || row.PositionMs != 1234 {
		t.Fatalf("transient state leaked to database: %+v", row)
	}
	if got := state.Get().CurrentTrackID; got != "track-preview" {
		t.Fatalf("in-memory state=%q want track-preview", got)
	}
}

func TestGetReturnsIndependentQueueSlice(t *testing.T) {
	state := NewStore(nil)
	value := state.Get()
	value.CurrentTrackID = "a"
	value.Queue = []rays.QueueItem{{TrackID: "a"}}
	state.ReplaceTransient(value)

	copy := state.Get()
	copy.Queue[0].TrackID = "mutated"

	got := state.Get()
	if got.Queue[0].TrackID != "a" {
		t.Fatalf("Get exposed internal queue slice: %+v", got.Queue)
	}
}

func TestReplacePreservesExternalQueueMetadataWhenQueueIsNil(t *testing.T) {
	state := NewStore(nil)
	value := state.Get()
	value.Status = PlaybackPlaying
	value.CurrentTrackID = "podcast-1"
	value.Queue = nil
	value.QueueIndex = 3
	value.QueueLength = 12
	state.ReplaceTransient(value)

	got := state.Get()
	if got.QueueIndex != 3 || got.QueueLength != 12 {
		t.Fatalf("external queue metadata was overwritten: index=%d length=%d", got.QueueIndex, got.QueueLength)
	}
}
