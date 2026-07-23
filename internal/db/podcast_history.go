package db

import (
	"database/sql"
	"errors"
	"time"
)

func (s *Store) InsertPodcastHistory(row PodcastHistoryRow) error {
	now := time.Now().Unix()
	if row.StartedAt == 0 {
		row.StartedAt = now
	}
	if row.UpdatedAt == 0 {
		row.UpdatedAt = now
	}

	_, err := s.db.Exec(`
		INSERT INTO podcast_history (
			id,
			item_id,
			ray_id,
			started_at,
			ended_at,
			start_position,
			end_position,
			listened_seconds,
			completed_ratio,
			source,
			end_reason,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		row.ID,
		row.ItemID,
		row.RayID,
		row.StartedAt,
		row.EndedAt,
		row.StartPosition,
		row.EndPosition,
		row.ListenedSeconds,
		row.CompletedRatio,
		row.Source,
		row.EndReason,
		row.UpdatedAt,
	)
	return err
}

func (s *Store) UpdatePodcastHistoryProgress(
	id string,
	endPosition float64,
	listenedSeconds float64,
	completedRatio float64,
) error {
	_, err := s.db.Exec(`
		UPDATE podcast_history
		SET end_position = ?,
		    listened_seconds = ?,
		    completed_ratio = ?,
		    updated_at = unixepoch()
		WHERE id = ?
	`,
		endPosition,
		listenedSeconds,
		completedRatio,
		id,
	)
	return err
}

func (s *Store) FinishPodcastHistory(
	id string,
	endPosition float64,
	listenedSeconds float64,
	completedRatio float64,
	endReason string,
) error {
	_, err := s.db.Exec(`
		UPDATE podcast_history
		SET ended_at = unixepoch(),
		    end_position = ?,
		    listened_seconds = ?,
		    completed_ratio = ?,
		    end_reason = ?,
		    updated_at = unixepoch()
		WHERE id = ?
		  AND ended_at = 0
	`,
		endPosition,
		listenedSeconds,
		completedRatio,
		endReason,
		id,
	)
	return err
}

func (s *Store) FinishOpenPodcastHistories(endReason string) error {
	_, err := s.db.Exec(`
		UPDATE podcast_history
		SET ended_at = CASE
		        WHEN updated_at > 0 THEN updated_at
		        ELSE unixepoch()
		    END,
		    end_reason = CASE
		        WHEN end_reason = '' THEN ?
		        ELSE end_reason
		    END
		WHERE ended_at = 0
	`, endReason)
	return err
}

func (s *Store) ListPodcastHistory(limit int) ([]PodcastHistoryRow, error) {
	if limit <= 0 {
		limit = 200
	}

	rows, err := s.db.Query(`
		SELECT
			ph.id,
			ph.item_id,
			ph.ray_id,
			ph.started_at,
			ph.ended_at,
			ph.start_position,
			ph.end_position,
			ph.listened_seconds,
			ph.completed_ratio,
			ph.source,
			ph.end_reason,
			ph.updated_at,
			`+prefixListColumns(podcastColumns, "pi.")+`
		FROM podcast_history ph
		JOIN podcast_items pi ON pi.id = ph.item_id
		ORDER BY ph.started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]PodcastHistoryRow, 0)
	for rows.Next() {
		var history PodcastHistoryRow
		var completed int
		dest := []any{
			&history.ID,
			&history.ItemID,
			&history.RayID,
			&history.StartedAt,
			&history.EndedAt,
			&history.StartPosition,
			&history.EndPosition,
			&history.ListenedSeconds,
			&history.CompletedRatio,
			&history.Source,
			&history.EndReason,
			&history.UpdatedAt,
		}
		dest = append(dest, podcastScanDest(&history.Item, &completed)...)

		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		history.Item.IsCompleted = completed != 0
		result = append(result, history)
	}

	return result, rows.Err()
}

func (s *Store) PodcastHistoryByID(id string) (PodcastHistoryRow, error) {
	var history PodcastHistoryRow
	var completed int

	row := s.db.QueryRow(`
		SELECT
			ph.id,
			ph.item_id,
			ph.ray_id,
			ph.started_at,
			ph.ended_at,
			ph.start_position,
			ph.end_position,
			ph.listened_seconds,
			ph.completed_ratio,
			ph.source,
			ph.end_reason,
			ph.updated_at,
			`+prefixListColumns(podcastColumns, "pi.")+`
		FROM podcast_history ph
		JOIN podcast_items pi ON pi.id = ph.item_id
		WHERE ph.id = ?
	`, id)

	dest := []any{
		&history.ID,
		&history.ItemID,
		&history.RayID,
		&history.StartedAt,
		&history.EndedAt,
		&history.StartPosition,
		&history.EndPosition,
		&history.ListenedSeconds,
		&history.CompletedRatio,
		&history.Source,
		&history.EndReason,
		&history.UpdatedAt,
	}
	dest = append(dest, podcastScanDest(&history.Item, &completed)...)

	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PodcastHistoryRow{}, errors.New("podcast history item not found")
		}
		return PodcastHistoryRow{}, err
	}

	history.Item.IsCompleted = completed != 0
	return history, nil
}
