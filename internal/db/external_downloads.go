package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ray-player1/internal/externalmedia"
)

func (s *Store) InsertExternalDownloadJob(
	job externalmedia.Job,
) error {
	_, err := s.db.Exec(`
		INSERT INTO external_download_jobs (
			id,
			library_type,
			item_id,
			url,
			source_site,
			external_id,
			status,
			progress,
			title,
			uploader,
			duration,
			thumbnail_url,
			output_path,
			temp_path,
			bitrate,
			attempts,
			max_attempts,
			error,
			created_at,
			updated_at,
			started_at,
			finished_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		job.ID,
		job.LibraryType,
		job.ItemID,
		job.URL,
		job.SourceSite,
		job.ExternalID,
		job.Status,
		job.Progress,
		job.Title,
		job.Uploader,
		job.Duration,
		job.ThumbnailURL,
		job.OutputPath,
		job.TempPath,
		job.Bitrate,
		job.Attempts,
		job.MaxAttempts,
		job.Error,
		job.CreatedAt,
		job.UpdatedAt,
		job.StartedAt,
		job.FinishedAt,
	)
	return err
}

func (s *Store) ExternalDownloadJob(
	id string,
) (externalmedia.Job, error) {
	var job externalmedia.Job
	err := s.db.QueryRow(`
		SELECT
			id,
			library_type,
			item_id,
			url,
			source_site,
			external_id,
			status,
			progress,
			title,
			uploader,
			duration,
			thumbnail_url,
			output_path,
			temp_path,
			bitrate,
			attempts,
			max_attempts,
			error,
			created_at,
			updated_at,
			started_at,
			finished_at
		FROM external_download_jobs
		WHERE id = ?
	`, id).Scan(
		&job.ID,
		&job.LibraryType,
		&job.ItemID,
		&job.URL,
		&job.SourceSite,
		&job.ExternalID,
		&job.Status,
		&job.Progress,
		&job.Title,
		&job.Uploader,
		&job.Duration,
		&job.ThumbnailURL,
		&job.OutputPath,
		&job.TempPath,
		&job.Bitrate,
		&job.Attempts,
		&job.MaxAttempts,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.StartedAt,
		&job.FinishedAt,
	)
	return job, err
}

func (s *Store) QueuedExternalDownloadJobs() (
	[]externalmedia.Job,
	error,
) {
	rows, err := s.db.Query(`
		SELECT
			id,
			library_type,
			item_id,
			url,
			source_site,
			external_id,
			status,
			progress,
			title,
			uploader,
			duration,
			thumbnail_url,
			output_path,
			temp_path,
			bitrate,
			attempts,
			max_attempts,
			error,
			created_at,
			updated_at,
			started_at,
			finished_at
		FROM external_download_jobs
		WHERE status = 'queued'
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []externalmedia.Job
	for rows.Next() {
		var job externalmedia.Job
		if err := rows.Scan(
			&job.ID,
			&job.LibraryType,
			&job.ItemID,
			&job.URL,
			&job.SourceSite,
			&job.ExternalID,
			&job.Status,
			&job.Progress,
			&job.Title,
			&job.Uploader,
			&job.Duration,
			&job.ThumbnailURL,
			&job.OutputPath,
			&job.TempPath,
			&job.Bitrate,
			&job.Attempts,
			&job.MaxAttempts,
			&job.Error,
			&job.CreatedAt,
			&job.UpdatedAt,
			&job.StartedAt,
			&job.FinishedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) UpdateExternalDownloadJob(
	id string,
	status externalmedia.Status,
	progress float64,
	errText string,
) error {
	_, err := s.db.Exec(`
		UPDATE external_download_jobs
		SET status = ?,
		    progress = ?,
		    error = ?,
		    updated_at = unixepoch(),
		    started_at = CASE
		        WHEN started_at = 0
		          AND ? IN ('downloading', 'converting')
		        THEN unixepoch()
		        ELSE started_at
		    END,
		    finished_at = CASE
		        WHEN ? IN ('ready', 'error', 'canceled')
		        THEN unixepoch()
		        ELSE finished_at
		    END
		WHERE id = ?
	`,
		status,
		progress,
		errText,
		status,
		status,
		id,
	)
	return err
}

func (s *Store) IncrementExternalDownloadAttempt(
	id string,
) error {
	_, err := s.db.Exec(`
		UPDATE external_download_jobs
		SET attempts = attempts + 1,
		    updated_at = unixepoch()
		WHERE id = ?
	`, id)
	return err
}

func (s *Store) MarkExternalDownloadReady(
	id string,
	outputPath string,
) error {
	_, err := s.db.Exec(`
		UPDATE external_download_jobs
		SET status = 'ready',
		    progress = 1,
		    output_path = ?,
		    error = '',
		    updated_at = unixepoch(),
		    finished_at = unixepoch()
		WHERE id = ?
	`, outputPath, id)
	return err
}

func (s *Store) ExistingExternalItem(
	libraryType externalmedia.LibraryType,
	sourceURL string,
	sourceSite string,
	externalID string,
) (string, string, error) {
	table := "tracks"
	if libraryType == externalmedia.LibraryPodcast {
		table = "podcast_items"
	}

	var itemID, status string
	query := fmt.Sprintf(`
		SELECT id, download_status
		FROM %s
		WHERE source_type = 'yt_dlp'
		  AND (
		    source_url = ?
		    OR (
		      source_site = ?
		      AND external_id = ?
		      AND external_id != ''
		    )
		  )
		LIMIT 1
	`, table)

	err := s.db.QueryRow(
		query,
		sourceURL,
		sourceSite,
		externalID,
	).Scan(&itemID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return itemID, status, err
}

func (s *Store) InsertPendingExternalTrack(
	itemID string,
	path string,
	title string,
	artist string,
	durationSeconds float64,
	sourceURL string,
	sourceSite string,
	externalID string,
) error {
	durationMs := int(durationSeconds * 1000)
	now := time.Now().Unix()

	_, err := s.db.Exec(`
		INSERT INTO tracks (
			id,
			path,
			normalized_path,
			title,
			artist,
			album,
			genre,
			duration_ms,
			duration_label,
			folder,
			file_name,
			import_status,
			analysis_status,
			file_missing,
			source_type,
			source_url,
			source_site,
			external_id,
			download_status,
			download_progress,
			download_error,
			download_attempts,
			downloaded_at,
			created_at,
			updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		itemID,
		path,
		path,
		title,
		artist,
		"",
		"",
		durationMs,
		formatDurationMs(durationMs),
		filepath.Dir(path),
		filepath.Base(path),
		"pending_external",
		"pending",
		1,
		"yt_dlp",
		sourceURL,
		sourceSite,
		externalID,
		"queued",
		0,
		"",
		0,
		0,
		now,
		now,
	)
	return err
}

func (s *Store) InsertPendingExternalPodcast(
	itemID string,
	path string,
	title string,
	author string,
	durationSeconds float64,
	sourceURL string,
	sourceSite string,
	externalID string,
) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO podcast_items (
			id,
			path,
			title,
			author,
			folder,
			duration,
			file_size,
			added_at,
			updated_at,
			modified_at,
			semantic_status,
			source_type,
			source_url,
			source_site,
			external_id,
			download_status,
			download_progress,
			download_error,
			download_attempts,
			downloaded_at
		) VALUES (
			?, ?, ?, ?, ?, ?, 0, ?, ?, 0,
			'pending',
			'yt_dlp',
			?, ?, ?,
			'queued',
			0,
			'',
			0,
			0
		)
	`,
		itemID,
		path,
		title,
		author,
		filepath.Dir(path),
		durationSeconds,
		now,
		now,
		sourceURL,
		sourceSite,
		externalID,
	)
	return err
}

func (s *Store) UpdateExternalItemDownloadState(
	libraryType externalmedia.LibraryType,
	itemID string,
	status externalmedia.Status,
	progress float64,
	errorText string,
	attempts int,
) error {
	table := "tracks"
	if libraryType == externalmedia.LibraryPodcast {
		table = "podcast_items"
	}

	query := fmt.Sprintf(`
		UPDATE %s
		SET download_status = ?,
		    download_progress = ?,
		    download_error = ?,
		    download_attempts = ?,
		    updated_at = unixepoch()
		WHERE id = ?
	`, table)

	_, err := s.db.Exec(
		query,
		status,
		progress,
		errorText,
		attempts,
		itemID,
	)
	return err
}

func (s *Store) MarkExternalTrackDownloaded(
	itemID string,
	path string,
) error {
	info, err := osStat(path)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE tracks
		SET path = ?,
		    normalized_path = ?,
		    folder = ?,
		    file_name = ?,
		    file_size = ?,
		    file_mtime = ?,
		    file_missing = 0,
		    import_status = 'ready',
		    analysis_status = 'pending',
		    download_status = 'ready',
		    download_progress = 1,
		    download_error = '',
		    downloaded_at = unixepoch(),
		    updated_at = unixepoch()
		WHERE id = ?
	`,
		path,
		path,
		filepath.Dir(path),
		filepath.Base(path),
		info.Size(),
		info.ModTime().Unix(),
		itemID,
	)
	return err
}

func (s *Store) MarkExternalPodcastDownloaded(
	itemID string,
	path string,
) error {
	info, err := osStat(path)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE podcast_items
		SET path = ?,
		    folder = ?,
		    file_size = ?,
		    modified_at = ?,
		    semantic_status = 'metadata_ready',
		    download_status = 'ready',
		    download_progress = 1,
		    download_error = '',
		    downloaded_at = unixepoch(),
		    updated_at = unixepoch()
		WHERE id = ?
	`,
		path,
		filepath.Dir(path),
		info.Size(),
		info.ModTime().Unix(),
		itemID,
	)
	return err
}

func (s *Store) DeleteExternalItem(
	libraryType externalmedia.LibraryType,
	itemID string,
) error {
	table := "tracks"
	if libraryType == externalmedia.LibraryPodcast {
		table = "podcast_items"
	}
	_, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE id = ?", table),
		itemID,
	)
	return err
}

func formatDurationMs(durationMs int) string {
	totalSeconds := durationMs / 1000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf(
			"%d:%02d:%02d",
			hours,
			minutes,
			seconds,
		)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func (s *Store) ListExternalDownloadJobs(limit int) ([]externalmedia.Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT
			id, library_type, item_id, url, source_site, external_id,
			status, progress, title, uploader, duration, thumbnail_url,
			output_path, temp_path, bitrate, attempts, max_attempts,
			error, created_at, updated_at, started_at, finished_at
		FROM external_download_jobs
		ORDER BY
			CASE status
				WHEN 'downloading' THEN 0
				WHEN 'converting' THEN 1
				WHEN 'queued' THEN 2
				WHEN 'error' THEN 3
				ELSE 4
			END,
			updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]externalmedia.Job, 0, limit)
	for rows.Next() {
		var job externalmedia.Job
		if err := rows.Scan(
			&job.ID, &job.LibraryType, &job.ItemID, &job.URL,
			&job.SourceSite, &job.ExternalID, &job.Status, &job.Progress,
			&job.Title, &job.Uploader, &job.Duration, &job.ThumbnailURL,
			&job.OutputPath, &job.TempPath, &job.Bitrate, &job.Attempts,
			&job.MaxAttempts, &job.Error, &job.CreatedAt, &job.UpdatedAt,
			&job.StartedAt, &job.FinishedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func osStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
