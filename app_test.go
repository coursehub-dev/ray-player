package main

import (
	"context"
	"testing"
	"time"

	"ray-player1/internal/appstate"
	"ray-player1/internal/library"
	"ray-player1/internal/rays"
)

func TestPlaybackKindForID(t *testing.T) {
	tests := []struct {
		id   string
		want currentPlaybackKind
	}{
		{
			id:   "podcast_35e53eae4704e19af844b851",
			want: currentPlaybackPodcast,
		},
		{
			id:   "track_35e53eae4704e19af844b851",
			want: currentPlaybackMusic,
		},
	}

	for _, test := range tests {
		if got := playbackKindForID(test.id); got != test.want {
			t.Fatalf("playbackKindForID(%q) = %q, want %q", test.id, got, test.want)
		}
	}
}

func TestSkipRewardForSourceManualSwitchDoesNotHardPenalize(t *testing.T) {
	reward := skipRewardForSource("play_track_switch", 127, 254995)
	if reward != 0 {
		t.Fatalf("manual play switch at 127ms should not be preference penalty, got %.2f", reward)
	}
}

func TestRayBuildKeyDistinguishesPreviewAndActiveRay(t *testing.T) {
	a := &App{}
	k1 := a.rayBuildKey("seed", "continue_mood", "ray-1")
	k2 := a.rayBuildKey("seed", "continue_mood", "")
	if k1 == k2 {
		t.Fatal("ray build key must distinguish active ray and preview")
	}
}

func TestBeginPlayRequestSupersedesPreviousRequest(
	t *testing.T,
) {
	app := &App{
		ctx: context.Background(),
	}

	firstCtx, firstSeq := app.beginPlayRequest()
	secondCtx, secondSeq := app.beginPlayRequest()
	defer app.finishPlayRequest(secondSeq)

	if firstSeq == secondSeq {
		t.Fatal("play request sequence must advance")
	}
	if app.isCurrentPlayRequest(firstSeq) {
		t.Fatal("first request must be stale")
	}
	if !app.isCurrentPlayRequest(secondSeq) {
		t.Fatal("second request must be current")
	}

	select {
	case <-firstCtx.Done():
	default:
		t.Fatal("first request context must be canceled")
	}

	select {
	case <-secondCtx.Done():
		t.Fatal("current request must remain active")
	default:
	}
}

func TestFinishStalePlayRequestDoesNotCancelCurrent(
	t *testing.T,
) {
	app := &App{
		ctx: context.Background(),
	}

	_, firstSeq := app.beginPlayRequest()
	secondCtx, secondSeq := app.beginPlayRequest()

	app.finishPlayRequest(firstSeq)

	select {
	case <-secondCtx.Done():
		t.Fatal(
			"finishing stale request canceled current request",
		)
	default:
	}

	app.finishPlayRequest(secondSeq)
}

func TestVisibleRayQueueHidesPreviousRayWhileBuilding(
	t *testing.T,
) {
	queue := []rays.QueueItem{
		{TrackID: "old-track"},
	}

	got := visibleRayQueue(
		queue,
		appstate.RayBuildState{
			Status: appstate.RayBuildBuilding,
		},
	)

	if len(got) != 0 {
		t.Fatalf(
			"visible queue length = %d, want 0",
			len(got),
		)
	}
}

func TestVisibleRayQueueReturnsQueueWhenReady(
	t *testing.T,
) {
	queue := []rays.QueueItem{
		{TrackID: "new-track"},
	}

	got := visibleRayQueue(
		queue,
		appstate.RayBuildState{
			Status: appstate.RayBuildReady,
		},
	)

	if len(got) != 1 || got[0].TrackID != "new-track" {
		t.Fatalf("unexpected visible queue: %#v", got)
	}
}

func TestPlaybackEpochInvalidatesPreviousMedia(t *testing.T) {
	app := &App{}

	musicEpoch := app.beginPlaybackEpoch()
	if !app.playbackEpochIsCurrent(musicEpoch) {
		t.Fatal("new music epoch must be current")
	}

	podcastEpoch := app.beginPlaybackEpoch()
	if app.playbackEpochIsCurrent(musicEpoch) {
		t.Fatal("music epoch must be stale after podcast starts")
	}
	if !app.playbackEpochIsCurrent(podcastEpoch) {
		t.Fatal("podcast epoch must be current")
	}
}

func TestPodcastIDIsNotMusicPlayback(t *testing.T) {
	if got := playbackKindForID(
		"podcast_35e53eae4704e19af844b851",
	); got != currentPlaybackPodcast {
		t.Fatalf("kind = %q, want podcast", got)
	}
}

func TestPlaybackTickerStopsWhenContextIsCanceled(t *testing.T) {
	app := &App{state: appstate.NewStore(nil)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		app.playbackTicker(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("playback ticker did not stop after cancellation")
	}
}

func TestBackgroundLifecycleRejectsNewWorkAfterStop(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	app.runCtx = ctx
	app.runCancel = cancel

	started := make(chan struct{})
	if !app.launchBackground(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	}) {
		t.Fatal("expected background work to start")
	}
	<-started
	app.stopBackgroundWork()

	if app.launchBackground(func(context.Context) {}) {
		t.Fatal("background work must be rejected after shutdown begins")
	}
}

func TestShouldDeferReclusterWhileLibraryIsIndexing(t *testing.T) {
	if !shouldDeferRecluster(false, library.IndexingState{IsIndexing: true}) {
		t.Fatal("background embedding repair must defer reclustering")
	}
	if !shouldDeferRecluster(true, library.IndexingState{}) {
		t.Fatal("manual reindex must defer reclustering")
	}
	if shouldDeferRecluster(false, library.IndexingState{IsIndexing: false, Phase: "done"}) {
		t.Fatal("completed indexing must allow one final recluster")
	}
}
