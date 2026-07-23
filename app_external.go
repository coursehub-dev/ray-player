package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ray-player1/internal/externalmedia"
)

const (
	metaYtDlpPath        = "external.ytdlp_path"
	metaYtDlpDownloadDir = "external.ytdlp_download_dir"
	metaFFmpegPath       = "external.ffmpeg_path"
)

func (a *App) GetExternalMediaSettings() externalmedia.Settings {
	settings := externalmedia.Settings{
		YtDlpPath: "yt-dlp",
	}

	if value, err := a.store.GetMeta(metaYtDlpPath); err == nil &&
		strings.TrimSpace(value) != "" {
		settings.YtDlpPath = value
	}
	if value, err := a.store.GetMeta(metaYtDlpDownloadDir); err == nil {
		settings.YtDlpDownloadDir = value
	}
	if value, err := a.store.GetMeta(metaFFmpegPath); err == nil {
		settings.FFmpegPath = value
	}

	return settings
}

func (a *App) SaveExternalMediaSettings(
	settings externalmedia.Settings,
) error {
	if err := a.store.SetMeta(
		metaYtDlpPath,
		strings.TrimSpace(settings.YtDlpPath),
	); err != nil {
		return err
	}
	if err := a.store.SetMeta(
		metaYtDlpDownloadDir,
		strings.TrimSpace(settings.YtDlpDownloadDir),
	); err != nil {
		return err
	}
	return a.store.SetMeta(
		metaFFmpegPath,
		strings.TrimSpace(settings.FFmpegPath),
	)
}

func (a *App) TestYtDlp(
	path string,
) externalmedia.ToolCheckResult {
	return externalmedia.NewClient(path).Test(context.Background())
}

func (a *App) AddExternalLink(
	rawURL string,
	libraryType string,
) (externalmedia.JobDTO, error) {
	rawURL = strings.TrimSpace(rawURL)
	if err := externalmedia.ValidateURL(rawURL); err != nil {
		return externalmedia.JobDTO{}, err
	}

	kind, err := externalmedia.NormalizeLibraryType(
		libraryType,
	)
	if err != nil {
		return externalmedia.JobDTO{}, err
	}

	settings := a.GetExternalMediaSettings()
	client := externalmedia.NewClient(settings.YtDlpPath)
	check := client.Test(context.Background())
	if !check.OK {
		return externalmedia.JobDTO{}, fmt.Errorf(
			"yt-dlp недоступен: %s",
			check.Error,
		)
	}

	metadata, err := client.FetchMetadata(
		context.Background(),
		rawURL,
	)
	if err != nil {
		return externalmedia.JobDTO{}, err
	}

	existingID, existingStatus, err :=
		a.store.ExistingExternalItem(
			kind,
			rawURL,
			metadata.Extractor,
			metadata.ID,
		)
	if err != nil {
		return externalmedia.JobDTO{}, err
	}
	if existingID != "" {
		return externalmedia.JobDTO{}, fmt.Errorf(
			"элемент уже есть в библиотеке: %s (%s)",
			existingID,
			existingStatus,
		)
	}

	outputDir := externalmedia.ResolveDownloadDir(
		settings,
		kind,
	)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return externalmedia.JobDTO{}, err
	}

	outputPath := filepath.Join(
		outputDir,
		externalmedia.OutputFilename(metadata),
	)

	itemID := externalItemID(
		kind,
		metadata.Extractor,
		metadata.ID,
		rawURL,
	)
	jobID := fmt.Sprintf(
		"external-job-%d",
		time.Now().UnixNano(),
	)

	switch kind {
	case externalmedia.LibraryMusic:
		err = a.store.InsertPendingExternalTrack(
			itemID,
			outputPath,
			metadata.Title,
			metadata.Author(),
			metadata.Duration,
			rawURL,
			metadata.Extractor,
			metadata.ID,
		)

	case externalmedia.LibraryPodcast:
		err = a.store.InsertPendingExternalPodcast(
			itemID,
			outputPath,
			metadata.Title,
			metadata.Author(),
			metadata.Duration,
			rawURL,
			metadata.Extractor,
			metadata.ID,
		)
	}
	if err != nil {
		return externalmedia.JobDTO{}, err
	}

	now := time.Now().Unix()
	job := externalmedia.Job{
		ID:           jobID,
		LibraryType:  kind,
		ItemID:       itemID,
		URL:          rawURL,
		SourceSite:   metadata.Extractor,
		ExternalID:   metadata.ID,
		Status:       externalmedia.StatusQueued,
		Progress:     0,
		Title:        metadata.Title,
		Uploader:     metadata.Author(),
		Duration:     metadata.Duration,
		ThumbnailURL: metadata.Thumbnail,
		OutputPath:   outputPath,
		Bitrate:      externalmedia.BitrateFor(kind),
		MaxAttempts:  3,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := a.store.InsertExternalDownloadJob(job); err != nil {
		_ = a.store.DeleteExternalItem(kind, itemID)
		return externalmedia.JobDTO{}, err
	}

	a.refreshExternalLibraries()
	a.externalDownloads.Enqueue(job.ID)
	a.pushSnapshot()

	return job.DTO(), nil
}

func (a *App) RetryExternalDownload(jobID string) error {
	job, err := a.store.ExternalDownloadJob(jobID)
	if err != nil {
		return err
	}

	if job.Status != externalmedia.StatusError &&
		job.Status != externalmedia.StatusCanceled {
		return fmt.Errorf(
			"job %s cannot be retried from status %s",
			job.ID,
			job.Status,
		)
	}

	if err := a.store.UpdateExternalDownloadJob(
		job.ID,
		externalmedia.StatusQueued,
		0,
		"",
	); err != nil {
		return err
	}

	a.externalDownloads.Enqueue(job.ID)
	return nil
}

func (a *App) CancelExternalDownload(jobID string) error {
	return a.externalDownloads.Cancel(jobID)
}

func (a *App) DeleteExternalItem(
	itemID string,
	libraryType string,
	deleteFile bool,
) error {
	kind, err := externalmedia.NormalizeLibraryType(
		libraryType,
	)
	if err != nil {
		return err
	}

	var path string
	switch kind {
	case externalmedia.LibraryMusic:
		row, getErr := a.store.GetTrack(itemID)
		if getErr != nil {
			return getErr
		}
		path = row.Path

	case externalmedia.LibraryPodcast:
		row, getErr := a.store.PodcastItemByID(itemID)
		if getErr != nil {
			return getErr
		}
		path = row.Path
	}

	if err := a.store.DeleteExternalItem(kind, itemID); err != nil {
		return err
	}
	if deleteFile {
		externalmedia.RemoveOutputArtifacts(path)
	}

	a.refreshExternalLibraries()
	a.pushSnapshot()
	return nil
}

func (a *App) OpenExternalSource(rawURL string) error {
	if err := externalmedia.ValidateURL(rawURL); err != nil {
		return err
	}
	runtime.BrowserOpenURL(a.ctx, rawURL)
	return nil
}

func (a *App) refreshExternalLibraries() {
	_ = a.library.ReloadFromStore()
}

func externalItemID(
	kind externalmedia.LibraryType,
	site string,
	externalID string,
	rawURL string,
) string {
	sum := sha256.Sum256([]byte(
		string(kind) + "\x00" +
			site + "\x00" +
			externalID + "\x00" +
			rawURL,
	))
	prefix := "external-track-"
	if kind == externalmedia.LibraryPodcast {
		prefix = "external-podcast-"
	}
	return prefix + hex.EncodeToString(sum[:12])
}

var errExternalWorkerUnavailable = errors.New(
	"external download worker is unavailable",
)
