package db

import (
	"database/sql"
	"errors"
)

type PodcastRayHistoryRow struct {
	Ray       PodcastRayRow
	Seed      PodcastItemRow
	ItemCount int
}

func (s *Store) ListPodcastRayHistory(limit int) ([]PodcastRayHistoryRow, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT
			pr.id,
			pr.seed_item_id,
			pr.title,
			pr.mode,
			pr.content_mode,
			pr.sort_mode,
			pr.config_json,
			pr.is_manual_order,
			pr.manual_updated_at,
			pr.parent_ray_id,
			pr.revision,
			pr.created_at,
			pr.folder_scope,
			`+prefixListColumns(podcastColumns, "pi.")+`,
			COUNT(pri.item_id)
		FROM podcast_rays pr
		JOIN podcast_items pi ON pi.id = pr.seed_item_id
		LEFT JOIN podcast_ray_items pri ON pri.ray_id = pr.id
		GROUP BY pr.id
		ORDER BY pr.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]PodcastRayHistoryRow, 0)
	for rows.Next() {
		var entry PodcastRayHistoryRow
		var manual int
		var completed int
		dest := []any{
			&entry.Ray.ID,
			&entry.Ray.SeedItemID,
			&entry.Ray.Title,
			&entry.Ray.Mode,
			&entry.Ray.ContentMode,
			&entry.Ray.SortMode,
			&entry.Ray.ConfigJSON,
			&manual,
			&entry.Ray.ManualUpdatedAt,
			&entry.Ray.ParentRayID,
			&entry.Ray.Revision,
			&entry.Ray.CreatedAt,
			&entry.Ray.FolderScope,
		}
		dest = append(dest, podcastScanDest(&entry.Seed, &completed)...)
		dest = append(dest, &entry.ItemCount)

		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		entry.Ray.IsManualOrder = manual != 0
		entry.Seed.IsCompleted = completed != 0
		result = append(result, entry)
	}

	return result, rows.Err()
}

func (s *Store) PodcastRayByID(id string) (PodcastRayRow, error) {
	var row PodcastRayRow
	var manual int

	err := s.db.QueryRow(`
		SELECT
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
		FROM podcast_rays
		WHERE id = ?
	`, id).Scan(
		&row.ID,
		&row.SeedItemID,
		&row.Title,
		&row.Mode,
		&row.ContentMode,
		&row.SortMode,
		&row.ConfigJSON,
		&manual,
		&row.ManualUpdatedAt,
		&row.ParentRayID,
		&row.Revision,
		&row.CreatedAt,
		&row.FolderScope,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PodcastRayRow{}, errors.New("podcast ray not found")
	}
	if err != nil {
		return PodcastRayRow{}, err
	}

	row.IsManualOrder = manual != 0
	return row, nil
}
