package appstate

import (
	"fmt"
	"sync"
	"time"

	"ray-player1/internal/db"
	"ray-player1/internal/library"
	"ray-player1/internal/rays"
)

type PlaybackStatus string

const (
	PlaybackStopped PlaybackStatus = "stopped"
	PlaybackLoading PlaybackStatus = "loading"
	PlaybackPlaying PlaybackStatus = "playing"
	PlaybackPaused  PlaybackStatus = "paused"
	PlaybackError   PlaybackStatus = "error"
)

type RayBuildStatus string

const (
	RayBuildIdle     RayBuildStatus = "idle"
	RayBuildBuilding RayBuildStatus = "building"
	RayBuildReady    RayBuildStatus = "ready"
	RayBuildError    RayBuildStatus = "error"
)

type RayBuildState struct {
	Status      RayBuildStatus `json:"status"`
	SeedTrackID string         `json:"seedTrackId"`
	RequestID   uint64         `json:"requestId"`
	StartedAt   int64          `json:"startedAt"`
	FinishedAt  int64          `json:"finishedAt"`
	LastError   string         `json:"lastError,omitempty"`
}

type PlayerState struct {
	Status         PlaybackStatus `json:"status"`
	CurrentTrackID string         `json:"currentTrackId"`
	CurrentPath    string         `json:"currentPath"`
	CurrentTitle   string         `json:"currentTitle"`
	CurrentArtist  string         `json:"currentArtist"`
	CurrentSub     string         `json:"currentSub"`
	DurationMs     int            `json:"durationMs"`
	DurationLabel  string         `json:"durationLabel"`
	PositionMs     int            `json:"positionMs"`
	PositionLabel  string         `json:"positionLabel"`

	QueueID     string `json:"queueId"`
	QueueIndex  int    `json:"queueIndex"`
	QueueLength int    `json:"queueLength"`

	RayID          string `json:"rayId"`
	RaySeedTrackID string `json:"raySeedTrackId"`
	UpdatedAt      int64  `json:"updatedAt"`
	LastError      string `json:"lastError,omitempty"`

	// Deprecated compatibility fields. Frontend playback UI must use Status/RayID.
	Playing           bool             `json:"playing"`
	Volume            float64          `json:"volume"`
	Muted             bool             `json:"muted"`
	LastNonZeroVolume float64          `json:"lastNonZeroVolume"`
	CurrentRayID      string           `json:"currentRayId"`
	Queue             []rays.QueueItem `json:"queue"`
}

type Store struct {
	mu        sync.RWMutex
	persistMu sync.Mutex
	state     PlayerState
	db        *db.Store
}

func NewStore(dbx *db.Store) *Store {
	return &Store{
		db: dbx,
		state: PlayerState{
			Status:        PlaybackStopped,
			QueueIndex:    -1,
			Volume:        0.58,
			PositionLabel: "0:00",
		},
	}
}

func (s *Store) Load(librarySvc *library.Service, raySvc *rays.Service) error {
	row, err := s.db.GetAppState()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Volume = row.Volume
	s.state.Status = PlaybackStopped
	s.state.Playing = false
	s.state.PositionMs = row.PositionMs
	s.state.PositionLabel = fmt.Sprintf("%d:%02d", row.PositionMs/60000, (row.PositionMs/1000)%60)
	if row.CurrentTrackID != "" {
		if t, ok := librarySvc.TrackByID(row.CurrentTrackID); ok {
			s.state.CurrentTrackID = t.ID
			s.state.CurrentPath = t.Path
			s.state.CurrentTitle = t.Title
			s.state.CurrentArtist = t.Artist
			s.state.DurationMs = t.DurationMs
			s.state.DurationLabel = t.DurationLabel
		}
	}
	if row.CurrentRayID != "" && raySvc.LoadCurrent(row.CurrentRayID) {
		ray := raySvc.CurrentRay()
		s.state.Status = PlaybackPaused
		s.state.RayID = ray.ID
		s.state.CurrentRayID = ray.ID
		s.state.RaySeedTrackID = ray.SeedTrackID
		s.state.QueueID = ray.ID
		s.state.Queue = raySvc.CurrentQueue()
		s.state.QueueIndex = queueIndex(s.state.Queue, s.state.CurrentTrackID)
		s.state.QueueLength = len(s.state.Queue)
		s.state.CurrentSub = queueSubtitle(s.state.Queue)
	}
	return nil
}

func (s *Store) Get() PlayerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clonePlayerState(s.state)
}

func (s *Store) Replace(st PlayerState) {
	s.replace(st, true)
}

// ReplaceTransient updates in-memory player state without persisting it.
// It is used for high-frequency UI previews such as a volume drag.
func (s *Store) ReplaceTransient(st PlayerState) {
	s.replace(st, false)
}

func (s *Store) replace(st PlayerState, persist bool) {
	st.Playing = st.Status == PlaybackPlaying
	st.CurrentRayID = st.RayID
	st.Queue = cloneQueue(st.Queue)
	if st.Queue != nil {
		st.QueueLength = len(st.Queue)
		st.QueueIndex = queueIndex(st.Queue, st.CurrentTrackID)
	}
	st.UpdatedAt = time.Now().UnixMilli()

	if persist && s.db != nil {
		s.persistMu.Lock()
		defer s.persistMu.Unlock()
	}

	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
	if !persist || s.db == nil {
		return
	}

	_ = s.db.SetAppState(db.AppStateRow{
		CurrentTrackID: st.CurrentTrackID,
		PositionMs:     st.PositionMs,
		Volume:         st.Volume,
		Playing:        st.Playing,
		CurrentRayID:   st.CurrentRayID,
	})
}

func clonePlayerState(st PlayerState) PlayerState {
	st.Queue = cloneQueue(st.Queue)
	return st
}

func cloneQueue(queue []rays.QueueItem) []rays.QueueItem {
	if queue == nil {
		return nil
	}
	return append([]rays.QueueItem{}, queue...)
}

func (s *Store) SetPlaybackPosition(positionMs int, persist bool) {
	if persist && s.db != nil {
		s.persistMu.Lock()
		defer s.persistMu.Unlock()
	}

	s.mu.Lock()
	s.state.PositionMs = positionMs
	s.state.PositionLabel = fmt.Sprintf("%d:%02d", positionMs/60000, (positionMs/1000)%60)
	st := s.state
	s.mu.Unlock()
	if persist && s.db != nil {
		_ = s.db.SetAppState(db.AppStateRow{CurrentTrackID: st.CurrentTrackID, PositionMs: st.PositionMs, Volume: st.Volume, Playing: st.Playing, CurrentRayID: st.CurrentRayID})
	}
}
func (s *Store) SetCurrent(track library.Track, rayID string, queue []rays.QueueItem, positionMs int) {
	queue = cloneQueue(queue)
	for i := range queue {
		if queue[i].TrackID == track.ID && queue[i].Track.ID == "" {
			queue[i].Track = track
		}
	}
	st := s.Get()
	st.Status = PlaybackPlaying
	st.CurrentTrackID = track.ID
	st.CurrentPath = track.Path
	st.CurrentTitle = track.Title
	st.CurrentArtist = track.Artist
	st.DurationMs = track.DurationMs
	st.DurationLabel = track.DurationLabel
	st.PositionMs = positionMs
	st.PositionLabel = fmt.Sprintf("%d:%02d", positionMs/60000, (positionMs/1000)%60)
	st.RayID = rayID
	st.CurrentRayID = rayID
	st.QueueID = rayID
	st.Queue = append([]rays.QueueItem{}, queue...)
	st.QueueIndex = queueIndex(st.Queue, track.ID)
	st.QueueLength = len(st.Queue)
	st.LastError = ""
	st.CurrentSub = queueSubtitle(st.Queue)
	s.Replace(st)
}

func (s *Store) SetRaySeed(trackID string) {
	st := s.Get()
	st.RaySeedTrackID = trackID
	s.Replace(st)
}

func queueIndex(queue []rays.QueueItem, trackID string) int {
	for i, item := range queue {
		if item.TrackID == trackID {
			return i
		}
	}
	return -1
}

func queueSubtitle(queue []rays.QueueItem) string {
	return "Луч играет · далее " + nextTitle(queue)
}

func nextTitle(queue []rays.QueueItem) string {
	currentIndex := -1
	for i, item := range queue {
		if item.IsCurrent {
			currentIndex = i
			break
		}
	}
	if currentIndex >= 0 && currentIndex+1 < len(queue) {
		return queue[currentIndex+1].Title
	}
	if currentIndex == -1 && len(queue) > 1 {
		return queue[1].Title
	}
	return "ничего"
}
