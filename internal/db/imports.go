package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const trackSelectColumns = `id, path, title, artist, album, genre, genre_primary, genre_detail, genre_tags, genre_label, duration_ms, duration_label, folder, file_name, tempo, bpm_perceived, tempo_confidence, tempo_stability, bpm_half, bpm_double, tempo_source, tempo_model_version, tempo_analyzed_at, tempo_error, energy, danceability, valence, acousticness, electronicness, instrumentalness, vocalness, happy, sad, relaxed, party, aggressive, timbre_brightness, tonality, approachability, engagement, melodicness, softness, heaviness, dreaminess, emotionality, loudness, spectral_centroid, zero_crossing_rate, rms, spectral_flatness, spectral_rolloff85, spectral_flux, onset_rate, dynamic_range, low_band_ratio, mid_band_ratio, high_band_ratio, cluster_id, play_count, skip_count, complete_count, metadata_source, analyzed_level, analysis_version, analyzed_at, analysis_error, essentia_model_version, normalized_path, library_root_id, import_status, analysis_status, file_missing, file_size, file_mtime, file_inode, quick_hash, last_seen_at, last_error, embedding, text_embedding, playback_error_count, last_playback_error, last_playback_error_at, created_at,
	source_type, source_url, source_site, external_id,
	download_status, download_progress, download_error, download_attempts, downloaded_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanTrackRow(scanner rowScanner) (TrackRow, error) {
	return scanTrackRowTail(scanner)
}

func scanTrackRowTail(scanner rowScanner, extra ...any) (TrackRow, error) {
	var t TrackRow
	var emb, textEmb []byte
	var fileMissing int
	dest := []any{&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.Genre, &t.GenrePrimary, &t.GenreDetail, &t.GenreTagsJSON, &t.GenreLabel, &t.DurationMs, &t.DurationLabel, &t.Folder, &t.FileName, &t.Tempo, &t.BPMPerceived, &t.TempoConfidence, &t.TempoStability, &t.BPMHalf, &t.BPMDouble, &t.TempoSource, &t.TempoModelVersion, &t.TempoAnalyzedAt, &t.TempoError, &t.Energy, &t.Danceability, &t.Valence, &t.Acousticness, &t.Electronicness, &t.Instrumentalness, &t.Vocalness, &t.Happy, &t.Sad, &t.Relaxed, &t.Party, &t.Aggressive, &t.TimbreBrightness, &t.Tonality, &t.Approachability, &t.Engagement, &t.Melodicness, &t.Softness, &t.Heaviness, &t.Dreaminess, &t.Emotionality, &t.Loudness, &t.SpectralCentroid, &t.ZeroCrossingRate, &t.RMS, &t.SpectralFlatness, &t.SpectralRolloff85, &t.SpectralFlux, &t.OnsetRate, &t.DynamicRange, &t.LowBandRatio, &t.MidBandRatio, &t.HighBandRatio, &t.ClusterID, &t.PlayCount, &t.SkipCount, &t.CompleteCount, &t.MetadataSource, &t.AnalyzedLevel, &t.AnalysisVersion, &t.AnalyzedAt, &t.AnalysisError, &t.EssentiaModelVersion, &t.NormalizedPath, &t.LibraryRootID, &t.ImportStatus, &t.AnalysisStatus, &fileMissing, &t.FileSize, &t.FileMTime, &t.FileInode, &t.QuickHash, &t.LastSeenAt, &t.LastError, &emb, &textEmb, &t.PlaybackErrorCount, &t.LastPlaybackError, &t.LastPlaybackErrorAt, &t.AddedAt,
		&t.SourceType, &t.SourceURL, &t.SourceSite, &t.ExternalID,
		&t.DownloadStatus, &t.DownloadProgress, &t.DownloadError, &t.DownloadAttempts, &t.DownloadedAt}
	dest = append(dest, extra...)
	if err := scanner.Scan(dest...); err != nil {
		return TrackRow{}, err
	}
	t.FileMissing = fileMissing != 0
	t.Embedding = bytesToFloats(emb)
	t.TextEmbedding = bytesToFloats(textEmb)
	return t, nil
}

func scanTrackRowWithScore(scanner rowScanner) (TrackRow, float64, error) {
	var score float64
	track, err := scanTrackRowTail(scanner, &score)
	return track, score, err
}

func scanTrackRowWithHits(scanner rowScanner) (TrackRow, int, error) {
	var hits int
	track, err := scanTrackRowTail(scanner, &hits)
	return track, hits, err
}

func prefixColumns(prefix string) string {
	parts := strings.Split(trackSelectColumns, ", ")
	for i, part := range parts {
		parts[i] = prefix + part
	}
	return strings.Join(parts, ", ")
}

func prefixListColumns(columns string, prefix string) string {
	parts := strings.Split(columns, ",")
	for i, part := range parts {
		parts[i] = prefix + strings.TrimSpace(part)
	}
	return strings.Join(parts, ", ")
}

func (s *Store) ListLibraryRoots() ([]LibraryRootRow, error) {
	rows, err := s.db.Query(`SELECT id, path, library_type, enabled, recursive, last_scan_started_at, last_scan_finished_at, last_scan_error, created_at, updated_at FROM library_roots ORDER BY path ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LibraryRootRow{}
	for rows.Next() {
		var row LibraryRootRow
		var enabled, recursive int
		if err := rows.Scan(&row.ID, &row.Path, &row.LibraryType, &enabled, &recursive, &row.LastScanStartedAt, &row.LastScanFinishedAt, &row.LastScanError, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.Enabled = enabled != 0
		row.Recursive = recursive != 0
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) UpsertLibraryRoot(row LibraryRootRow) error {
	_, err := s.db.Exec(`INSERT INTO library_roots(id, path, library_type, enabled, recursive, last_scan_started_at, last_scan_finished_at, last_scan_error, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,COALESCE(NULLIF(?,0),unixepoch()),unixepoch())
ON CONFLICT(id) DO UPDATE SET path=excluded.path, library_type=excluded.library_type, enabled=excluded.enabled, recursive=excluded.recursive, last_scan_started_at=excluded.last_scan_started_at, last_scan_finished_at=excluded.last_scan_finished_at, last_scan_error=excluded.last_scan_error, updated_at=unixepoch()`, row.ID, row.Path, row.LibraryType, boolInt(row.Enabled), boolInt(row.Recursive), row.LastScanStartedAt, row.LastScanFinishedAt, row.LastScanError, row.CreatedAt)
	return err
}

func (s *Store) StartImportSession(row ImportSessionRow) error {
	_, err := s.db.Exec(`INSERT INTO import_sessions(id, library_root_id, library_type, status, started_at, finished_at, scanned_count, audio_count, new_count, updated_count, unchanged_count, skipped_count, error_count, last_error)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, row.ID, row.LibraryRootID, row.LibraryType, row.Status, row.StartedAt, row.FinishedAt, row.ScannedCount, row.AudioCount, row.NewCount, row.UpdatedCount, row.UnchangedCount, row.SkippedCount, row.ErrorCount, row.LastError)
	return err
}

func (s *Store) UpdateImportSession(row ImportSessionRow) error {
	_, err := s.db.Exec(`UPDATE import_sessions SET status=?, finished_at=?, scanned_count=?, audio_count=?, new_count=?, updated_count=?, unchanged_count=?, skipped_count=?, error_count=?, last_error=? WHERE id=?`, row.Status, row.FinishedAt, row.ScannedCount, row.AudioCount, row.NewCount, row.UpdatedCount, row.UnchangedCount, row.SkippedCount, row.ErrorCount, row.LastError, row.ID)
	return err
}

func (s *Store) AddFileError(row FileErrorRow) error {
	if row.CreatedAt == 0 {
		row.CreatedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`INSERT INTO file_errors(id, track_id, path, library_type, stage, kind, message, created_at) VALUES(?,?,?,?,?,?,?,?)`, row.ID, row.TrackID, row.Path, row.LibraryType, row.Stage, row.Kind, row.Message, row.CreatedAt)
	return err
}

func (s *Store) ListFileErrors(limit int) ([]FileErrorRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, track_id, path, library_type, stage, kind, message, created_at FROM file_errors ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FileErrorRow{}
	for rows.Next() {
		var row FileErrorRow
		if err := rows.Scan(&row.ID, &row.TrackID, &row.Path, &row.LibraryType, &row.Stage, &row.Kind, &row.Message, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ClearFileErrorsForTrack(trackID string) error {
	_, err := s.db.Exec(`DELETE FROM file_errors WHERE track_id = ?`, trackID)
	return err
}

func (s *Store) GetTrackByNormalizedPath(normalizedPath string) (TrackRow, error) {
	if track, err := s.getTrackByPathColumn("normalized_path", normalizedPath); err == nil {
		return track, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return TrackRow{}, err
	}
	return s.getTrackByPathColumn("path", normalizedPath)
}

func (s *Store) getTrackByPathColumn(column string, value string) (TrackRow, error) {
	var t TrackRow
	var emb, textEmb []byte
	var fileMissing int
	query := fmt.Sprintf(`SELECT id, path, title, artist, album, genre, genre_primary, genre_detail, genre_tags, genre_label, duration_ms, duration_label, folder, file_name, tempo, bpm_perceived, tempo_confidence, tempo_stability, bpm_half, bpm_double, tempo_source, tempo_model_version, tempo_analyzed_at, tempo_error, energy, danceability, valence, acousticness, electronicness, instrumentalness, vocalness, happy, sad, relaxed, party, aggressive, timbre_brightness, tonality, approachability, engagement, melodicness, softness, heaviness, dreaminess, emotionality, loudness, spectral_centroid, zero_crossing_rate, rms, spectral_flatness, spectral_rolloff85, spectral_flux, onset_rate, dynamic_range, low_band_ratio, mid_band_ratio, high_band_ratio, cluster_id, play_count, skip_count, complete_count, metadata_source, analyzed_level, analysis_version, analyzed_at, analysis_error, essentia_model_version, normalized_path, library_root_id, import_status, analysis_status, file_missing, file_size, file_mtime, file_inode, quick_hash, last_seen_at, last_error, embedding, text_embedding, playback_error_count, last_playback_error, last_playback_error_at FROM tracks WHERE %s = ? LIMIT 1`, column)
	err := s.db.QueryRow(query, value).Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.Genre, &t.GenrePrimary, &t.GenreDetail, &t.GenreTagsJSON, &t.GenreLabel, &t.DurationMs, &t.DurationLabel, &t.Folder, &t.FileName, &t.Tempo, &t.BPMPerceived, &t.TempoConfidence, &t.TempoStability, &t.BPMHalf, &t.BPMDouble, &t.TempoSource, &t.TempoModelVersion, &t.TempoAnalyzedAt, &t.TempoError, &t.Energy, &t.Danceability, &t.Valence, &t.Acousticness, &t.Electronicness, &t.Instrumentalness, &t.Vocalness, &t.Happy, &t.Sad, &t.Relaxed, &t.Party, &t.Aggressive, &t.TimbreBrightness, &t.Tonality, &t.Approachability, &t.Engagement, &t.Melodicness, &t.Softness, &t.Heaviness, &t.Dreaminess, &t.Emotionality, &t.Loudness, &t.SpectralCentroid, &t.ZeroCrossingRate, &t.RMS, &t.SpectralFlatness, &t.SpectralRolloff85, &t.SpectralFlux, &t.OnsetRate, &t.DynamicRange, &t.LowBandRatio, &t.MidBandRatio, &t.HighBandRatio, &t.ClusterID, &t.PlayCount, &t.SkipCount, &t.CompleteCount, &t.MetadataSource, &t.AnalyzedLevel, &t.AnalysisVersion, &t.AnalyzedAt, &t.AnalysisError, &t.EssentiaModelVersion, &t.NormalizedPath, &t.LibraryRootID, &t.ImportStatus, &t.AnalysisStatus, &fileMissing, &t.FileSize, &t.FileMTime, &t.FileInode, &t.QuickHash, &t.LastSeenAt, &t.LastError, &emb, &textEmb, &t.PlaybackErrorCount, &t.LastPlaybackError, &t.LastPlaybackErrorAt)
	if err != nil {
		return TrackRow{}, err
	}
	t.FileMissing = fileMissing != 0
	t.Embedding = bytesToFloats(emb)
	t.TextEmbedding = bytesToFloats(textEmb)
	return t, nil
}

func (s *Store) MarkTrackAnalysisStatus(trackID, status, lastError string, analyzedLevel int) error {
	_, err := s.db.Exec(`UPDATE tracks SET analysis_status = ?, last_error = ?, analysis_error = ?, analyzed_level = ?, updated_at = unixepoch() WHERE id = ?`, status, lastError, lastError, analyzedLevel, trackID)
	return err
}

func (s *Store) MarkTrackReady(trackID string) error {
	_, err := s.db.Exec(`UPDATE tracks SET import_status = 'ready', analysis_status = 'done', file_missing = 0, last_error = '', analysis_error = '', updated_at = unixepoch() WHERE id = ?`, trackID)
	return err
}

func (s *Store) MarkTrackMissing(trackID string) error {
	_, err := s.db.Exec(`UPDATE tracks SET import_status = 'missing', file_missing = 1, updated_at = unixepoch() WHERE id = ?`, trackID)
	return err
}

func (s *Store) MarkMissingTracksForRoot(rootID string, seenAt int64) error {
	_, err := s.db.Exec(`UPDATE tracks SET import_status = 'missing', file_missing = 1, updated_at = unixepoch() WHERE library_root_id = ? AND last_seen_at < ?`, rootID, seenAt)
	return err
}

func (s *Store) RemoveTrack(trackID string) error {
	_, err := s.db.Exec(`DELETE FROM tracks WHERE id = ?`, trackID)
	return err
}

func (s *Store) UpdateTrackPathsAndStats(trackID, path, normalizedPath, rootID, importStatus, analysisStatus string, fileMissing bool, fileSize, fileMTime int64, fileInode, quickHash string, lastSeenAt int64, lastError string) error {
	_, err := s.db.Exec(`UPDATE tracks SET path = ?, normalized_path = ?, library_root_id = ?, import_status = ?, analysis_status = ?, file_missing = ?, file_size = ?, file_mtime = ?, file_inode = ?, quick_hash = ?, last_seen_at = ?, last_error = ?, updated_at = unixepoch() WHERE id = ?`, path, normalizedPath, rootID, importStatus, analysisStatus, boolInt(fileMissing), fileSize, fileMTime, fileInode, quickHash, lastSeenAt, lastError, trackID)
	return err
}

func (s *Store) LookupLibraryRootByPath(path string) (LibraryRootRow, error) {
	var row LibraryRootRow
	var enabled, recursive int
	err := s.db.QueryRow(`SELECT id, path, library_type, enabled, recursive, last_scan_started_at, last_scan_finished_at, last_scan_error, created_at, updated_at FROM library_roots WHERE path = ? LIMIT 1`, path).Scan(&row.ID, &row.Path, &row.LibraryType, &enabled, &recursive, &row.LastScanStartedAt, &row.LastScanFinishedAt, &row.LastScanError, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return LibraryRootRow{}, err
	}
	row.Enabled = enabled != 0
	row.Recursive = recursive != 0
	return row, nil
}

func (s *Store) TrackExists(trackID string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM tracks WHERE id = ?`, trackID).Scan(&n)
	return n > 0, err
}

func isNotFound(err error) bool {
	return err == sql.ErrNoRows
}

func (s *Store) UpdateLibraryRootScan(rootID string, startedAt, finishedAt int64, scanErr string) error {
	_, err := s.db.Exec(`UPDATE library_roots SET last_scan_started_at = ?, last_scan_finished_at = ?, last_scan_error = ?, updated_at = unixepoch() WHERE id = ?`, startedAt, finishedAt, scanErr, rootID)
	return err
}

func (s *Store) CountTrackErrors() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM file_errors`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) DebugTrackStatuses() (string, error) {
	var ready, missing, errored int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM tracks WHERE import_status = 'ready'`).Scan(&ready); err != nil {
		return "", err
	}
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM tracks WHERE import_status = 'missing'`).Scan(&missing); err != nil {
		return "", err
	}
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM tracks WHERE analysis_status = 'error'`).Scan(&errored); err != nil {
		return "", err
	}
	return fmt.Sprintf("ready=%d missing=%d analysis_error=%d", ready, missing, errored), nil
}
