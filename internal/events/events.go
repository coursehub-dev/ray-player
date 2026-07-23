package events

import (
	"ray-player1/internal/db"
	"ray-player1/internal/library"
)

type HistoryItem struct {
	Track         library.Track `json:"track"`
	PositionMs    int           `json:"positionMs"`
	Progress      float64       `json:"progress"`
	ProgressLabel string        `json:"progressLabel"`
	PlayedAtLabel string        `json:"playedAtLabel"`
}

type FeedbackItem struct {
	TrackID        string  `json:"trackId"`
	LastPlayedAt   int64   `json:"lastPlayedAt"`
	LastSkippedAt  int64   `json:"lastSkippedAt"`
	AvgCompletion  float64 `json:"avgCompletion"`
	Affinity       float64 `json:"affinity"`
	PlayEvents     int     `json:"playEvents"`
	SkipEvents     int     `json:"skipEvents"`
	CompleteEvents int     `json:"completeEvents"`
	LastEventType  string  `json:"lastEventType"`
}

type Service struct {
	store   *db.Store
	library *library.Service
}

func NewService(store *db.Store, library *library.Service) *Service {
	return &Service{store: store, library: library}
}
func (s *Service) MarkPlay(track library.Track) error {
	return s.store.RecordEvent(track.ID, "play_start", 0, track.DurationMs)
}

func (s *Service) MarkPlaybackStarted(track library.Track) error {
	return s.store.RecordEvent(track.ID, "playback_started", 0, track.DurationMs)
}

func (s *Service) MarkPlaybackFailed(track library.Track, reason string, err error, playedMs int) error {
	msg := reason
	if err != nil {
		msg = reason + ": " + err.Error()
	}
	_ = s.store.MarkPlaybackFailed(track.ID, reason, msg)
	return s.store.RecordEvent(track.ID, "playback_failed", playedMs, track.DurationMs)
}

func (s *Service) MarkTechnicalSkip(track library.Track, reason string, playedMs int) error {
	return s.store.RecordEvent(track.ID, "technical_skip", playedMs, track.DurationMs)
}
func (s *Service) MarkSeek(trackID string, fromPos, toPos int) error {
	t, ok := s.library.TrackByID(trackID)
	if !ok {
		return nil
	}
	eventType := "seek_forward"
	if toPos < fromPos {
		eventType = "seek_backward"
	}
	return s.store.RecordEvent(trackID, eventType, toPos, t.DurationMs)
}
func (s *Service) MarkComplete(track library.Track) error {
	return s.store.RecordEvent(track.ID, "play_complete", track.DurationMs, track.DurationMs)
}
func (s *Service) MarkProgress(track library.Track, eventType string, pos int) error {
	return s.store.RecordEvent(track.ID, eventType, pos, track.DurationMs)
}
func (s *Service) MarkSkip(track library.Track, pos int, source string) string {
	kind := classifySkipEvent(track.DurationMs, pos, source)
	reward := -0.4
	switch kind {
	case "early_skip":
		reward = -1.0
	case "late_skip":
		reward = -0.35
	case "manual_next":
		if track.DurationMs > 0 && float64(pos)/float64(track.DurationMs) >= 0.60 {
			reward = -0.10
		} else if pos < 30000 {
			reward = -0.80
		} else {
			reward = -0.40
		}
	}
	_ = s.store.UpdateStrategyReward("global", reward)
	_ = s.store.RecordEvent(track.ID, kind, pos, track.DurationMs)
	if source == "jump_in_queue" {
		_ = s.store.RecordEvent(track.ID, "jump_in_queue", pos, track.DurationMs)
	}
	return kind
}

func (s *Service) RewardStrategy(strategy string, reward float64) error {
	return s.store.UpdateStrategyReward(strategy, reward)
}

func (s *Service) Feedback() map[string]FeedbackItem {
	rows, err := s.store.ListTrackFeedback()
	if err != nil {
		return nil
	}
	out := map[string]FeedbackItem{}
	for key, row := range rows {
		out[key] = FeedbackItem{TrackID: row.TrackID, LastPlayedAt: row.LastPlayedAt, LastSkippedAt: row.LastSkippedAt, AvgCompletion: row.AvgCompletion, Affinity: row.Affinity, PlayEvents: row.PlayEvents, SkipEvents: row.SkipEvents, CompleteEvents: row.CompleteEvents, LastEventType: row.LastEventType}
	}
	return out
}

func (s *Service) RecentTrackIDs(limit int) []string {
	ids, err := s.store.RecentTrackIDs(limit)
	if err != nil {
		return nil
	}
	return ids
}

func (s *Service) StrategyStats() map[string]db.StrategyStat {
	stats, err := s.store.ListStrategyStats()
	if err != nil {
		return nil
	}
	return stats
}
func classifySkipEvent(durationMs, pos int, source string) string {
	if source == "manual_next" {
		return "manual_next"
	}
	if durationMs <= 0 {
		if pos < 30000 {
			return "early_skip"
		}
		return "late_skip"
	}
	progress := float64(pos) / float64(durationMs)
	if pos < 30000 || progress < 0.30 {
		return "early_skip"
	}
	return "late_skip"
}

func (s *Service) History() []HistoryItem {
	rows, err := s.store.History(50)
	if err != nil {
		return nil
	}
	out := make([]HistoryItem, 0, len(rows))
	for _, row := range rows {
		if t, ok := s.library.TrackByID(row.TrackID); ok {
			out = append(out, HistoryItem{Track: t, PositionMs: row.PositionMs, Progress: row.Progress, ProgressLabel: row.ProgressLabel, PlayedAtLabel: row.PlayedAtLabel})
		}
	}
	return out
}

func (s *Service) MarkTransitionStarted(track library.Track, pos int) error {
	return s.store.RecordEvent(track.ID, "transition_started", pos, track.DurationMs)
}

func (s *Service) MarkTransitionSurvived30(track library.Track, pos int) error {
	return s.store.RecordEvent(track.ID, "transition_survived_30s", pos, track.DurationMs)
}

func (s *Service) MarkTransitionSurvived60(track library.Track, pos int) error {
	return s.store.RecordEvent(track.ID, "transition_survived_60s", pos, track.DurationMs)
}

func (s *Service) MarkTransitionSkippedEarly(track library.Track, pos int) error {
	return s.store.RecordEvent(track.ID, "transition_skipped_early", pos, track.DurationMs)
}

func (s *Service) MarkTransitionCompleted(track library.Track) error {
	return s.store.RecordEvent(track.ID, "transition_completed", track.DurationMs, track.DurationMs)
}

func (s *Service) RecordTransition(prevTrackID, nextTrackID string, strategy string, bucket string, transitionType string, energyDirection string, isBridge bool, isDiscovery bool) error {
	if prevTrackID == "" || nextTrackID == "" {
		return nil
	}
	_ = s.store.RecordTransitionEvent(prevTrackID, nextTrackID, strategy, bucket, transitionType, energyDirection, isBridge, isDiscovery)
	return nil
}
