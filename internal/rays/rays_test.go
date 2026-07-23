package rays

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ray-player1/internal/db"
	"ray-player1/internal/library"
	"ray-player1/internal/onnx"
)

func TestResumePreservesQueueWithoutRebuild(t *testing.T) {
	store, lib, svc := newTestRayServices(t)
	defer store.Close()

	seed, _ := lib.ImportVirtualFile(filepath.Join(t.TempDir(), "seed.mp3"), 3*time.Minute)
	next, _ := lib.ImportVirtualFile(filepath.Join(t.TempDir(), "next.mp3"), 3*time.Minute)
	other, _ := lib.ImportVirtualFile(filepath.Join(t.TempDir(), "other.mp3"), 3*time.Minute)

	rayID := svc.Activate(seed, []QueueItem{
		{TrackID: seed.ID, Title: seed.Title, Subtitle: "текущий трек", DurationLabel: seed.DurationLabel, IsCurrent: true, Bucket: "core", Strategy: "seed", Score: 1},
		{TrackID: next.ID, Title: next.Title, Subtitle: "далее", DurationLabel: next.DurationLabel, Bucket: "core", Strategy: "same_cluster", Score: 0.9},
	})
	if !svc.JumpToTrack(next.ID) {
		t.Fatal("expected jump to next track")
	}

	resumed, ok := svc.Resume(rayID)
	if !ok {
		t.Fatal("expected resume to succeed")
	}
	if resumed.CurrentTrackID != next.ID {
		t.Fatalf("expected resumed current track %s, got %s", next.ID, resumed.CurrentTrackID)
	}
	if len(resumed.Queue) != 2 {
		t.Fatalf("expected preserved queue len 2, got %d", len(resumed.Queue))
	}
	for _, item := range resumed.Queue {
		if item.TrackID == other.ID {
			t.Fatal("resume rebuilt queue unexpectedly")
		}
	}
}

func TestPreviousMovesBackInQueue(t *testing.T) {
	store, lib, svc := newTestRayServices(t)
	defer store.Close()

	a, _ := lib.ImportVirtualFile(filepath.Join(t.TempDir(), "a.mp3"), 3*time.Minute)
	b, _ := lib.ImportVirtualFile(filepath.Join(t.TempDir(), "b.mp3"), 3*time.Minute)
	c, _ := lib.ImportVirtualFile(filepath.Join(t.TempDir(), "c.mp3"), 3*time.Minute)

	svc.Activate(a, []QueueItem{
		{TrackID: a.ID, Title: a.Title, Subtitle: "текущий трек", DurationLabel: a.DurationLabel, IsCurrent: true},
		{TrackID: b.ID, Title: b.Title, Subtitle: "далее", DurationLabel: b.DurationLabel},
		{TrackID: c.ID, Title: c.Title, Subtitle: "далее", DurationLabel: c.DurationLabel},
	})
	if !svc.JumpToTrack(c.ID) {
		t.Fatal("expected jump to c")
	}
	item, ok := svc.Previous()
	if !ok {
		t.Fatal("expected previous to succeed")
	}
	if item.TrackID != b.ID {
		t.Fatalf("expected previous track %s, got %s", b.ID, item.TrackID)
	}
}

func newTestRayServices(t *testing.T) (*db.Store, *library.Service, *Service) {
	t.Helper()
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "ray.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	lib := library.NewService(store, nil, nil, nil)
	svc := NewService(store, lib)
	return store, lib, svc
}

func TestQueueItemRole(t *testing.T) {
	tests := []struct {
		name     string
		item     QueueItem
		position int
		want     string
	}{
		{
			name: "discovery bucket",
			item: QueueItem{Bucket: "discovery"},
			want: "discovery",
		},
		{
			name: "explore strategy",
			item: QueueItem{Strategy: "explore"},
			want: "discovery",
		},
		{
			name: "bridge",
			item: QueueItem{Bucket: "bridge"},
			want: "bridge",
		},
		{
			name: "manual",
			item: QueueItem{Strategy: "manual"},
			want: "manual",
		},
		{
			name:     "legacy current first item",
			item:     QueueItem{IsCurrent: true},
			position: 0,
			want:     "seed",
		},
		{
			name:     "default",
			item:     QueueItem{},
			position: 3,
			want:     "next",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queueItemRole(tt.item, tt.position); got != tt.want {
				t.Fatalf("queueItemRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeQueueMetadataPreservesOldFields(t *testing.T) {
	input := []QueueItem{
		{
			TrackID: "track-1",
			Title:   "Track",
			Reason:  "похожее настроение",
			Bucket:  "nearby",
		},
	}

	got := normalizeQueueMetadata(input)
	if len(got) != 1 {
		t.Fatalf("unexpected queue length: %d", len(got))
	}
	if got[0].Position != 0 {
		t.Fatalf("Position = %d, want 0", got[0].Position)
	}
	if got[0].RayRole != "nearby" {
		t.Fatalf("RayRole = %q, want nearby", got[0].RayRole)
	}
	if got[0].RayReason != input[0].Reason {
		t.Fatalf("RayReason = %q, want %q", got[0].RayReason, input[0].Reason)
	}
	if !reflect.DeepEqual(got[0].GenreTags, []onnx.GenreTag{}) {
		t.Fatalf("GenreTags must be a non-nil empty array: %#v", got[0].GenreTags)
	}
}

func TestApplySortRestoresRecommendedOrder(t *testing.T) {
	ray := Ray{
		CurrentTrackID: "a",
		Queue: []QueueItem{
			{
				TrackID: "a",
				Title:   "Zulu",
				Track: library.Track{
					ID:         "a",
					Title:      "Zulu",
					ImportedAt: 100,
				},
				Score:            0.3,
				OriginalPosition: 0,
			},
			{
				TrackID: "b",
				Title:   "Alpha",
				Track: library.Track{
					ID:         "b",
					Title:      "Alpha",
					ImportedAt: 300,
				},
				Score:            0.9,
				OriginalPosition: 1,
			},
			{
				TrackID: "c",
				Title:   "Beta",
				Track: library.Track{
					ID:         "c",
					Title:      "Beta",
					ImportedAt: 200,
				},
				Score:            0.6,
				OriginalPosition: 2,
			},
		},
	}

	applySort(&ray, SortNameAsc, "a")
	if ray.Queue[0].TrackID != "b" {
		t.Fatalf(
			"name asc first = %s, want b",
			ray.Queue[0].TrackID,
		)
	}

	applySort(&ray, SortDateDesc, "a")
	if ray.Queue[0].TrackID != "b" {
		t.Fatalf(
			"date desc first = %s, want b",
			ray.Queue[0].TrackID,
		)
	}

	applySort(&ray, SortRecommended, "a")
	if ray.Queue[0].TrackID != "b" ||
		ray.Queue[1].TrackID != "c" ||
		ray.Queue[2].TrackID != "a" {
		t.Fatalf(
			"unexpected recommended order: %#v",
			ray.Queue,
		)
	}
}

func TestManualMoveDoesNotChangeContentMode(t *testing.T) {
	ray := Ray{
		ID:          "ray",
		ContentMode: ContentCoolDown,
		SortMode:    SortRecommended,
		Queue: []QueueItem{
			{TrackID: "a"},
			{TrackID: "b"},
			{TrackID: "c"},
		},
	}

	item := ray.Queue[2]
	ray.Queue = append(ray.Queue[:2], ray.Queue[3:]...)
	ray.Queue = append([]QueueItem{item}, ray.Queue...)
	ray.SortMode = SortManual
	ray.IsManualOrder = true

	if ray.ContentMode != ContentCoolDown {
		t.Fatalf(
			"content mode changed to %q",
			ray.ContentMode,
		)
	}
	if ray.SortMode != SortManual {
		t.Fatalf(
			"sort mode = %q, want manual",
			ray.SortMode,
		)
	}
}
