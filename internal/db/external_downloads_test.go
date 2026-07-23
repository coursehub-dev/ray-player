package db

import (
	"testing"
)

func TestInsertPendingExternalTrack(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	if err := store.InsertPendingExternalTrack(
		"track-1",
		"/tmp/ray-player1/external/music/test.mp3",
		"Test Track",
		"Test Artist",
		123.4,
		"https://youtube.com/watch?v=test",
		"youtube",
		"abc123",
	); err != nil {
		t.Fatalf("insert pending track: %v", err)
	}

	row, err := store.GetTrack("track-1")
	if err != nil {
		t.Fatalf("get track: %v", err)
	}
	if row.SourceType != "yt_dlp" {
		t.Fatalf("unexpected source type: %#v", row.SourceType)
	}
	if row.DownloadStatus != "queued" {
		t.Fatalf("unexpected download status: %#v", row.DownloadStatus)
	}
	if row.DownloadProgress != 0 {
		t.Fatalf("unexpected download progress: %#v", row.DownloadProgress)
	}
	if row.AddedAt == 0 {
		t.Fatalf("expected created_at to be set, got %#v", row.AddedAt)
	}
}

func TestInsertPendingExternalPodcast(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	if err := store.InsertPendingExternalPodcast(
		"pod-1",
		"/tmp/ray-player1/external/podcast/test.mp3",
		"Test Podcast",
		"Test Author",
		321.0,
		"https://youtube.com/watch?v=podcast",
		"youtube",
		"pod123",
	); err != nil {
		t.Fatalf("insert pending podcast: %v", err)
	}

	row, err := store.PodcastItemByID("pod-1")
	if err != nil {
		t.Fatalf("get podcast item: %v", err)
	}
	if row.SourceType != "yt_dlp" {
		t.Fatalf("unexpected source type: %#v", row.SourceType)
	}
	if row.DownloadStatus != "queued" {
		t.Fatalf("unexpected download status: %#v", row.DownloadStatus)
	}
	if row.DownloadProgress != 0 {
		t.Fatalf("unexpected download progress: %#v", row.DownloadProgress)
	}
	if row.AddedAt == 0 {
		t.Fatalf("expected added_at to be set, got %#v", row.AddedAt)
	}
}

func TestTrackByPathReturnsPendingExternalItem(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	path := "/tmp/ray-player1/external/music/test.mp3"
	if err := store.InsertPendingExternalTrack(
		"track-lookup",
		path,
		"Test Track",
		"Test Artist",
		123.4,
		"https://youtube.com/watch?v=test",
		"youtube",
		"abc123",
	); err != nil {
		t.Fatalf("insert pending track: %v", err)
	}

	row, found, err := store.TrackByPath(path)
	if err != nil {
		t.Fatalf("track by path: %v", err)
	}
	if !found {
		t.Fatal("expected pending track lookup to find row")
	}
	if row.ID != "track-lookup" {
		t.Fatalf("unexpected track id: %#v", row.ID)
	}
}
