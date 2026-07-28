package rays

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/db"
	"ray-player1/internal/library"
	"ray-player1/internal/onnx"
)

type ContentMode string

const (
	ContentStable    ContentMode = "stable"
	ContentWarmUp    ContentMode = "warm_up"
	ContentCoolDown  ContentMode = "cool_down"
	ContentExplore   ContentMode = "explore"
	ContentIntensify ContentMode = "intensify"
	ContentDeepen    ContentMode = "deepen"
)

type SortMode string

const (
	SortRecommended SortMode = "recommended"
	SortNameAsc     SortMode = "name_asc"
	SortNameDesc    SortMode = "name_desc"
	SortDateDesc    SortMode = "date_desc"
	SortDateAsc     SortMode = "date_asc"
	SortManual      SortMode = "manual"
)

type EmotionBasisInsight struct {
	Label             string  `json:"label,omitempty"`
	PrevLabel         string  `json:"prevLabel,omitempty"`
	Distance          float64 `json:"distance,omitempty"`
	Smoothness        float64 `json:"smoothness,omitempty"`
	HardJump          float64 `json:"hardJump,omitempty"`
	BridgeScore       float64 `json:"bridgeScore,omitempty"`
	RawDistance       float64 `json:"rawDistance,omitempty"`
	TextureConfidence float64 `json:"textureConfidence,omitempty"`
	EdgeDrive         float64 `json:"edgeDrive,omitempty"`
	DirtyElectro      float64 `json:"dirtyElectro,omitempty"`

	Joy         float64 `json:"joy,omitempty"`
	Melancholy  float64 `json:"melancholy,omitempty"`
	Serenity    float64 `json:"serenity,omitempty"`
	Combat      float64 `json:"combat,omitempty"`
	Pressure    float64 `json:"pressure,omitempty"`
	Roughness   float64 `json:"roughness,omitempty"`
	Swagger     float64 `json:"swagger,omitempty"`
	SprintClean float64 `json:"sprintClean,omitempty"`
}

type QueueInsight struct {
	Similarity         float64             `json:"similarity"`
	MoodSmoothness     float64             `json:"moodSmoothness"`
	MoodDistance       float64             `json:"moodDistance"`
	EnergyDelta        float64             `json:"energyDelta"`
	JumpPenalty        float64             `json:"jumpPenalty"`
	Novelty            float64             `json:"novelty"`
	TempoCompatibility float64             `json:"tempoCompatibility"`
	TempoUnknown       bool                `json:"tempoUnknown,omitempty"`
	TextureContinuity  float64             `json:"textureContinuity"`
	VocalContinuity    float64             `json:"vocalContinuity"`
	SessionFit         float64             `json:"sessionFit"`
	TargetMoodFit      float64             `json:"targetMoodFit"`
	Mode               string              `json:"mode"`
	Bucket             string              `json:"bucket,omitempty"`
	Strategy           string              `json:"strategy,omitempty"`
	Score              float64             `json:"score,omitempty"`
	Transition         string              `json:"transition,omitempty"`
	EnergyDirection    string              `json:"energyDirection,omitempty"`
	Discovery          bool                `json:"discovery"`
	Bridge             bool                `json:"bridge"`
	Confidence         float64             `json:"confidence,omitempty"`
	Fallback           string              `json:"fallback,omitempty"`
	Warning            string              `json:"warning,omitempty"`
	LowTrustFeatures   []string            `json:"lowTrustFeatures,omitempty"`
	Emotion            EmotionBasisInsight `json:"emotion,omitempty"`
}

type QueueItem struct {
	// Старые поля сохраняются для обратной совместимости.
	TrackID       string       `json:"trackId"`
	Title         string       `json:"title"`
	Subtitle      string       `json:"subtitle"`
	DurationLabel string       `json:"durationLabel"`
	IsCurrent     bool         `json:"isCurrent"`
	Reason        string       `json:"reason"`
	Bucket        string       `json:"-"`
	Strategy      string       `json:"-"`
	Score         float64      `json:"-"`
	Insight       QueueInsight `json:"insight"`

	// Компактная metadata для списков. Frontend не должен искать Track
	// отдельно в общей библиотеке.
	Artist           string          `json:"artist"`
	Album            string          `json:"album,omitempty"`
	GenrePrimary     string          `json:"genrePrimary"`
	GenreLabel       string          `json:"genreLabel"`
	GenreDetail      string          `json:"genreDetail"`
	GenreTags        []onnx.GenreTag `json:"genreTags"`
	DurationMs       int             `json:"durationMs"`
	Position         int             `json:"position"`
	OriginalPosition int             `json:"originalPosition"`

	// Нормализованная сценарная роль элемента луча.
	RayRole   string `json:"rayRole"`
	RayReason string `json:"rayReason"`

	// Вложенный Track пока оставляем, чтобы не ломать debug/UI-код.
	Track library.Track `json:"track"`
}

type RaySummary struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	TrackCount       int         `json:"trackCount"`
	CurrentTrackID   string      `json:"currentTrackId"`
	CurrentTrackName string      `json:"currentTrackName"`
	ResumeLabel      string      `json:"resumeLabel"`
	PositionMs       int         `json:"positionMs"`
	Active           bool        `json:"active"`
	ContentMode      ContentMode `json:"contentMode"`
	SortMode         SortMode    `json:"sortMode"`
	IsManualOrder    bool        `json:"isManualOrder"`
	ParentRayID      string      `json:"parentRayId"`
	Revision         int         `json:"revision"`
	Kind             string      `json:"kind"`
	SnapshotKey      string      `json:"snapshotKey"`
	SavedAt          int64       `json:"savedAt"`
	Saved            bool        `json:"saved"`
	Items            []QueueItem `json:"items"`
}

type Ray struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	SeedTrackID    string      `json:"seedTrackId"`
	CurrentTrackID string      `json:"currentTrackId"`
	Queue          []QueueItem `json:"queue"`
	ResumeLabel    string      `json:"resumeLabel"`
	PositionMs     int         `json:"positionMs"`

	ContentMode     ContentMode `json:"contentMode"`
	SortMode        SortMode    `json:"sortMode"`
	IsManualOrder   bool        `json:"isManualOrder"`
	ManualUpdatedAt int64       `json:"manualUpdatedAt"`
	ParentRayID     string      `json:"parentRayId"`
	Revision        int         `json:"revision"`
	Kind            string      `json:"kind"`
	SnapshotKey     string      `json:"snapshotKey"`
	SavedAt         int64       `json:"savedAt"`
}

type PlaybackQueue struct {
	ID             string      `json:"id"`
	Kind           string      `json:"kind"`
	Items          []QueueItem `json:"items"`
	Index          int         `json:"index"`
	RayID          string      `json:"rayId,omitempty"`
	RaySeedTrackID string      `json:"raySeedTrackId,omitempty"`
	ContentMode    ContentMode `json:"contentMode,omitempty"`
	SortMode       SortMode    `json:"sortMode,omitempty"`
	CreatedAt      int64       `json:"createdAt"`
	UpdatedAt      int64       `json:"updatedAt"`
}

type Service struct {
	store   *db.Store
	library *library.Service
	current Ray
	mu      sync.RWMutex
}

func NewService(store *db.Store, library *library.Service) *Service {
	return &Service{store: store, library: library}
}

func (s *Service) Activate(seed library.Track, queue []QueueItem) string {
	id := fmt.Sprintf("ray-%d", time.Now().UnixNano())
	queue = normalizedQueue(queue, seed.ID)
	queue = s.hydrateQueue(queue)
	queue = normalizeQueueMetadata(queue)
	s.current = Ray{ID: id, Name: seed.Title, SeedTrackID: seed.ID, CurrentTrackID: seed.ID, Queue: append([]QueueItem{}, queue...), ResumeLabel: "продолжить с 0:00", PositionMs: 0, ContentMode: ContentStable, SortMode: SortRecommended, Kind: "history", Revision: 1}
	_ = s.persist(true)
	return id
}

func (s *Service) SeedHistory(seed library.Track, queue []QueueItem) {
	queue = normalizedQueue(queue, seed.ID)
	queue = s.hydrateQueue(queue)
	queue = normalizeQueueMetadata(queue)
	s.current = Ray{ID: fmt.Sprintf("ray-seed-%d", time.Now().UnixNano()), Name: seed.Title, SeedTrackID: seed.ID, CurrentTrackID: seed.ID, Queue: append([]QueueItem{}, queue...), ResumeLabel: "продолжить с 0:00", PositionMs: 0, ContentMode: ContentStable, SortMode: SortRecommended, Kind: "history", Revision: 1}
	_ = s.persist(false)
}

func (s *Service) LoadCurrent(rayID string) bool {
	if rayID == "" {
		return false
	}
	header, err := s.store.RaySummaryByID(rayID)
	if err != nil {
		return false
	}
	rows, err := s.store.GetRayQueue(rayID)
	if err != nil || len(rows) == 0 {
		return false
	}
	seedTrackID, err := s.store.GetRaySeedTrackID(rayID)
	if err != nil {
		return false
	}
	s.current = Ray{
		ID:             rayID,
		Name:           header.Name,
		SeedTrackID:    seedTrackID,
		CurrentTrackID: header.CurrentTrackID,
		ResumeLabel:    header.ResumeLabel,
		PositionMs:     header.PositionMs,
		ContentMode:    NormalizeContentMode(header.ContentMode),
		SortMode:       NormalizeSortMode(header.SortMode),
		IsManualOrder:  header.IsManualOrder,
		ParentRayID:    header.ParentRayID,
		Revision:       header.Revision,
		Kind:           header.Kind,
		SnapshotKey:    header.SnapshotKey,
		SavedAt:        header.SavedAt,
	}

	for _, r := range rows {
		s.current.Queue = append(s.current.Queue, QueueItem{
			TrackID:       r.TrackID,
			Title:         r.Title,
			Subtitle:      r.Subtitle,
			DurationLabel: r.DurationLabel,
			Artist:        r.Artist,
			Album:         r.Album,
			GenrePrimary:  r.GenrePrimary,
			GenreLabel:    r.GenreLabel,
			GenreDetail:   r.GenreDetail,
			GenreTags:     r.GenreTags,
			DurationMs:    r.DurationMs,
			Position:      r.PositionIndex,
			IsCurrent:     r.IsCurrent,
			Reason:        r.Reason,
			RayReason:     r.Reason,
			Bucket:        r.Bucket,
			Strategy:      r.Strategy,
			Score:         r.Score,
		})
		if r.IsCurrent {
			s.current.CurrentTrackID = r.TrackID
		}
	}
	s.current.Queue = s.hydrateQueue(s.current.Queue)
	s.current.Queue = normalizeQueueMetadata(s.current.Queue)
	if s.current.CurrentTrackID == "" && len(s.current.Queue) > 0 {
		s.current.CurrentTrackID = s.current.Queue[0].TrackID
		s.current.Queue = normalizedQueue(s.current.Queue, s.current.CurrentTrackID)
	}
	return true
}

func (s *Service) CurrentQueue() []QueueItem {
	queue := s.hydrateQueue(s.current.Queue)
	return normalizeQueueMetadata(queue)
}

func (s *Service) CurrentItem() (QueueItem, bool) {
	idx := queueIndex(s.current.Queue, s.current.CurrentTrackID)
	if idx < 0 {
		idx = queueIndexCurrentFlag(s.current.Queue)
	}
	if idx < 0 || idx >= len(s.current.Queue) {
		return QueueItem{}, false
	}
	return s.current.Queue[idx], true
}

func (s *Service) CurrentRay() Ray { return s.current }

func NormalizeContentMode(mode string) ContentMode {
	switch ContentMode(strings.TrimSpace(mode)) {
	case ContentWarmUp,
		ContentCoolDown,
		ContentExplore,
		ContentIntensify,
		ContentDeepen:
		return ContentMode(mode)
	default:
		return ContentStable
	}
}

func NormalizeSortMode(mode string) SortMode {
	switch SortMode(strings.TrimSpace(mode)) {
	case SortNameAsc,
		SortNameDesc,
		SortDateDesc,
		SortDateAsc,
		SortManual:
		return SortMode(mode)
	default:
		return SortRecommended
	}
}

func normalizeTitleForSort(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, prefix := range []string{
		"the ",
		"a ",
		"an ",
	} {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}

func cloneRay(ray Ray) Ray {
	out := ray
	out.Queue = append([]QueueItem(nil), ray.Queue...)
	return out
}

func currentItemID(ray Ray) string {
	if ray.CurrentTrackID != "" {
		return ray.CurrentTrackID
	}
	for _, item := range ray.Queue {
		if item.IsCurrent {
			return item.TrackID
		}
	}
	return ""
}

func reindexItems(ray *Ray, currentTrackID string) {
	for index := range ray.Queue {
		ray.Queue[index].Position = index
		ray.Queue[index].IsCurrent = ray.Queue[index].TrackID == currentTrackID
	}
}

func (s *Service) SetSortMode(mode SortMode) (Ray, error) {
	mode = NormalizeSortMode(string(mode))

	if s.current.ID == "" {
		return Ray{}, errors.New("music ray is not built")
	}

	currentTrackID := currentItemID(s.current)
	applySort(&s.current, mode, currentTrackID)
	s.current.Revision++
	ray := cloneRay(s.current)
	_ = s.persist(true)

	return ray, nil
}

func applySort(
	ray *Ray,
	mode SortMode,
	currentTrackID string,
) {
	ray.SortMode = mode

	switch mode {
	case SortNameAsc:
		sort.SliceStable(ray.Queue, func(i, j int) bool {
			return normalizeTitleForSort(
				ray.Queue[i].Title,
			) < normalizeTitleForSort(
				ray.Queue[j].Title,
			)
		})
		ray.IsManualOrder = false

	case SortNameDesc:
		sort.SliceStable(ray.Queue, func(i, j int) bool {
			return normalizeTitleForSort(
				ray.Queue[i].Title,
			) > normalizeTitleForSort(
				ray.Queue[j].Title,
			)
		})
		ray.IsManualOrder = false

	case SortDateDesc:
		sort.SliceStable(ray.Queue, func(i, j int) bool {
			return ray.Queue[i].Track.ImportedAt >
				ray.Queue[j].Track.ImportedAt
		})
		ray.IsManualOrder = false

	case SortDateAsc:
		sort.SliceStable(ray.Queue, func(i, j int) bool {
			return ray.Queue[i].Track.ImportedAt <
				ray.Queue[j].Track.ImportedAt
		})
		ray.IsManualOrder = false

	case SortManual:
		ray.IsManualOrder = true

	default:
		ray.SortMode = SortRecommended
		sort.SliceStable(ray.Queue, func(i, j int) bool {
			if ray.Queue[i].Score == ray.Queue[j].Score {
				return ray.Queue[i].OriginalPosition <
					ray.Queue[j].OriginalPosition
			}
			return ray.Queue[i].Score >
				ray.Queue[j].Score
		})
		ray.IsManualOrder = false
	}

	if !ray.IsManualOrder {
		ray.ManualUpdatedAt = 0
	}
	reindexItems(ray, currentTrackID)
}

func (s *Service) ReplaceWithRebuiltRay(
	parent Ray,
	mode ContentMode,
	items []QueueItem,
) (Ray, error) {
	currentTrackID := currentItemID(parent)
	if currentTrackID == "" && len(items) > 0 {
		currentTrackID = items[0].TrackID
	}
	items = normalizedQueue(items, currentTrackID)
	ray := Ray{
		ID: fmt.Sprintf(
			"ray-%d",
			time.Now().UnixNano(),
		),
		SeedTrackID:    currentTrackID,
		Name:           parent.Name,
		CurrentTrackID: currentTrackID,
		ResumeLabel:    parent.ResumeLabel,
		PositionMs:     parent.PositionMs,
		ContentMode:    mode,
		SortMode:       SortRecommended,
		ParentRayID:    parent.ID,
		Revision:       parent.Revision + 1,
		Kind:           "history",
		Queue:          append([]QueueItem(nil), items...),
	}

	for index := range ray.Queue {
		ray.Queue[index].Position = index
		ray.Queue[index].OriginalPosition = index
	}

	s.mu.Lock()
	s.current = ray
	s.mu.Unlock()

	_ = s.persist(true)
	return cloneRay(ray), nil
}

func (s *Service) FrozenQueue() PlaybackQueue {
	items := s.CurrentQueue()
	return PlaybackQueue{
		ID:             s.current.ID,
		Kind:           "ray",
		Items:          items,
		Index:          queueIndex(items, s.current.CurrentTrackID),
		RayID:          s.current.ID,
		RaySeedTrackID: s.current.SeedTrackID,
		ContentMode:    s.current.ContentMode,
		SortMode:       s.current.SortMode,
		UpdatedAt:      time.Now().UnixMilli(),
	}
}

func (s *Service) FrozenQueueJSON() (string, error) {
	data, err := json.Marshal(s.FrozenQueue())
	if err != nil {
		return "", fmt.Errorf("marshal playback queue: %w", err)
	}
	return string(data), nil
}

func (s *Service) RestoreFrozenQueue(raw string, currentTrackID string) error {
	var queue PlaybackQueue
	if err := json.Unmarshal([]byte(raw), &queue); err != nil {
		return fmt.Errorf("decode frozen queue: %w", err)
	}
	if len(queue.Items) == 0 {
		return fmt.Errorf("frozen queue is empty")
	}

	queue.Items = s.hydrateQueue(queue.Items)
	queue.Items = normalizeQueueMetadata(queue.Items)
	currentIndex := queueIndex(queue.Items, currentTrackID)
	if currentIndex < 0 {
		currentIndex = queue.Index
	}
	if currentIndex < 0 || currentIndex >= len(queue.Items) {
		return fmt.Errorf("current track is absent from frozen queue")
	}

	queue.Items = normalizedQueue(queue.Items, queue.Items[currentIndex].TrackID)
	s.current = Ray{
		ID:             queue.RayID,
		SeedTrackID:    queue.RaySeedTrackID,
		CurrentTrackID: queue.Items[currentIndex].TrackID,
		Queue:          queue.Items,
		ContentMode:    NormalizeContentMode(string(queue.ContentMode)),
		SortMode:       NormalizeSortMode(string(queue.SortMode)),
		Kind:           "history",
	}
	return nil
}

func (s *Service) Remaining() int {
	idx := queueIndex(s.current.Queue, s.current.CurrentTrackID)
	if idx < 0 {
		idx = queueIndexCurrentFlag(s.current.Queue)
	}
	if idx < 0 {
		return len(s.current.Queue)
	}
	return len(s.current.Queue) - idx - 1
}

func (s *Service) RepeatToStart() (QueueItem, bool) {
	if len(s.current.Queue) == 0 {
		return QueueItem{}, false
	}
	return s.jumpToIndex(0)
}

func (s *Service) Append(items []QueueItem) {
	if len(items) == 0 {
		return
	}
	s.current.Queue = append(s.current.Queue, items...)
	_ = s.persist(true)
}

func (s *Service) TrackIDs() []string {
	out := make([]string, 0, len(s.current.Queue))
	for _, item := range s.current.Queue {
		out = append(out, item.TrackID)
	}
	return out
}

func (s *Service) jumpToIndex(index int) (QueueItem, bool) {
	if index < 0 || index >= len(s.current.Queue) {
		return QueueItem{}, false
	}
	for i := range s.current.Queue {
		s.current.Queue[i].IsCurrent = i == index
	}
	s.current.CurrentTrackID = s.current.Queue[index].TrackID
	s.current.PositionMs = 0
	s.current.ResumeLabel = "продолжить с 0:00"
	_ = s.persist(true)
	return s.current.Queue[index], true
}

func (s *Service) Summaries() []RaySummary {
	return s.summariesByKind("history")
}

func (s *Service) SavedSummaries() []RaySummary {
	return s.summariesByKind("saved")
}

func (s *Service) summariesByKind(kind string) []RaySummary {
	rows, err := s.store.ListRays(kind)
	if err != nil {
		return nil
	}
	out := make([]RaySummary, 0, len(rows))
	for _, r := range rows {
		summary := RaySummary{
			ID: r.ID, Name: r.Name, TrackCount: r.TrackCount,
			CurrentTrackID: r.CurrentTrackID, CurrentTrackName: r.CurrentTrackName,
			ResumeLabel: r.ResumeLabel, PositionMs: r.PositionMs, Active: r.Active,
			ContentMode: NormalizeContentMode(r.ContentMode), SortMode: NormalizeSortMode(r.SortMode),
			IsManualOrder: r.IsManualOrder, ParentRayID: r.ParentRayID, Revision: r.Revision,
			Kind: r.Kind, SnapshotKey: r.SnapshotKey, SavedAt: r.SavedAt, Saved: r.Saved,
		}
		if queue, queueErr := s.store.GetRayQueue(r.ID); queueErr == nil {
			for _, item := range queue {
				summary.Items = append(summary.Items, QueueItem{
					TrackID: item.TrackID, Title: item.Title, Subtitle: item.Subtitle,
					Artist: item.Artist, Album: item.Album, GenrePrimary: item.GenrePrimary,
					GenreLabel: item.GenreLabel, GenreDetail: item.GenreDetail,
					GenreTags: item.GenreTags, DurationMs: item.DurationMs,
					DurationLabel: item.DurationLabel, Position: item.PositionIndex,
					OriginalPosition: item.PositionIndex, IsCurrent: item.IsCurrent,
					Reason: item.Reason, RayReason: item.Reason,
					Bucket: item.Bucket, Strategy: item.Strategy, Score: item.Score,
				})
			}
			summary.Items = normalizeQueueMetadata(s.hydrateQueue(summary.Items))
		}
		out = append(out, summary)
	}
	return out
}

func snapshotKey(ray Ray) string {
	parts := []string{
		string(NormalizeContentMode(string(ray.ContentMode))),
		string(NormalizeSortMode(string(ray.SortMode))),
	}
	for _, item := range ray.Queue {
		parts = append(parts, item.TrackID)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

func (s *Service) SaveSnapshot(rayID string) (string, error) {
	if !s.LoadCurrent(rayID) {
		return "", fmt.Errorf("ray not found: %s", rayID)
	}
	key := snapshotKey(s.current)
	if err := s.store.SetRaySnapshotKey(s.current.ID, key); err != nil {
		return "", err
	}
	destID := fmt.Sprintf("saved-%x", sha256.Sum256([]byte(key)))[:30]
	return s.store.CloneRaySnapshot(s.current.ID, destID, key)
}

func (s *Service) UnsaveSnapshot(rayID string) error {
	header, err := s.store.RaySummaryByID(rayID)
	if err != nil {
		return err
	}
	key := header.SnapshotKey
	if key == "" {
		if !s.LoadCurrent(rayID) {
			return fmt.Errorf("ray not found: %s", rayID)
		}
		key = snapshotKey(s.current)
	}
	return s.store.DeleteSavedRayBySnapshotKey(key)
}

func (s *Service) DeleteSaved(id string) error {
	return s.store.DeleteSavedRay(id)
}

func (s *Service) ActivateSaved(id string) (Ray, error) {
	if !s.LoadCurrent(id) || s.current.Kind != "saved" {
		return Ray{}, fmt.Errorf("saved ray not found: %s", id)
	}
	source := cloneRay(s.current)
	source.ID = fmt.Sprintf("ray-%d", time.Now().UnixNano())
	source.Kind = "history"
	source.ParentRayID = id
	source.SavedAt = 0
	source.SnapshotKey = snapshotKey(source)
	s.mu.Lock()
	s.current = source
	s.mu.Unlock()
	if err := s.persist(true); err != nil {
		return Ray{}, err
	}
	return cloneRay(source), nil
}

func (s *Service) Resume(rayID string) (Ray, bool) {
	if !s.LoadCurrent(rayID) {
		return Ray{}, false
	}
	return s.current, true
}

func (s *Service) Next() (QueueItem, bool) {
	currentIndex := queueIndex(s.current.Queue, s.current.CurrentTrackID)
	if currentIndex < 0 {
		currentIndex = queueIndexCurrentFlag(s.current.Queue)
	}
	if currentIndex < 0 || currentIndex+1 >= len(s.current.Queue) {
		return QueueItem{}, false
	}
	return s.jumpToIndex(currentIndex + 1)
}

func (s *Service) Previous() (QueueItem, bool) {
	currentIndex := queueIndex(s.current.Queue, s.current.CurrentTrackID)
	if currentIndex < 0 {
		currentIndex = queueIndexCurrentFlag(s.current.Queue)
	}
	if currentIndex <= 0 || currentIndex >= len(s.current.Queue) {
		return QueueItem{}, false
	}
	return s.jumpToIndex(currentIndex - 1)
}

func (s *Service) Remove(trackID string) {
	out := make([]QueueItem, 0, len(s.current.Queue))
	removedCurrent := false
	for _, item := range s.current.Queue {
		if item.TrackID != trackID {
			out = append(out, item)
			continue
		}
		if item.IsCurrent || item.TrackID == s.current.CurrentTrackID {
			removedCurrent = true
		}
	}
	s.current.Queue = out
	if removedCurrent {
		s.current.CurrentTrackID = ""
		for i := range s.current.Queue {
			s.current.Queue[i].IsCurrent = false
		}
		if len(s.current.Queue) > 0 {
			s.current.Queue[0].IsCurrent = true
			s.current.CurrentTrackID = s.current.Queue[0].TrackID
		}
	}
	_ = s.persist(true)
}

func (s *Service) Move(trackID string, newIndex int) (Ray, error) {
	idx := -1
	for i, item := range s.current.Queue {
		if item.TrackID == trackID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Ray{}, fmt.Errorf("track not found in current ray: %s", trackID)
	}
	if newIndex < 0 || newIndex >= len(s.current.Queue) || idx == newIndex {
		ray := cloneRay(s.current)
		return ray, nil
	}

	currentTrackID := currentItemID(s.current)
	item := s.current.Queue[idx]
	q := append(append([]QueueItem{}, s.current.Queue[:idx]...), s.current.Queue[idx+1:]...)
	q = append(q[:newIndex], append([]QueueItem{item}, q[newIndex:]...)...)
	s.current.Queue = q
	s.current.SortMode = SortManual
	s.current.IsManualOrder = true
	s.current.ManualUpdatedAt = time.Now().UnixMilli()
	s.current.Revision++
	reindexItems(&s.current, currentTrackID)

	ray := cloneRay(s.current)
	_ = s.persist(true)
	return ray, nil
}

func (s *Service) UpdateCurrentTrack(trackID string) {
	_ = s.JumpToTrack(trackID)
}

func (s *Service) JumpToTrack(trackID string) bool {
	index := queueIndex(s.current.Queue, trackID)
	if index < 0 {
		return false
	}
	s.current.CurrentTrackID = trackID
	s.current.PositionMs = 0
	s.current.ResumeLabel = "продолжить с 0:00"
	for i := range s.current.Queue {
		s.current.Queue[i].IsCurrent = s.current.Queue[i].TrackID == trackID
	}
	_ = s.persist(true)
	return true
}

func (s *Service) UpdateState(rayID, trackID string, positionMs int, resumeLabel string) error {
	if s.current.ID == rayID {
		s.current.CurrentTrackID = trackID
		s.current.PositionMs = positionMs
		s.current.ResumeLabel = resumeLabel
	}
	return s.store.UpdateRayState(rayID, trackID, positionMs, resumeLabel)
}

func (s *Service) persist(active bool) error {
	rows := make([]db.RayTrackRow, 0, len(s.current.Queue))
	for i, item := range s.current.Queue {
		rows = append(rows, db.RayTrackRow{
			TrackID:       item.TrackID,
			Title:         item.Title,
			Subtitle:      item.Subtitle,
			DurationLabel: item.DurationLabel,
			IsCurrent:     item.IsCurrent,
			Reason:        firstNonEmpty(item.RayReason, item.Reason),
			Bucket:        persistableRoleBucket(item),
			Strategy:      item.Strategy,
			Score:         item.Score,
			PositionIndex: i,
		})
	}
	seedID := s.current.SeedTrackID
	if seedID == "" && len(s.current.Queue) > 0 {
		seedID = s.current.Queue[0].TrackID
	}
	name := s.current.Name
	if name == "" && len(s.current.Queue) > 0 {
		name = s.current.Queue[0].Title
	}
	if err := s.store.SaveRay(s.current.ID, name, seedID, s.current.CurrentTrackID, s.current.ResumeLabel, s.current.PositionMs, active, rows); err != nil {
		return err
	}
	if s.current.Kind == "" {
		s.current.Kind = "history"
	}
	s.current.SnapshotKey = snapshotKey(s.current)
	if err := s.store.SetRaySnapshotKey(s.current.ID, s.current.SnapshotKey); err != nil {
		return err
	}
	return s.store.SaveRayState(
		s.current.ID,
		string(s.current.ContentMode),
		string(s.current.SortMode),
		s.current.IsManualOrder,
		s.current.ManualUpdatedAt,
		s.current.ParentRayID,
		s.current.Revision,
	)
}

func persistableRoleBucket(item QueueItem) string {
	if strings.TrimSpace(item.Bucket) != "" {
		return item.Bucket
	}
	if strings.TrimSpace(item.RayRole) != "" {
		return item.RayRole
	}
	return "next"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func normalizedQueue(queue []QueueItem, currentTrackID string) []QueueItem {
	if len(queue) == 0 {
		return queue
	}
	out := append([]QueueItem{}, queue...)
	hasCurrent := false
	for i := range out {
		out[i].IsCurrent = out[i].TrackID == currentTrackID
		hasCurrent = hasCurrent || out[i].IsCurrent
	}
	if !hasCurrent {
		out[0].IsCurrent = true
	}
	return out
}

func (s *Service) hydrateQueue(queue []QueueItem) []QueueItem {
	if len(queue) == 0 {
		return queue
	}

	out := append([]QueueItem{}, queue...)
	for i := range out {
		out[i].Position = i
		out[i].RayRole = queueItemRole(out[i], i)
		out[i].RayReason = out[i].Reason

		if s.library == nil {
			continue
		}

		track, ok := s.library.TrackByID(out[i].TrackID)
		if ok {
			applyTrackMetadata(&out[i], track)
		}
	}
	return out
}

func applyTrackMetadata(item *QueueItem, track library.Track) {
	if item == nil {
		return
	}

	item.Track = track
	item.TrackID = track.ID
	item.Artist = track.Artist
	item.Album = track.Album
	item.GenrePrimary = track.GenrePrimary
	item.GenreLabel = track.GenreLabel
	item.GenreDetail = track.GenreDetail
	item.GenreTags = append([]onnx.GenreTag(nil), track.GenreTags...)
	item.DurationMs = track.DurationMs

	if item.Title == "" {
		item.Title = track.Title
	}
	if item.Subtitle == "" {
		item.Subtitle = track.Artist
	}
	if item.DurationLabel == "" {
		item.DurationLabel = track.DurationLabel
	}
}

func queueItemRole(item QueueItem, position int) string {
	bucket := strings.ToLower(strings.TrimSpace(item.Bucket))
	strategy := strings.ToLower(strings.TrimSpace(item.Strategy))

	// Существующие bucket/strategy остаются source of truth,
	// а rayRole — стабильное UI-представление.
	switch {
	case bucket == "seed" || strategy == "seed":
		return "seed"

	case bucket == "manual" ||
		strategy == "manual" ||
		strategy == "user":
		return "manual"

	case bucket == "discovery" ||
		bucket == "explore" ||
		strategy == "discovery" ||
		strategy == "explore":
		return "discovery"

	case bucket == "bridge" ||
		strategy == "bridge":
		return "bridge"

	case bucket == "comfort" ||
		bucket == "familiar" ||
		strategy == "comfort":
		return "comfort"

	case bucket == "nearby" ||
		bucket == "near" ||
		strategy == "nearby":
		return "nearby"

	case position == 0 && item.IsCurrent:
		// Для старых сохранённых лучей seed мог не иметь отдельного bucket.
		return "seed"

	default:
		return "next"
	}
}

func normalizeQueueMetadata(queue []QueueItem) []QueueItem {
	out := append([]QueueItem(nil), queue...)
	for i := range out {
		out[i].Position = i
		out[i].RayRole = queueItemRole(out[i], i)
		if out[i].RayReason == "" {
			out[i].RayReason = out[i].Reason
		}
		if out[i].GenreTags == nil {
			out[i].GenreTags = []onnx.GenreTag{}
		}
	}
	return out
}

func queueIndex(queue []QueueItem, trackID string) int {
	for i, item := range queue {
		if item.TrackID == trackID {
			return i
		}
	}
	return -1
}

func queueIndexCurrentFlag(queue []QueueItem) int {
	for i, item := range queue {
		if item.IsCurrent {
			return i
		}
	}
	return -1
}
