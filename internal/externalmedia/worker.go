package externalmedia

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/logx"
)

var workerLog = logx.New("YtDlpWorker")

var retryDelays = []time.Duration{
	2 * time.Second,
	8 * time.Second,
	30 * time.Second,
}

type Hooks struct {
	Settings func() Settings
	Emit     func(event string, payload JobDTO)

	OnMusicReady func(
		itemID string,
		path string,
	) error

	OnPodcastReady func(
		itemID string,
		path string,
	) error

	OnLibraryChanged func()
}

type Store interface {
	QueuedExternalDownloadJobs() ([]Job, error)
	ExternalDownloadJob(id string) (Job, error)
	UpdateExternalDownloadJob(id string, status Status, progress float64, errText string) error
	IncrementExternalDownloadAttempt(id string) error
	MarkExternalDownloadReady(id string, outputPath string) error
	UpdateExternalItemDownloadState(libraryType LibraryType, itemID string, status Status, progress float64, errorText string, attempts int) error
	MarkExternalTrackDownloaded(itemID string, path string) error
	MarkExternalPodcastDownloaded(itemID string, path string) error
}

type Worker struct {
	store Store
	hooks Hooks

	queue chan string

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	running map[string]context.CancelFunc
	queued  map[string]struct{}
}

func NewWorker(
	store Store,
	hooks Hooks,
) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		store:   store,
		hooks:   hooks,
		queue:   make(chan string, 128),
		ctx:     ctx,
		cancel:  cancel,
		running: make(map[string]context.CancelFunc),
		queued:  make(map[string]struct{}),
	}
}

func (w *Worker) Start() {
	go w.loop()

	jobs, err := w.store.QueuedExternalDownloadJobs()
	if err != nil {
		workerLog.E("restore queued jobs: %v", err)
		return
	}
	for _, job := range jobs {
		w.Enqueue(job.ID)
	}
}

func (w *Worker) Close() {
	w.cancel()

	w.mu.Lock()
	for _, cancel := range w.running {
		cancel()
	}
	w.running = make(map[string]context.CancelFunc)
	w.mu.Unlock()
}

func (w *Worker) Enqueue(jobID string) {
	w.mu.Lock()
	if _, exists := w.queued[jobID]; exists {
		w.mu.Unlock()
		return
	}
	w.queued[jobID] = struct{}{}
	w.mu.Unlock()

	select {
	case w.queue <- jobID:
	case <-w.ctx.Done():
	}
}

func (w *Worker) Cancel(jobID string) error {
	w.mu.Lock()
	cancel := w.running[jobID]
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if err := w.store.UpdateExternalDownloadJob(
		jobID,
		StatusCanceled,
		0,
		"canceled by user",
	); err != nil {
		return err
	}

	job, err := w.store.ExternalDownloadJob(jobID)
	if err == nil {
		_ = w.store.UpdateExternalItemDownloadState(
			job.LibraryType,
			job.ItemID,
			StatusCanceled,
			job.Progress,
			"canceled by user",
			job.Attempts,
		)
		w.emit("external-download:canceled", job)
	}
	w.notifyLibraryChanged()
	return nil
}

func (w *Worker) loop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case jobID := <-w.queue:
			w.mu.Lock()
			delete(w.queued, jobID)
			w.mu.Unlock()
			w.process(jobID)
		}
	}
}

func (w *Worker) process(jobID string) {
	job, err := w.store.ExternalDownloadJob(jobID)
	if err != nil {
		workerLog.E("load job=%s: %v", jobID, err)
		return
	}
	if job.Status == StatusReady ||
		job.Status == StatusCanceled {
		return
	}

	ctx, cancel := context.WithCancel(w.ctx)
	w.mu.Lock()
	w.running[jobID] = cancel
	w.mu.Unlock()
	defer func() {
		cancel()
		w.mu.Lock()
		delete(w.running, jobID)
		w.mu.Unlock()
	}()

	settings := w.hooks.Settings()
	client := NewClient(settings.YtDlpPath)

	for job.Attempts < job.MaxAttempts {
		if ctx.Err() != nil {
			return
		}

		if err := w.store.IncrementExternalDownloadAttempt(
			job.ID,
		); err != nil {
			workerLog.E("increment attempt job=%s: %v", job.ID, err)
			return
		}

		job, _ = w.store.ExternalDownloadJob(job.ID)
		_ = w.update(
			job,
			StatusDownloading,
			maxFloat(job.Progress, 0.03),
			"",
		)

		err := w.tryDownload(ctx, client, job, settings)
		if err == nil {
			return
		}

		if errors.Is(err, context.Canceled) {
			return
		}

		job, _ = w.store.ExternalDownloadJob(job.ID)
		workerLog.W(
			"download attempt failed job=%s attempt=%d/%d err=%v",
			job.ID,
			job.Attempts,
			job.MaxAttempts,
			err,
		)

		if job.Attempts >= job.MaxAttempts {
			_ = w.update(
				job,
				StatusError,
				job.Progress,
				err.Error(),
			)
			w.emit("external-download:error", job)
			w.notifyLibraryChanged()
			return
		}

		delayIndex := job.Attempts - 1
		if delayIndex >= len(retryDelays) {
			delayIndex = len(retryDelays) - 1
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelays[delayIndex]):
		}
	}
}

func (w *Worker) tryDownload(
	ctx context.Context,
	client *Client,
	job Job,
	settings Settings,
) error {
	outputDir := ResolveDownloadDir(
		settings,
		job.LibraryType,
	)
	request := DownloadRequest{
		URL:            job.URL,
		OutputDir:      outputDir,
		OutputTemplate: OutputTemplate(outputDir),
		Bitrate:        job.Bitrate,
		FFmpegPath:     settings.FFmpegPath,
	}

	metadata := Metadata{
		ID:        job.ExternalID,
		Extractor: job.SourceSite,
		Title:     job.Title,
		Uploader:  job.Uploader,
		Duration:  job.Duration,
	}

	workerLog.I(
		"download start job=%s source=%s externalID=%s",
		job.ID,
		job.SourceSite,
		job.ExternalID,
	)

	err := client.Download(
		ctx,
		request,
		DownloadCallbacks{
			OnLine: func(line string) {
				workerLog.D("job=%s %s", job.ID, line)
			},
			OnProgress: func(progress float64) {
				current, loadErr :=
					w.store.ExternalDownloadJob(job.ID)
				if loadErr != nil {
					return
				}

				mapped := 0.03 + progress*0.82
				_ = w.update(
					current,
					StatusDownloading,
					mapped,
					"",
				)
			},
			OnConverting: func() {
				current, loadErr :=
					w.store.ExternalDownloadJob(job.ID)
				if loadErr != nil {
					return
				}
				_ = w.update(
					current,
					StatusConverting,
					maxFloat(current.Progress, 0.88),
					"",
				)
			},
		},
	)
	if err != nil {
		return err
	}

	outputPath, err := FindOutputMP3(
		outputDir,
		metadata,
	)
	if err != nil {
		return err
	}

	if err := w.finalize(job, outputPath); err != nil {
		return err
	}
	return nil
}

func (w *Worker) finalize(
	job Job,
	outputPath string,
) error {
	switch job.LibraryType {
	case LibraryMusic:
		if err := w.store.MarkExternalTrackDownloaded(
			job.ItemID,
			outputPath,
		); err != nil {
			return err
		}
		if w.hooks.OnMusicReady != nil {
			if err := w.hooks.OnMusicReady(
				job.ItemID,
				outputPath,
			); err != nil {
				return fmt.Errorf(
					"enqueue music analysis: %w",
					err,
				)
			}
		}

	case LibraryPodcast:
		if err := w.store.MarkExternalPodcastDownloaded(
			job.ItemID,
			outputPath,
		); err != nil {
			return err
		}
		if w.hooks.OnPodcastReady != nil {
			if err := w.hooks.OnPodcastReady(
				job.ItemID,
				outputPath,
			); err != nil {
				return err
			}
		}
	}

	if err := w.store.MarkExternalDownloadReady(
		job.ID,
		outputPath,
	); err != nil {
		return err
	}

	job, _ = w.store.ExternalDownloadJob(job.ID)
	w.emit("external-download:done", job)
	w.notifyLibraryChanged()

	workerLog.I(
		"download ready job=%s item=%s path=%q",
		job.ID,
		job.ItemID,
		outputPath,
	)
	return nil
}

func (w *Worker) update(
	job Job,
	status Status,
	progress float64,
	errorText string,
) error {
	if err := w.store.UpdateExternalDownloadJob(
		job.ID,
		status,
		progress,
		errorText,
	); err != nil {
		return err
	}

	latest, err := w.store.ExternalDownloadJob(job.ID)
	if err != nil {
		return err
	}

	if err := w.store.UpdateExternalItemDownloadState(
		latest.LibraryType,
		latest.ItemID,
		status,
		progress,
		errorText,
		latest.Attempts,
	); err != nil {
		return err
	}

	w.emit("external-download:update", latest)
	w.notifyLibraryChanged()
	return nil
}

func (w *Worker) emit(event string, job Job) {
	if w.hooks.Emit != nil {
		w.hooks.Emit(event, job.DTO())
	}
}

func (w *Worker) notifyLibraryChanged() {
	if w.hooks.OnLibraryChanged != nil {
		w.hooks.OnLibraryChanged()
	}
}

func RemoveOutputArtifacts(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + ".info.json")

	base := strings.TrimSuffix(
		path,
		filepath.Ext(path),
	)
	matches, _ := filepath.Glob(base + ".*.part")
	for _, match := range matches {
		_ = os.Remove(match)
	}
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
