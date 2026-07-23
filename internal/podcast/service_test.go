package podcast

import (
	"path/filepath"
	"testing"
	"time"

	"ray-player1/internal/db"
)

func TestResumePosition(t *testing.T) {
	tests := []struct {
		name        string
		position    float64
		isCompleted bool
		want        float64
	}{
		{
			name:        "new episode (short) starts at zero",
			position:    20,
			isCompleted: false,
			want:        0,
		},
		{
			name:        "completed episode starts at zero",
			position:    3600,
			isCompleted: true,
			want:        0,
		},
		{
			name:        "nearly-complete but not marked done rewinds five seconds",
			position:    3420,
			isCompleted: false,
			want:        3415,
		},
		{
			name:        "unfinished episode rewinds five seconds",
			position:    125,
			isCompleted: false,
			want:        120,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResumePosition(test.position, test.isCompleted)
			if got != test.want {
				t.Fatalf("ResumePosition() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFolderAffinity(t *testing.T) {
	root := filepath.Join("podcasts", "history")
	rome := filepath.Join(root, "rome")
	greece := filepath.Join(root, "greece")

	if got := folderAffinity(rome, rome); got != 1 {
		t.Fatalf("same folder affinity = %v, want 1", got)
	}
	if got := folderAffinity(rome, greece); got != 0.65 {
		t.Fatalf("sibling folder affinity = %v, want 0.65", got)
	}
	if got := folderAffinity(rome, filepath.Join(rome, "season-1")); got != 0.85 {
		t.Fatalf("child folder affinity = %v, want 0.85", got)
	}
}

func TestBuildRayPrefersSameFolder(t *testing.T) {
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	items := []db.PodcastItemRow{
		{
			ID:             "seed",
			Path:           "/podcasts/history/rome/01.mp3",
			Title:          "Rome 1",
			Folder:         "/podcasts/history/rome",
			AddedAt:        now,
			UpdatedAt:      now,
			SemanticStatus: "metadata_ready",
		},
		{
			ID:             "same-folder",
			Path:           "/podcasts/history/rome/02.mp3",
			Title:          "Rome 2",
			Folder:         "/podcasts/history/rome",
			AddedAt:        now,
			UpdatedAt:      now,
			SemanticStatus: "metadata_ready",
		},
		{
			ID:             "other-folder",
			Path:           "/podcasts/science/space/01.mp3",
			Title:          "Space",
			Folder:         "/podcasts/science/space",
			AddedAt:        now,
			UpdatedAt:      now,
			SemanticStatus: "metadata_ready",
		},
	}

	for _, item := range items {
		if err := store.UpsertPodcastItem(item); err != nil {
			t.Fatalf("upsert %s: %v", item.ID, err)
		}
	}

	service := NewService(store)
	ray, err := service.BuildRay("seed", 10)
	if err != nil {
		t.Fatalf("build ray: %v", err)
	}
	if len(ray.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(ray.Items))
	}
	if ray.Items[0].Item.ID != "seed" {
		t.Fatalf("first item = %q, want seed", ray.Items[0].Item.ID)
	}
	if ray.Items[1].Item.ID != "same-folder" {
		t.Fatalf(
			"second item = %q, want same-folder",
			ray.Items[1].Item.ID,
		)
	}
	if ray.Items[1].Reason != "Из той же папки" {
		t.Fatalf(
			"reason = %q, want same-folder reason",
			ray.Items[1].Reason,
		)
	}
}

func TestUpdateProgressRefreshesCurrentRayItem(t *testing.T) {
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	item := db.PodcastItemRow{
		ID:             "seed",
		Path:           "/podcasts/episode.mp3",
		Title:          "Episode",
		Folder:         "/podcasts",
		Duration:       3600,
		AddedAt:        now,
		UpdatedAt:      now,
		SemanticStatus: "metadata_ready",
	}
	if err := store.UpsertPodcastItem(item); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	service := NewService(store)
	if _, err := service.BuildRay("seed", 10); err != nil {
		t.Fatalf("build ray: %v", err)
	}

	updated, err := service.UpdateProgress(
		"seed",
		900,
		3600,
	)
	if err != nil {
		t.Fatalf("update progress: %v", err)
	}
	if updated.ProgressPercentage != 25 {
		t.Fatalf(
			"progress = %d, want 25",
			updated.ProgressPercentage,
		)
	}

	ray := service.CurrentRay()
	if got := ray.Items[0].Item.ProgressPercentage; got != 25 {
		t.Fatalf(
			"ray progress = %d, want 25",
			got,
		)
	}
}

func TestApplySortDoesNotChangeContent(t *testing.T) {
	ray := Ray{
		SeedItemID:   "seed",
		CurrentIndex: 0,
		Items: []RayItem{
			{
				Item: Item{
					ID:         "seed",
					Title:      "Zulu",
					ImportedAt: 100,
				},
				Score:            1,
				OriginalPosition: 0,
				Current:          true,
			},
			{
				Item: Item{
					ID:         "a",
					Title:      "Alpha",
					ImportedAt: 300,
				},
				Score:            0.4,
				OriginalPosition: 1,
			},
			{
				Item: Item{
					ID:         "b",
					Title:      "Beta",
					ImportedAt: 200,
				},
				Score:            0.8,
				OriginalPosition: 2,
			},
		},
	}

	applySort(&ray, SortNameAsc)
	if len(ray.Items) != 3 {
		t.Fatalf("sort changed content size: %d", len(ray.Items))
	}
	if ray.Items[0].Item.ID != "a" ||
		ray.Items[1].Item.ID != "b" ||
		ray.Items[2].Item.ID != "seed" {
		t.Fatalf("unexpected name order: %#v", ray.Items)
	}

	applySort(&ray, SortDateDesc)
	if ray.Items[0].Item.ID != "a" ||
		ray.Items[1].Item.ID != "b" ||
		ray.Items[2].Item.ID != "seed" {
		t.Fatalf("unexpected date order: %#v", ray.Items)
	}
}

func TestCurrentFolderIncludesSubfoldersOnly(t *testing.T) {
	seed := filepath.Join(
		string(filepath.Separator),
		"podcasts",
		"history",
		"rome",
	)

	if !isInFolderScope(seed, seed, true) {
		t.Fatal("same folder must be included")
	}
	if !isInFolderScope(
		seed,
		filepath.Join(seed, "season-1"),
		true,
	) {
		t.Fatal("subfolder must be included")
	}
	if isInFolderScope(
		seed,
		filepath.Join(filepath.Dir(seed), "greece"),
		true,
	) {
		t.Fatal("sibling folder must not be included")
	}
}

func TestTopicBridgeScore(t *testing.T) {
	tests := []struct {
		sim  float64
		want float64
	}{
		{sim: 0.2, want: 0},
		{sim: 0.6, want: 1},
		{sim: 0.9, want: 0.35},
	}

	for _, test := range tests {
		if got := topicBridgeScore(test.sim); got != test.want {
			t.Fatalf(
				"topicBridgeScore(%v) = %v, want %v",
				test.sim,
				got,
				test.want,
			)
		}
	}
}

func TestRemoveDoesNotEnableManualOrder(t *testing.T) {
	service := &Service{
		currentRay: Ray{
			ID:          "ray",
			SeedItemID:  "seed",
			ContentMode: ContentExplore,
			SortMode:    SortDateDesc,
			Items: []RayItem{
				{
					Item:    Item{ID: "seed"},
					Current: true,
				},
				{
					Item: Item{ID: "remove"},
				},
			},
		},
	}

	service.mu.Lock()
	service.currentRay.Items =
		service.currentRay.Items[:1]
	reindexRayItems(&service.currentRay, "seed")
	got := cloneRay(service.currentRay)
	service.mu.Unlock()

	if got.SortMode != SortDateDesc {
		t.Fatalf(
			"remove changed sort mode to %q",
			got.SortMode,
		)
	}
	if got.IsManualOrder {
		t.Fatal("remove must not enable manual order")
	}
}

func TestNormalizeTitleForSort(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{
			value: "  The History of Rome ",
			want:  "history of rome",
		},
		{
			value: "A Philosophy Podcast",
			want:  "philosophy podcast",
		},
		{
			value: "История России",
			want:  "история россии",
		},
	}

	for _, test := range tests {
		if got := normalizeTitleForSort(test.value); got != test.want {
			t.Fatalf(
				"normalizeTitleForSort(%q) = %q, want %q",
				test.value,
				got,
				test.want,
			)
		}
	}
}

func TestOpenSavedRayRestoresFrozenOrder(t *testing.T) {
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "rays.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	for _, item := range []db.PodcastItemRow{
		{
			ID:             "seed",
			Path:           "/podcasts/seed.mp3",
			Title:          "Seed",
			Folder:         "/podcasts",
			Duration:       100,
			AddedAt:        now,
			UpdatedAt:      now,
			SemanticStatus: "metadata_ready",
			DownloadStatus: "ready",
		},
		{
			ID:             "second",
			Path:           "/podcasts/second.mp3",
			Title:          "Second",
			Folder:         "/podcasts",
			Duration:       100,
			AddedAt:        now,
			UpdatedAt:      now,
			SemanticStatus: "metadata_ready",
			DownloadStatus: "ready",
		},
	} {
		if err := store.UpsertPodcastItem(item); err != nil {
			t.Fatalf("upsert %s: %v", item.ID, err)
		}
	}

	service := NewService(store)
	ray, err := service.BuildRay("seed", 10)
	if err != nil {
		t.Fatalf("build ray: %v", err)
	}

	if _, err := service.MoveCurrentRayItem(1, 0); err != nil {
		t.Fatalf("move ray item: %v", err)
	}

	service.currentRay = Ray{}

	restored, err := service.OpenSavedRay(ray.ID, "second")
	if err != nil {
		t.Fatalf("open saved ray: %v", err)
	}

	if len(restored.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(restored.Items))
	}
	if restored.Items[0].Item.ID != "second" {
		t.Fatalf("first item = %q, want second", restored.Items[0].Item.ID)
	}
	if restored.CurrentIndex != 0 {
		t.Fatalf("current index = %d, want 0", restored.CurrentIndex)
	}
}
