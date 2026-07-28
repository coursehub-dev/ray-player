package podcast

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/db"
)

type HistoryItem struct {
	ID              string  `json:"id"`
	Item            Item    `json:"item"`
	RayID           string  `json:"rayId"`
	StartedAt       int64   `json:"startedAt"`
	EndedAt         int64   `json:"endedAt"`
	StartPosition   float64 `json:"startPosition"`
	EndPosition     float64 `json:"endPosition"`
	ListenedSeconds float64 `json:"listenedSeconds"`
	CompletedRatio  float64 `json:"completedRatio"`
	Source          string  `json:"source"`
	EndReason       string  `json:"endReason"`
	PlayedAtLabel   string  `json:"playedAtLabel"`
	ListenedLabel   string  `json:"listenedLabel"`
	PositionLabel   string  `json:"positionLabel"`
	ProgressPercent int     `json:"progressPercent"`
}

type RayHistoryItem struct {
	ID             string         `json:"id"`
	SeedItemID     string         `json:"seedItemId"`
	Seed           Item           `json:"seed"`
	Title          string         `json:"title"`
	ContentMode    RayContentMode `json:"contentMode"`
	SortMode       RaySortMode    `json:"sortMode"`
	IsManualOrder  bool           `json:"isManualOrder"`
	ParentRayID    string         `json:"parentRayId"`
	Revision       int            `json:"revision"`
	FolderScope    string         `json:"folderScope"`
	ItemCount      int            `json:"itemCount"`
	CreatedAt      int64          `json:"createdAt"`
	CreatedAtLabel string         `json:"createdAtLabel"`
	Items          []RayItem      `json:"items"`
}

type activeHistory struct {
	ID              string
	ItemID          string
	RayID           string
	StartPosition   float64
	LastPosition    float64
	ListenedSeconds float64
	CompletedRatio  float64
	LastTickAt      time.Time
}

type HistoryService struct {
	store *db.Store

	mu     sync.Mutex
	active activeHistory
}

func NewHistoryService(store *db.Store) *HistoryService {
	return &HistoryService{store: store}
}

func (h *HistoryService) Recover() error {
	return h.store.FinishOpenPodcastHistories("application_restart")
}

func (h *HistoryService) Begin(item Item, rayID string, source string, startPosition float64) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active.ID != "" {
		if h.active.ItemID == item.ID {
			return nil
		}
		if err := h.finishLocked(h.active.LastPosition, "switch_item"); err != nil {
			return err
		}
	}

	now := time.Now()
	if strings.TrimSpace(source) == "" {
		source = "manual"
	}
	id := fmt.Sprintf("podcast-history-%d", now.UnixNano())
	row := db.PodcastHistoryRow{
		ID:             id,
		ItemID:         item.ID,
		RayID:          rayID,
		StartedAt:      now.Unix(),
		StartPosition:  startPosition,
		EndPosition:    startPosition,
		CompletedRatio: ratioForPosition(startPosition, item.Duration),
		Source:         source,
		UpdatedAt:      now.Unix(),
	}
	if err := h.store.InsertPodcastHistory(row); err != nil {
		return err
	}

	h.active = activeHistory{
		ID:             id,
		ItemID:         item.ID,
		RayID:          rayID,
		StartPosition:  startPosition,
		LastPosition:   startPosition,
		CompletedRatio: row.CompletedRatio,
		LastTickAt:     now,
	}
	return nil
}

func (h *HistoryService) Tick(itemID string, position float64, duration float64, playing bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active.ID == "" || h.active.ItemID != itemID {
		return nil
	}

	now := time.Now()
	if playing && !h.active.LastTickAt.IsZero() {
		elapsed := now.Sub(h.active.LastTickAt).Seconds()
		if elapsed > 0 && elapsed <= 15 {
			h.active.ListenedSeconds += elapsed
		}
	}

	h.active.LastTickAt = now
	h.active.LastPosition = position
	h.active.CompletedRatio = ratioForPosition(position, duration)
	return h.store.UpdatePodcastHistoryProgress(h.active.ID, h.active.LastPosition, h.active.ListenedSeconds, h.active.CompletedRatio)
}

func (h *HistoryService) Pause(itemID string, position float64, duration float64) error {
	return h.Tick(itemID, position, duration, false)
}

func (h *HistoryService) Finish(itemID string, position float64, duration float64, reason string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active.ID == "" || h.active.ItemID != itemID {
		return nil
	}

	h.active.LastPosition = position
	h.active.CompletedRatio = ratioForPosition(position, duration)
	return h.finishLocked(position, reason)
}

func (h *HistoryService) finishLocked(position float64, reason string) error {
	if h.active.ID == "" {
		return nil
	}

	err := h.store.FinishPodcastHistory(h.active.ID, position, h.active.ListenedSeconds, h.active.CompletedRatio, reason)
	h.active = activeHistory{}
	return err
}

func (h *HistoryService) List(limit int) ([]HistoryItem, error) {
	rows, err := h.store.ListPodcastHistory(limit)
	if err != nil {
		return nil, err
	}

	result := make([]HistoryItem, 0, len(rows))
	for _, row := range rows {
		progress := int(row.CompletedRatio*100 + 0.5)
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		result = append(result, HistoryItem{
			ID:              row.ID,
			Item:            itemFromRow(row.Item),
			RayID:           row.RayID,
			StartedAt:       row.StartedAt,
			EndedAt:         row.EndedAt,
			StartPosition:   row.StartPosition,
			EndPosition:     row.EndPosition,
			ListenedSeconds: row.ListenedSeconds,
			CompletedRatio:  row.CompletedRatio,
			Source:          row.Source,
			EndReason:       row.EndReason,
			PlayedAtLabel:   historyTimeLabel(row.StartedAt),
			ListenedLabel:   durationLabel(row.ListenedSeconds),
			PositionLabel:   durationLabel(row.EndPosition),
			ProgressPercent: progress,
		})
	}
	return result, nil
}

func (h *HistoryService) RayList(limit int) ([]RayHistoryItem, error) {
	rows, err := h.store.ListPodcastRayHistory(limit)
	if err != nil {
		return nil, err
	}

	result := make([]RayHistoryItem, 0, len(rows))
	for _, row := range rows {
		entry := RayHistoryItem{
			ID:             row.Ray.ID,
			SeedItemID:     row.Ray.SeedItemID,
			Seed:           itemFromRow(row.Seed),
			Title:          row.Ray.Title,
			ContentMode:    RayContentMode(row.Ray.ContentMode),
			SortMode:       RaySortMode(row.Ray.SortMode),
			IsManualOrder:  row.Ray.IsManualOrder,
			ParentRayID:    row.Ray.ParentRayID,
			Revision:       row.Ray.Revision,
			FolderScope:    row.Ray.FolderScope,
			ItemCount:      row.ItemCount,
			CreatedAt:      row.Ray.CreatedAt,
			CreatedAtLabel: historyTimeLabel(row.Ray.CreatedAt),
		}
		if items, itemErr := h.store.ListPodcastRayItems(row.Ray.ID); itemErr == nil {
			entry.Items = make([]RayItem, 0, len(items))
			for _, item := range items {
				entry.Items = append(entry.Items, RayItem{
					Item:             itemFromRow(item.Item),
					Position:         item.PositionIndex,
					OriginalPosition: item.OriginalPosition,
					Reason:           item.Reason,
					Score:            item.Score,
					SemanticScore:    item.SemanticScore,
					FolderScore:      item.FolderScore,
					NoveltyScore:     item.NoveltyScore,
					ResumeScore:      item.ResumeScore,
					AddedBy:          item.AddedBy,
				})
			}
		}
		result = append(result, entry)
	}
	return result, nil
}

func ratioForPosition(position float64, duration float64) float64 {
	if duration <= 0 {
		return 0
	}
	ratio := position / duration
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func historyTimeLabel(value int64) string {
	if value <= 0 {
		return ""
	}
	t := time.Unix(value, 0)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "Сегодня · " + t.Format("15:04")
	}
	if t.Year() == now.Year() && t.YearDay() == now.AddDate(0, 0, -1).YearDay() {
		return "Вчера · " + t.Format("15:04")
	}
	return t.Format("02.01.2006 · 15:04")
}
