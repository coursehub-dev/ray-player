package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PodcastItemRow struct {
	ID               string
	Path             string
	Title            string
	Author           string
	Series           string
	Folder           string
	Duration         float64
	FileSize         int64
	AddedAt          int64
	UpdatedAt        int64
	ModifiedAt       int64
	TranscriptPath   string
	TranscriptStatus string
	SemanticStatus   string
	Summary          string
	TopicsJSON       string
	KeywordsJSON     string
	LastPosition     float64
	CompletedRatio   float64
	IsCompleted      bool
	PlayCount        int
	SkipCount        int
	LastPlayedAt     int64
	LastError        string

	SourceType       string
	SourceURL        string
	SourceSite       string
	ExternalID       string
	DownloadStatus   string
	DownloadProgress float64
	DownloadError    string
	DownloadAttempts int
	DownloadedAt     int64
}

const podcastColumns = `
	id,
	path,
	title,
	author,
	series,
	folder,
	duration,
	file_size,
	added_at,
	updated_at,
	modified_at,
	transcript_path,
	transcript_status,
	semantic_status,
	summary,
	topics_json,
	keywords_json,
	last_position,
	completed_ratio,
	is_completed,
	play_count,
	skip_count,
	last_played_at,
	last_error,
	source_type,
	source_url,
	source_site,
	external_id,
	download_status,
	download_progress,
	download_error,
	download_attempts,
	downloaded_at
`

type PodcastRayItemRow struct {
	Item             PodcastItemRow
	PositionIndex    int
	OriginalPosition int
	Reason           string
	Score            float64
	SemanticScore    float64
	FolderScore      float64
	NoveltyScore     float64
	ResumeScore      float64
	AddedBy          string
}

type PodcastRayRow struct {
	ID              string
	SeedItemID      string
	Title           string
	Mode            string
	ContentMode     string
	SortMode        string
	ConfigJSON      string
	IsManualOrder   bool
	ManualUpdatedAt int64
	ParentRayID     string
	Revision        int
	CreatedAt       int64
	FolderScope     string
}

type PodcastHistoryRow struct {
	ID              string
	ItemID          string
	RayID           string
	StartedAt       int64
	EndedAt         int64
	StartPosition   float64
	EndPosition     float64
	ListenedSeconds float64
	CompletedRatio  float64
	Source          string
	EndReason       string
	UpdatedAt       int64
	Item            PodcastItemRow
}

func (s *Store) UpsertPodcastItem(item PodcastItemRow) error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("podcast id is required")
	}
	if strings.TrimSpace(item.Path) == "" {
		return errors.New("podcast path is required")
	}

	now := time.Now().Unix()
	if item.AddedAt == 0 {
		item.AddedAt = now
	}
	if item.UpdatedAt == 0 {
		item.UpdatedAt = now
	}

	_, err := s.db.Exec(`
		INSERT INTO podcast_items (
			id, path, title, author, series, folder,
			duration, file_size, added_at, updated_at, modified_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			id = excluded.id,
			title = excluded.title,
			author = excluded.author,
			series = excluded.series,
			folder = excluded.folder,
			duration = CASE
				WHEN excluded.duration > 0
					THEN excluded.duration
				ELSE podcast_items.duration
			END,
			file_size = excluded.file_size,
			updated_at = excluded.updated_at,
			modified_at = excluded.modified_at,
			last_error = '',
			semantic_status = CASE
				WHEN podcast_items.semantic_status = 'failed' THEN 'metadata_ready'
				ELSE podcast_items.semantic_status
			END
	`,
		item.ID,
		item.Path,
		item.Title,
		item.Author,
		item.Series,
		item.Folder,
		item.Duration,
		item.FileSize,
		item.AddedAt,
		item.UpdatedAt,
		item.ModifiedAt,
	)
	return err
}

func (s *Store) ListPodcastItems(limit int) ([]PodcastItemRow, error) {
	if limit <= 0 {
		limit = 500
	}

	rows, err := s.db.Query(`
		SELECT `+podcastColumns+`
		FROM podcast_items
		ORDER BY
			CASE WHEN completed_ratio > 0 AND is_completed = 0 THEN 0 ELSE 1 END,
			last_played_at DESC,
			folder,
			title
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPodcastRows(rows)
}

func (s *Store) SearchPodcastItems(query string, limit int) ([]PodcastItemRow, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.ListPodcastItems(limit)
	}
	if limit <= 0 {
		limit = 100
	}

	like := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.Query(`
		SELECT `+podcastColumns+`
		FROM podcast_items
		WHERE
			lower(title) LIKE ? OR
			lower(author) LIKE ? OR
			lower(series) LIKE ? OR
			lower(folder) LIKE ? OR
			lower(summary) LIKE ? OR
			lower(keywords_json) LIKE ?
		ORDER BY
			CASE
				WHEN lower(title) = lower(?) THEN 0
				WHEN lower(title) LIKE lower(?) THEN 1
				ELSE 2
			END,
			last_played_at DESC,
			title
		LIMIT ?
	`, like, like, like, like, like, like, query, query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPodcastRows(rows)
}

func (s *Store) PodcastItemByID(id string) (PodcastItemRow, error) {
	row := s.db.QueryRow(`
		SELECT `+podcastColumns+`
		FROM podcast_items
		WHERE id = ?
	`, id)

	return scanPodcastRow(row)
}

func (s *Store) UpdatePodcastProgress(
	id string,
	position float64,
	duration float64,
) (PodcastItemRow, error) {
	if position < 0 {
		position = 0
	}

	ratio := 0.0
	if duration > 0 {
		ratio = position / duration
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	completed := ratio >= 0.95
	_, err := s.db.Exec(`
		UPDATE podcast_items
		SET
			last_position = ?,
			duration = CASE WHEN ? > 0 THEN ? ELSE duration END,
			completed_ratio = ?,
			is_completed = ?,
			last_played_at = ?,
			updated_at = ?
		WHERE id = ?
	`,
		position,
		duration,
		duration,
		ratio,
		func() int {
			if completed {
				return 1
			}
			return 0
		}(),
		time.Now().Unix(),
		time.Now().Unix(),
		id,
	)
	if err != nil {
		return PodcastItemRow{}, err
	}

	return s.PodcastItemByID(id)
}

func (s *Store) IncrementPodcastPlayCount(id string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE podcast_items
		SET
			play_count = play_count + 1,
			last_played_at = ?,
			updated_at = ?
		WHERE id = ?
	`, now, now, id)
	return err
}

func (s *Store) SavePodcastRaySnapshot(
	ray PodcastRayRow,
	items []PodcastRayItemRow,
) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO podcast_rays (
			id,
			seed_item_id,
			title,
			mode,
			content_mode,
			sort_mode,
			config_json,
			is_manual_order,
			manual_updated_at,
			parent_ray_id,
			revision,
			created_at,
			folder_scope
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			seed_item_id = excluded.seed_item_id,
			title = excluded.title,
			mode = excluded.mode,
			content_mode = excluded.content_mode,
			sort_mode = excluded.sort_mode,
			config_json = excluded.config_json,
			is_manual_order = excluded.is_manual_order,
			manual_updated_at = excluded.manual_updated_at,
			parent_ray_id = excluded.parent_ray_id,
			revision = excluded.revision,
			folder_scope = excluded.folder_scope
	`,
		ray.ID,
		ray.SeedItemID,
		ray.Title,
		ray.Mode,
		ray.ContentMode,
		ray.SortMode,
		ray.ConfigJSON,
		boolToInt(ray.IsManualOrder),
		ray.ManualUpdatedAt,
		ray.ParentRayID,
		ray.Revision,
		ray.CreatedAt,
		ray.FolderScope,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`DELETE FROM podcast_ray_items WHERE ray_id = ?`,
		ray.ID,
	); err != nil {
		return err
	}

	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO podcast_ray_items (
				ray_id,
				item_id,
				position_index,
				original_position,
				reason,
				score,
				semantic_score,
				folder_score,
				novelty_score,
				resume_score,
				added_by
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			ray.ID,
			item.Item.ID,
			item.PositionIndex,
			item.OriginalPosition,
			item.Reason,
			item.Score,
			item.SemanticScore,
			item.FolderScore,
			item.NoveltyScore,
			item.ResumeScore,
			item.AddedBy,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListPodcastRayItems(
	rayID string,
) ([]PodcastRayItemRow, error) {
	rows, err := s.db.Query(`
		SELECT
			`+podcastColumns+`,
			pri.position_index,
			pri.original_position,
			pri.reason,
			pri.score,
			pri.semantic_score,
			pri.folder_score,
			pri.novelty_score,
			pri.resume_score,
			pri.added_by
		FROM podcast_ray_items pri
		JOIN podcast_items pi ON pi.id = pri.item_id
		WHERE pri.ray_id = ?
		ORDER BY pri.position_index
	`, rayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PodcastRayItemRow, 0)
	for rows.Next() {
		var item PodcastItemRow
		var isCompleted int
		var rayItem PodcastRayItemRow

		dest := podcastScanDest(&item, &isCompleted)
		dest = append(dest,
			&rayItem.PositionIndex,
			&rayItem.OriginalPosition,
			&rayItem.Reason,
			&rayItem.Score,
			&rayItem.SemanticScore,
			&rayItem.FolderScore,
			&rayItem.NoveltyScore,
			&rayItem.ResumeScore,
			&rayItem.AddedBy,
		)

		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		item.IsCompleted = isCompleted != 0
		rayItem.Item = item
		items = append(items, rayItem)
	}

	return items, rows.Err()
}

type podcastScanner interface {
	Scan(dest ...any) error
}

func podcastScanDest(item *PodcastItemRow, isCompleted *int) []any {
	return []any{
		&item.ID,
		&item.Path,
		&item.Title,
		&item.Author,
		&item.Series,
		&item.Folder,
		&item.Duration,
		&item.FileSize,
		&item.AddedAt,
		&item.UpdatedAt,
		&item.ModifiedAt,
		&item.TranscriptPath,
		&item.TranscriptStatus,
		&item.SemanticStatus,
		&item.Summary,
		&item.TopicsJSON,
		&item.KeywordsJSON,
		&item.LastPosition,
		&item.CompletedRatio,
		&isCompleted,
		&item.PlayCount,
		&item.SkipCount,
		&item.LastPlayedAt,
		&item.LastError,
		&item.SourceType,
		&item.SourceURL,
		&item.SourceSite,
		&item.ExternalID,
		&item.DownloadStatus,
		&item.DownloadProgress,
		&item.DownloadError,
		&item.DownloadAttempts,
		&item.DownloadedAt,
	}
}

func scanPodcastRow(row podcastScanner) (PodcastItemRow, error) {
	var item PodcastItemRow
	var isCompleted int

	err := row.Scan(podcastScanDest(&item, &isCompleted)...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PodcastItemRow{}, fmt.Errorf("podcast item not found")
		}
		return PodcastItemRow{}, err
	}

	item.IsCompleted = isCompleted != 0
	return item, nil
}

func scanPodcastRows(rows *sql.Rows) ([]PodcastItemRow, error) {
	items := make([]PodcastItemRow, 0)
	for rows.Next() {
		item, err := scanPodcastRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
