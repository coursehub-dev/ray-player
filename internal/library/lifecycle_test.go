package library

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"ray-player1/internal/db"
)

func TestPendingRealFileFeaturesContainNoSyntheticSemantics(t *testing.T) {
	f := pendingFeatures()
	if f.Tempo != 0 || f.Energy != 0 || f.Danceability != 0 || f.Valence != 0 ||
		f.Acousticness != 0 || f.Instrumentalness != 0 || len(f.Embedding) != 0 {
		t.Fatalf("pending real-file features must be unknown, got %+v", f)
	}
}

func TestServiceCloseIsIdempotentAndStopsNewAnalysis(t *testing.T) {
	store, err := db.OpenAtPath(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := NewService(store, nil, nil, nil)
	service.Close()
	service.Close()

	service.enqueueAnalysis("after-close", "/does/not/exist.mp3", nil, "test")
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.inFlight["after-close"] {
		t.Fatal("closed service accepted a new analysis job")
	}
}

func TestSemanticAnalysisStateRequiresValidatedEmbedding(t *testing.T) {
	track := Track{Embedding: make([]float32, 16)}
	applySemanticAnalysisState(&track, false, nil)
	if track.AnalysisStatus != string(AnalysisQueued) || track.AnalysisVersion != 0 {
		t.Fatalf("missing semantic model must stay queued: %+v", track)
	}
	if len(track.Embedding) != 0 {
		t.Fatalf("compact DSP vector leaked as semantic embedding: len=%d", len(track.Embedding))
	}

	track.Embedding = make([]float32, 1280)
	applySemanticAnalysisState(&track, true, nil)
	if track.AnalysisStatus != string(AnalysisDone) || track.AnalysisVersion != currentAnalysisVersion {
		t.Fatalf("validated semantic output must complete analysis: %+v", track)
	}
}

func TestSemanticAnalysisStateKeepsInferenceFailureRetryable(t *testing.T) {
	track := Track{Embedding: make([]float32, 1280)}
	applySemanticAnalysisState(&track, false, errors.New("bad model output"))
	if track.AnalysisStatus != string(AnalysisError) || track.AnalysisVersion != 0 {
		t.Fatalf("failed semantic inference must not become current: %+v", track)
	}
	if track.AnalysisError == "" || len(track.Embedding) != 0 {
		t.Fatalf("failed semantic inference must retain diagnostics and clear embedding: %+v", track)
	}
}

func TestFinishAnalysisRequeuesJobDeferredByEngineReload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := &Service{
		ctx:           ctx,
		cancel:        cancel,
		analysisQueue: make(chan analysisJob, 1),
		inFlight:      map[string]bool{"track": true},
		requeue: map[string]analysisJob{
			"track": {TrackID: "track", Path: "/music/track.mp3", Source: "embedding-repair"},
		},
	}

	service.finishAnalysis("track")
	select {
	case job := <-service.analysisQueue:
		if job.TrackID != "track" {
			t.Fatalf("requeued track=%q want track", job.TrackID)
		}
	default:
		t.Fatal("deferred repair was not requeued")
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	if !service.inFlight["track"] {
		t.Fatal("requeued repair must remain in-flight")
	}
}

func TestUpdateEnginesDefersRepairAlreadyInFlight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := &Service{
		ctx:    ctx,
		cancel: cancel,
		cache: map[string]Track{
			"track": {
				ID:                   "track",
				Path:                 "/music/track.mp3",
				AnalyzedLevel:        2,
				AnalysisVersion:      currentAnalysisVersion - 1,
				EssentiaModelVersion: "old",
			},
		},
		analysisQueue: make(chan analysisJob, 1),
		inFlight:      map[string]bool{"track": true},
		requeue:       map[string]analysisJob{},
	}

	service.UpdateEngines(nil, nil, nil)
	service.mu.RLock()
	_, deferred := service.requeue["track"]
	service.mu.RUnlock()
	if !deferred {
		t.Fatal("engine reload must defer a repair for an already-running track")
	}
	if len(service.analysisQueue) != 0 {
		t.Fatal("in-flight repair must not be duplicated immediately")
	}
}
