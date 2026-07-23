package podcast

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dhowden/tag"

	"ray-player1/internal/audio"
	"ray-player1/internal/db"
	"ray-player1/internal/library"
)

type RayContentMode string

const (
	ContentRecommended   RayContentMode = "recommended"
	ContentExplore       RayContentMode = "explore"
	ContentCurrentFolder RayContentMode = "current_folder"
)

type RaySortMode string

const (
	SortRecommended RaySortMode = "recommended"
	SortNameAsc     RaySortMode = "name_asc"
	SortNameDesc    RaySortMode = "name_desc"
	SortDateDesc    RaySortMode = "date_desc"
	SortDateAsc     RaySortMode = "date_asc"
	SortManual      RaySortMode = "manual"
)

type Scope struct {
	SeedFolder        string `json:"seedFolder"`
	IncludeSubfolders bool   `json:"includeSubfolders"`
	PreferSameFolder  bool   `json:"preferSameFolder"`
	AllowOutside      bool   `json:"allowOutside"`
}

type RayWeights struct {
	SemanticSimilarity float64 `json:"semanticSimilarity"`
	FolderAffinity     float64 `json:"folderAffinity"`
	SeriesAffinity     float64 `json:"seriesAffinity"`
	ResumeValue        float64 `json:"resumeValue"`
	Novelty            float64 `json:"novelty"`
	Freshness          float64 `json:"freshness"`
	UserTaste          float64 `json:"userTaste"`
	SkipPenalty        float64 `json:"skipPenalty"`
	RecentPenalty      float64 `json:"recentPenalty"`
	TopicBridge        float64 `json:"topicBridge"`
}

type RayFilters struct {
	ExcludeCompleted      bool    `json:"excludeCompleted"`
	ExcludeHardSkipped    bool    `json:"excludeHardSkipped"`
	MinSemanticSimilarity float64 `json:"minSemanticSimilarity"`
	MaxItems              int     `json:"maxItems"`
}

type RayConfig struct {
	ContentMode        RayContentMode `json:"contentMode"`
	SortMode           RaySortMode    `json:"sortMode"`
	SeedItemID         string         `json:"seedItemId"`
	Scope              Scope          `json:"scope"`
	Weights            RayWeights     `json:"weights"`
	Filters            RayFilters     `json:"filters"`
	CreatedWithVersion int            `json:"createdWithVersion"`
}

type Item struct {
	ID               string  `json:"id"`
	Path             string  `json:"path"`
	Title            string  `json:"title"`
	Author           string  `json:"author"`
	Series           string  `json:"series"`
	Folder           string  `json:"folder"`
	Duration         float64 `json:"duration"`
	FileSize         int64   `json:"fileSize"`
	TranscriptPath   string  `json:"transcriptPath"`
	TranscriptStatus string  `json:"transcriptStatus"`
	SemanticStatus   string  `json:"semanticStatus"`
	Summary          string  `json:"summary"`
	LastPosition     float64 `json:"lastPosition"`
	CompletedRatio   float64 `json:"completedRatio"`
	IsCompleted      bool    `json:"isCompleted"`
	PlayCount        int     `json:"playCount"`
	SkipCount        int     `json:"skipCount"`
	LastPlayedAt     int64   `json:"lastPlayedAt"`
	LastError        string  `json:"lastError"`
	ImportedAt       int64   `json:"importedAt"`
	ModifiedAt       int64   `json:"modifiedAt"`

	SourceType       string  `json:"sourceType"`
	SourceURL        string  `json:"sourceUrl"`
	SourceSite       string  `json:"sourceSite"`
	ExternalID       string  `json:"externalId"`
	DownloadStatus   string  `json:"downloadStatus"`
	DownloadProgress float64 `json:"downloadProgress"`
	DownloadError    string  `json:"downloadError"`
	DownloadAttempts int     `json:"downloadAttempts"`
	DownloadedAt     int64   `json:"downloadedAt"`

	ResumePosition     float64 `json:"resumePosition"`
	DurationLabel      string  `json:"durationLabel"`
	ProgressPercentage int     `json:"progressPercentage"`
}

type RayItem struct {
	Item             Item    `json:"item"`
	Position         int     `json:"position"`
	OriginalPosition int     `json:"originalPosition"`
	Reason           string  `json:"reason"`
	Score            float64 `json:"score"`
	SemanticScore    float64 `json:"semanticScore"`
	FolderScore      float64 `json:"folderScore"`
	NoveltyScore     float64 `json:"noveltyScore"`
	ResumeScore      float64 `json:"resumeScore"`
	AddedBy          string  `json:"addedBy"`
	Current          bool    `json:"current"`
}

type Ray struct {
	ID              string         `json:"id"`
	SeedItemID      string         `json:"seedItemId"`
	Title           string         `json:"title"`
	Mode            string         `json:"mode"`
	ContentMode     RayContentMode `json:"contentMode"`
	SortMode        RaySortMode    `json:"sortMode"`
	Config          RayConfig      `json:"config"`
	IsManualOrder   bool           `json:"isManualOrder"`
	ManualUpdatedAt int64          `json:"manualUpdatedAt"`
	ParentRayID     string         `json:"parentRayId"`
	Revision        int            `json:"revision"`
	CreatedAt       int64          `json:"createdAt"`
	FolderScope     string         `json:"folderScope"`
	CurrentIndex    int            `json:"currentIndex"`
	Items           []RayItem      `json:"items"`
}

type Playback struct {
	ItemID      string `json:"itemId"`
	RayID       string `json:"rayId"`
	QueueIndex  int    `json:"queueIndex"`
	QueueLength int    `json:"queueLength"`
	ResumeMs    int    `json:"resumeMs"`
	DurationMs  int    `json:"durationMs"`
	Title       string `json:"title"`
	Author      string `json:"author"`
}

type ImportResult struct {
	InputCount     int      `json:"inputCount"`
	AudioFound     int      `json:"audioFound"`
	AddedOrUpdated int      `json:"addedOrUpdated"`
	Skipped        int      `json:"skipped"`
	Errors         []string `json:"errors"`
}

type Service struct {
	store *db.Store

	mu         sync.RWMutex
	currentRay Ray
}

func NewService(store *db.Store) *Service {
	return &Service{
		store:      store,
		currentRay: Ray{CurrentIndex: -1},
	}
}

func (s *Service) List(limit int) ([]Item, error) {
	rows, err := s.store.ListPodcastItems(limit)
	if err != nil {
		return nil, err
	}
	return itemsFromRows(rows), nil
}

func (s *Service) Search(query string, limit int) ([]Item, error) {
	rows, err := s.store.SearchPodcastItems(query, limit)
	if err != nil {
		return nil, err
	}
	return itemsFromRows(rows), nil
}

func (s *Service) ItemByID(id string) (Item, error) {
	row, err := s.store.PodcastItemByID(id)
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(row), nil
}

func (s *Service) CurrentRay() Ray {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ray := s.currentRay
	ray.Items = append([]RayItem(nil), s.currentRay.Items...)
	return ray
}

func DefaultRayConfig(
	seed Item,
	count int,
) RayConfig {
	if count <= 0 {
		count = 20
	}

	return RayConfig{
		ContentMode: ContentRecommended,
		SortMode:    SortRecommended,
		SeedItemID:  seed.ID,
		Scope: Scope{
			SeedFolder:        seed.Folder,
			IncludeSubfolders: true,
			PreferSameFolder:  true,
			AllowOutside:      true,
		},
		Weights: weightsForContentMode(ContentRecommended),
		Filters: RayFilters{
			ExcludeCompleted:      false,
			ExcludeHardSkipped:    true,
			MinSemanticSimilarity: 0,
			MaxItems:              count,
		},
		CreatedWithVersion: 1,
	}
}

func weightsForContentMode(
	mode RayContentMode,
) RayWeights {
	switch mode {
	case ContentExplore:
		return RayWeights{
			SemanticSimilarity: 0.30,
			FolderAffinity:     0.08,
			SeriesAffinity:     0,
			ResumeValue:        0.06,
			Novelty:            0.16,
			Freshness:          0.12,
			UserTaste:          0.10,
			SkipPenalty:        0.12,
			RecentPenalty:      0.10,
			TopicBridge:        0.18,
		}

	case ContentCurrentFolder:
		return RayWeights{
			SemanticSimilarity: 0.28,
			FolderAffinity:     0.36,
			SeriesAffinity:     0.12,
			ResumeValue:        0.12,
			Novelty:            0.07,
			Freshness:          0.05,
			SkipPenalty:        0.15,
			RecentPenalty:      0.10,
		}

	default:
		return RayWeights{
			SemanticSimilarity: 0.38,
			FolderAffinity:     0.22,
			SeriesAffinity:     0.12,
			ResumeValue:        0.12,
			UserTaste:          0.08,
			Novelty:            0.08,
			SkipPenalty:        0.15,
			RecentPenalty:      0.10,
		}
	}
}

func (s *Service) UpdateProgress(
	id string,
	position float64,
	duration float64,
) (Item, error) {
	row, err := s.store.UpdatePodcastProgress(id, position, duration)
	if err != nil {
		return Item{}, err
	}

	item := itemFromRow(row)

	for index := range s.currentRay.Items {
		if s.currentRay.Items[index].Item.ID != id {
			continue
		}

		current := s.currentRay.Items[index].Current
		s.currentRay.Items[index].Item = item
		s.currentRay.Items[index].Current = current
		break
	}

	return item, nil
}

func (s *Service) BuildRay(
	seedID string,
	count int,
) (Ray, error) {
	seed, err := s.ItemByID(seedID)
	if err != nil {
		return Ray{}, err
	}
	return s.BuildRayWithConfig(
		DefaultRayConfig(seed, count),
		"",
	)
}

func (s *Service) BuildRayWithConfig(
	config RayConfig,
	existingRayID string,
) (Ray, error) {
	if config.Filters.MaxItems <= 0 {
		config.Filters.MaxItems = 20
	}

	seedRow, err := s.store.PodcastItemByID(
		config.SeedItemID,
	)
	if err != nil {
		return Ray{}, err
	}
	if seedRow.SourceType == "yt_dlp" &&
		seedRow.DownloadStatus != "ready" {
		return Ray{}, fmt.Errorf(
			"podcast seed is not downloaded yet: %s",
			seedRow.DownloadStatus,
		)
	}

	config.ContentMode = normalizeContentMode(
		config.ContentMode,
	)
	config.SortMode = normalizeSortMode(config.SortMode)
	config.Scope.SeedFolder = seedRow.Folder
	config.Weights = weightsForContentMode(
		config.ContentMode,
	)

	allRows, err := s.store.ListPodcastItems(2000)
	if err != nil {
		return Ray{}, err
	}

	type candidate struct {
		row      db.PodcastItemRow
		score    float64
		reason   string
		semantic float64
		folder   float64
		novelty  float64
		resume   float64
	}

	candidates := make([]candidate, 0, len(allRows))
	for _, row := range allRows {
		if row.ID == seedRow.ID {
			continue
		}
		if row.SourceType == "yt_dlp" &&
			(row.DownloadStatus != "ready" ||
				strings.TrimSpace(row.Path) == "") {
			continue
		}

		if config.ContentMode == ContentCurrentFolder &&
			!isInFolderScope(
				seedRow.Folder,
				row.Folder,
				config.Scope.IncludeSubfolders,
			) {
			continue
		}
		if config.Filters.ExcludeCompleted &&
			row.IsCompleted {
			continue
		}
		if config.Filters.ExcludeHardSkipped &&
			row.SkipCount >= 3 {
			continue
		}

		semantic := metadataSimilarity(seedRow, row)
		folder := folderAffinity(seedRow.Folder, row.Folder)
		series := stringAffinity(seedRow.Series, row.Series)
		author := stringAffinity(seedRow.Author, row.Author)
		resume := resumeValue(row)
		novelty := 1.0
		if row.PlayCount > 0 {
			novelty = 1 / float64(row.PlayCount+1)
		}

		if semantic <
			config.Filters.MinSemanticSimilarity {
			continue
		}

		freshness := freshnessValue(row.AddedAt)
		userTaste := math.Min(
			1,
			float64(row.PlayCount)/4,
		)
		skipPenalty := math.Min(
			1,
			float64(row.SkipCount)/3,
		)
		recentPenalty := recentPlayPenalty(
			row.LastPlayedAt,
		)
		bridge := topicBridgeScore(semantic)

		weights := config.Weights
		score :=
			weights.SemanticSimilarity*semantic +
				weights.FolderAffinity*folder +
				weights.SeriesAffinity*series +
				weights.ResumeValue*resume +
				weights.Novelty*novelty +
				weights.Freshness*freshness +
				weights.UserTaste*userTaste +
				weights.TopicBridge*bridge -
				weights.SkipPenalty*skipPenalty -
				weights.RecentPenalty*recentPenalty

		score += 0.04 * author

		reason := "Смежный выпуск"
		switch {
		case config.ContentMode == ContentExplore &&
			bridge >= 0.8:
			reason = "Смежная тема"
		case folder >= 1:
			reason = "Из той же папки"
		case folder >= 0.65:
			reason = "Из соседней папки"
		case series > 0:
			reason = "Из той же серии"
		case resume > 0:
			reason = "Недослушано"
		}

		candidates = append(candidates, candidate{
			row:      row,
			score:    score,
			reason:   reason,
			semantic: semantic,
			folder:   folder,
			novelty:  novelty,
			resume:   resume,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].row.Title < candidates[j].row.Title
		}
		return candidates[i].score > candidates[j].score
	})

	rayID := existingRayID
	if rayID == "" {
		rayID = fmt.Sprintf(
			"podcast-ray-%d",
			time.Now().UnixNano(),
		)
	}

	ray := Ray{
		ID:           rayID,
		SeedItemID:   seedRow.ID,
		Title:        seedRow.Title,
		Mode:         string(config.ContentMode),
		ContentMode:  config.ContentMode,
		SortMode:     config.SortMode,
		Config:       config,
		Revision:     1,
		CreatedAt:    time.Now().Unix(),
		FolderScope:  seedRow.Folder,
		CurrentIndex: 0,
		Items: []RayItem{{
			Item:             itemFromRow(seedRow),
			Position:         0,
			OriginalPosition: 0,
			Reason:           "Начало луча",
			Score:            1,
			SemanticScore:    1,
			FolderScore:      1,
			NoveltyScore:     0,
			ResumeScore:      resumeValue(seedRow),
			AddedBy:          "generator",
			Current:          true,
		}},
	}

	for _, candidate := range candidates {
		if len(ray.Items) >= config.Filters.MaxItems {
			break
		}
		position := len(ray.Items)
		ray.Items = append(ray.Items, RayItem{
			Item:             itemFromRow(candidate.row),
			Position:         position,
			OriginalPosition: position,
			Reason:           candidate.reason,
			Score:            candidate.score,
			SemanticScore:    candidate.semantic,
			FolderScore:      candidate.folder,
			NoveltyScore:     candidate.novelty,
			ResumeScore:      candidate.resume,
			AddedBy:          "generator",
		})
	}

	applySort(&ray, config.SortMode)
	if err := s.saveRay(ray); err != nil {
		return Ray{}, err
	}

	s.mu.Lock()
	s.currentRay = ray
	s.mu.Unlock()

	return cloneRay(ray), nil
}

func (s *Service) SelectRayItem(itemID string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.currentRay.Items {
		isCurrent := s.currentRay.Items[index].Item.ID == itemID
		s.currentRay.Items[index].Current = isCurrent
		if isCurrent {
			s.currentRay.CurrentIndex = index
			return s.currentRay.Items[index].Item, nil
		}
	}

	return s.ItemByID(itemID)
}

func (s *Service) NextRayItem() (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.currentRay.CurrentIndex + 1
	if next < 0 || next >= len(s.currentRay.Items) {
		return Item{}, false
	}

	for index := range s.currentRay.Items {
		s.currentRay.Items[index].Current = index == next
	}
	s.currentRay.CurrentIndex = next
	return s.currentRay.Items[next].Item, true
}

func (s *Service) PreviousRayItem() (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.currentRay.CurrentIndex - 1
	if previous < 0 || previous >= len(s.currentRay.Items) {
		return Item{}, false
	}

	for index := range s.currentRay.Items {
		s.currentRay.Items[index].Current =
			index == previous
	}
	s.currentRay.CurrentIndex = previous
	return s.currentRay.Items[previous].Item, true
}

func (s *Service) ImportPaths(paths []string) (ImportResult, error) {
	result := ImportResult{InputCount: len(paths)}
	files := make([]string, 0)

	for _, input := range paths {
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		info, err := os.Stat(input)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		if !info.IsDir() {
			if isAudioFile(input) {
				files = append(files, input)
			} else {
				result.Skipped++
			}
			continue
		}

		err = filepath.WalkDir(input, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				result.Errors = append(result.Errors, walkErr.Error())
				return nil
			}
			if !entry.IsDir() && isAudioFile(path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	sort.Strings(files)
	result.AudioFound = len(files)

	for _, path := range files {
		item, err := itemFromFile(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if err := s.store.UpsertPodcastItem(item); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		result.AddedOrUpdated++
	}

	if len(result.Errors) > 0 && result.AddedOrUpdated == 0 {
		return result, errors.New("podcast import failed")
	}
	return result, nil
}

func itemFromFile(path string) (db.PodcastItemRow, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return db.PodcastItemRow{}, err
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return db.PodcastItemRow{}, err
	}

	title := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
	author := ""
	series := ""
	durationSeconds := 0.0

	if duration, probeErr := audio.ProbeDuration(absolutePath); probeErr == nil {
		durationSeconds = duration.Seconds()
	}

	file, openErr := os.Open(absolutePath)
	if openErr == nil {
		if metadata, metadataErr := tag.ReadFrom(file); metadataErr == nil {
			if value := strings.TrimSpace(metadata.Title()); value != "" {
				title = value
			}
			author = strings.TrimSpace(metadata.Artist())
			series = strings.TrimSpace(metadata.Album())
		}
		_ = file.Close()
	}

	now := time.Now().Unix()
	return db.PodcastItemRow{
		ID:             stableID(absolutePath),
		Path:           absolutePath,
		Title:          title,
		Author:         author,
		Series:         series,
		Folder:         filepath.Dir(absolutePath),
		Duration:       durationSeconds,
		FileSize:       info.Size(),
		AddedAt:        now,
		UpdatedAt:      now,
		ModifiedAt:     info.ModTime().Unix(),
		SemanticStatus: "metadata_ready",
	}, nil
}

func stableID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return "podcast_" + hex.EncodeToString(sum[:12])
}

func isAudioFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".flac", ".wav", ".ogg", ".m4a", ".aac",
		".opus", ".wma", ".aiff", ".aif":
		return true
	default:
		return false
	}
}

func AsTrack(item Item) library.Track {
	return library.Track{
		ID:            item.ID,
		Path:          item.Path,
		Title:         item.Title,
		Artist:        item.Author,
		Album:         item.Series,
		Folder:        item.Folder,
		DurationMs:    int(item.Duration * 1000),
		DurationLabel: item.DurationLabel,
		ImportStatus:  string(library.ImportReady),
	}
}

func folderAffinity(seedFolder, candidateFolder string) float64 {
	seedFolder = filepath.Clean(seedFolder)
	candidateFolder = filepath.Clean(candidateFolder)

	if seedFolder == candidateFolder {
		return 1
	}
	if strings.HasPrefix(candidateFolder, seedFolder+string(filepath.Separator)) {
		return 0.85
	}
	if filepath.Dir(seedFolder) == filepath.Dir(candidateFolder) {
		return 0.65
	}
	return 0
}

func stringAffinity(left, right string) float64 {
	left = strings.TrimSpace(strings.ToLower(left))
	right = strings.TrimSpace(strings.ToLower(right))
	if left != "" && left == right {
		return 1
	}
	return 0
}

func resumeValue(row db.PodcastItemRow) float64 {
	if row.IsCompleted || row.CompletedRatio >= 0.95 {
		return -0.2
	}
	if row.LastPosition > 60 && row.CompletedRatio > 0.05 {
		return math.Max(0, 1-row.CompletedRatio)
	}
	return 0
}

func (s *Service) SetCurrentRaySortMode(
	mode RaySortMode,
) (Ray, error) {
	mode = normalizeSortMode(mode)

	s.mu.Lock()
	if s.currentRay.ID == "" {
		s.mu.Unlock()
		return Ray{}, errors.New("podcast ray is not built")
	}

	ray := cloneRay(s.currentRay)
	applySort(&ray, mode)
	s.currentRay = ray
	s.mu.Unlock()

	if err := s.saveRay(ray); err != nil {
		return Ray{}, err
	}
	return cloneRay(ray), nil
}

func (s *Service) SetCurrentRayContentMode(
	mode RayContentMode,
) (Ray, error) {
	mode = normalizeContentMode(mode)

	s.mu.RLock()
	current := cloneRay(s.currentRay)
	s.mu.RUnlock()

	if current.ID == "" {
		return Ray{}, errors.New("podcast ray is not built")
	}

	config := current.Config
	config.ContentMode = mode
	config.Weights = weightsForContentMode(mode)
	config.SeedItemID = current.SeedItemID
	config.Scope.SeedFolder = current.FolderScope

	if current.SortMode == SortManual ||
		current.IsManualOrder {
		config.SortMode = SortRecommended
	}

	next, err := s.BuildRayWithConfig(config, "")
	if err != nil {
		return Ray{}, err
	}

	next.ParentRayID = current.ID
	next.Revision = current.Revision + 1
	if next.Revision <= 1 {
		next.Revision = 2
	}
	if err := s.saveRay(next); err != nil {
		return Ray{}, err
	}
	return cloneRay(next), nil
}

func (s *Service) MoveCurrentRayItem(
	from int,
	to int,
) (Ray, error) {
	s.mu.Lock()
	if s.currentRay.ID == "" {
		s.mu.Unlock()
		return Ray{}, errors.New("podcast ray is not built")
	}
	if from < 0 ||
		from >= len(s.currentRay.Items) ||
		to < 0 ||
		to >= len(s.currentRay.Items) {
		s.mu.Unlock()
		return Ray{}, fmt.Errorf(
			"podcast ray move out of range: %d -> %d",
			from,
			to,
		)
	}
	if from == to {
		ray := cloneRay(s.currentRay)
		s.mu.Unlock()
		return ray, nil
	}

	currentID := currentRayItemID(s.currentRay)
	item := s.currentRay.Items[from]
	items := append(
		s.currentRay.Items[:from],
		s.currentRay.Items[from+1:]...,
	)

	items = append(items, RayItem{})
	copy(items[to+1:], items[to:])
	items[to] = item
	s.currentRay.Items = items

	s.currentRay.SortMode = SortManual
	s.currentRay.Config.SortMode = SortManual
	s.currentRay.IsManualOrder = true
	s.currentRay.ManualUpdatedAt =
		time.Now().UnixMilli()
	reindexRayItems(&s.currentRay, currentID)

	ray := cloneRay(s.currentRay)
	s.mu.Unlock()

	if err := s.saveRay(ray); err != nil {
		return Ray{}, err
	}
	return ray, nil
}

func (s *Service) RemoveCurrentRayItem(
	itemID string,
) (Ray, error) {
	s.mu.Lock()
	if s.currentRay.ID == "" {
		s.mu.Unlock()
		return Ray{}, errors.New("podcast ray is not built")
	}
	if itemID == s.currentRay.SeedItemID {
		s.mu.Unlock()
		return Ray{}, errors.New(
			"seed podcast cannot be removed from its ray",
		)
	}

	currentID := currentRayItemID(s.currentRay)
	filtered := make(
		[]RayItem,
		0,
		len(s.currentRay.Items),
	)
	found := false
	for _, item := range s.currentRay.Items {
		if item.Item.ID == itemID {
			found = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !found {
		ray := cloneRay(s.currentRay)
		s.mu.Unlock()
		return ray, nil
	}

	s.currentRay.Items = filtered
	reindexRayItems(&s.currentRay, currentID)
	ray := cloneRay(s.currentRay)
	s.mu.Unlock()

	if err := s.saveRay(ray); err != nil {
		return Ray{}, err
	}
	return ray, nil
}

func applySort(ray *Ray, mode RaySortMode) {
	currentID := currentRayItemID(*ray)
	ray.SortMode = mode
	ray.Config.SortMode = mode

	switch mode {
	case SortNameAsc:
		sort.SliceStable(ray.Items, func(i, j int) bool {
			return normalizeTitleForSort(
				ray.Items[i].Item.Title,
			) < normalizeTitleForSort(
				ray.Items[j].Item.Title,
			)
		})
		ray.IsManualOrder = false

	case SortNameDesc:
		sort.SliceStable(ray.Items, func(i, j int) bool {
			return normalizeTitleForSort(
				ray.Items[i].Item.Title,
			) > normalizeTitleForSort(
				ray.Items[j].Item.Title,
			)
		})
		ray.IsManualOrder = false

	case SortDateDesc:
		sort.SliceStable(ray.Items, func(i, j int) bool {
			return ray.Items[i].Item.ImportedAt >
				ray.Items[j].Item.ImportedAt
		})
		ray.IsManualOrder = false

	case SortDateAsc:
		sort.SliceStable(ray.Items, func(i, j int) bool {
			return ray.Items[i].Item.ImportedAt <
				ray.Items[j].Item.ImportedAt
		})
		ray.IsManualOrder = false

	case SortManual:
		ray.IsManualOrder = true

	default:
		ray.SortMode = SortRecommended
		ray.Config.SortMode = SortRecommended
		sort.SliceStable(ray.Items, func(i, j int) bool {
			if ray.Items[i].Score ==
				ray.Items[j].Score {
				return ray.Items[i].OriginalPosition <
					ray.Items[j].OriginalPosition
			}
			return ray.Items[i].Score >
				ray.Items[j].Score
		})
		ray.IsManualOrder = false
	}

	if !ray.IsManualOrder {
		ray.ManualUpdatedAt = 0
	}
	reindexRayItems(ray, currentID)
}

func reindexRayItems(ray *Ray, currentID string) {
	ray.CurrentIndex = -1
	for index := range ray.Items {
		ray.Items[index].Position = index
		ray.Items[index].Current =
			ray.Items[index].Item.ID == currentID
		if ray.Items[index].Current {
			ray.CurrentIndex = index
		}
	}
	if ray.CurrentIndex < 0 && len(ray.Items) > 0 {
		ray.CurrentIndex = 0
		ray.Items[0].Current = true
	}
}

func currentRayItemID(ray Ray) string {
	if ray.CurrentIndex >= 0 &&
		ray.CurrentIndex < len(ray.Items) {
		return ray.Items[ray.CurrentIndex].Item.ID
	}
	for _, item := range ray.Items {
		if item.Current {
			return item.Item.ID
		}
	}
	return ray.SeedItemID
}

func normalizeTitleForSort(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, prefix := range []string{"the ", "a ", "an "} {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}

func normalizeContentMode(
	mode RayContentMode,
) RayContentMode {
	switch mode {
	case ContentExplore, ContentCurrentFolder:
		return mode
	default:
		return ContentRecommended
	}
}

func normalizeSortMode(mode RaySortMode) RaySortMode {
	switch mode {
	case SortNameAsc,
		SortNameDesc,
		SortDateDesc,
		SortDateAsc,
		SortManual:
		return mode
	default:
		return SortRecommended
	}
}

func metadataSimilarity(
	seed db.PodcastItemRow,
	candidate db.PodcastItemRow,
) float64 {
	left := podcastTokens(seed)
	right := podcastTokens(candidate)
	if len(left) == 0 || len(right) == 0 {
		return 0
	}

	intersection := 0
	union := make(map[string]struct{}, len(left)+len(right))
	for token := range left {
		union[token] = struct{}{}
		if _, ok := right[token]; ok {
			intersection++
		}
	}
	for token := range right {
		union[token] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func podcastTokens(
	item db.PodcastItemRow,
) map[string]struct{} {
	text := strings.Join([]string{
		item.Title,
		item.Author,
		item.Series,
		filepath.Base(item.Folder),
		item.Summary,
		item.KeywordsJSON,
	}, " ")

	var builder strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteByte(' ')
		}
	}

	tokens := make(map[string]struct{})
	for _, token := range strings.Fields(builder.String()) {
		if len([]rune(token)) < 3 {
			continue
		}
		tokens[token] = struct{}{}
	}
	return tokens
}

func topicBridgeScore(similarity float64) float64 {
	switch {
	case similarity < 0.35:
		return 0
	case similarity >= 0.55 && similarity <= 0.75:
		return 1
	case similarity > 0.85:
		return 0.35
	default:
		return 0.65
	}
}

func freshnessValue(importedAt int64) float64 {
	if importedAt <= 0 {
		return 0
	}
	age := time.Since(time.Unix(importedAt, 0))
	switch {
	case age <= 7*24*time.Hour:
		return 1
	case age <= 30*24*time.Hour:
		return 0.75
	case age <= 180*24*time.Hour:
		return 0.45
	default:
		return 0.2
	}
}

func recentPlayPenalty(lastPlayedAt int64) float64 {
	if lastPlayedAt <= 0 {
		return 0
	}
	age := time.Since(time.Unix(lastPlayedAt, 0))
	switch {
	case age <= 24*time.Hour:
		return 1
	case age <= 7*24*time.Hour:
		return 0.65
	case age <= 30*24*time.Hour:
		return 0.25
	default:
		return 0
	}
}

func isInFolderScope(
	seedFolder string,
	candidateFolder string,
	includeSubfolders bool,
) bool {
	seedFolder = filepath.Clean(seedFolder)
	candidateFolder = filepath.Clean(candidateFolder)
	if candidateFolder == seedFolder {
		return true
	}
	if !includeSubfolders {
		return false
	}

	relative, err := filepath.Rel(
		seedFolder,
		candidateFolder,
	)
	if err != nil {
		return false
	}
	return relative != ".." &&
		relative != "." &&
		!strings.HasPrefix(
			relative,
			".."+string(filepath.Separator),
		)
}

func (s *Service) OpenSavedRay(rayID string, currentItemID string) (Ray, error) {
	row, err := s.store.PodcastRayByID(rayID)
	if err != nil {
		return Ray{}, err
	}

	itemRows, err := s.store.ListPodcastRayItems(rayID)
	if err != nil {
		return Ray{}, err
	}

	config := RayConfig{}
	if strings.TrimSpace(row.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(row.ConfigJSON), &config); err != nil {
			return Ray{}, fmt.Errorf("decode saved podcast ray config: %w", err)
		}
	}

	ray := Ray{
		ID:              row.ID,
		SeedItemID:      row.SeedItemID,
		Title:           row.Title,
		Mode:            row.Mode,
		ContentMode:     RayContentMode(row.ContentMode),
		SortMode:        RaySortMode(row.SortMode),
		Config:          config,
		IsManualOrder:   row.IsManualOrder,
		ManualUpdatedAt: row.ManualUpdatedAt,
		ParentRayID:     row.ParentRayID,
		Revision:        row.Revision,
		CreatedAt:       row.CreatedAt,
		FolderScope:     row.FolderScope,
		CurrentIndex:    -1,
		Items:           make([]RayItem, 0, len(itemRows)),
	}

	for _, itemRow := range itemRows {
		ray.Items = append(ray.Items, RayItem{
			Item:             itemFromRow(itemRow.Item),
			Position:         itemRow.PositionIndex,
			OriginalPosition: itemRow.OriginalPosition,
			Reason:           itemRow.Reason,
			Score:            itemRow.Score,
			SemanticScore:    itemRow.SemanticScore,
			FolderScore:      itemRow.FolderScore,
			NoveltyScore:     itemRow.NoveltyScore,
			ResumeScore:      itemRow.ResumeScore,
			AddedBy:          itemRow.AddedBy,
		})
	}

	for index := range ray.Items {
		isCurrent := currentItemID != "" && ray.Items[index].Item.ID == currentItemID
		ray.Items[index].Current = isCurrent
		if isCurrent {
			ray.CurrentIndex = index
		}
	}

	s.mu.Lock()
	s.currentRay = ray
	s.mu.Unlock()

	return cloneRay(ray), nil
}

func (s *Service) saveRay(ray Ray) error {
	configJSON, err := json.Marshal(ray.Config)
	if err != nil {
		return fmt.Errorf(
			"encode podcast ray config: %w",
			err,
		)
	}

	rows := make(
		[]db.PodcastRayItemRow,
		0,
		len(ray.Items),
	)
	for _, item := range ray.Items {
		row, err := s.store.PodcastItemByID(item.Item.ID)
		if err != nil {
			return err
		}
		rows = append(rows, db.PodcastRayItemRow{
			Item:             row,
			PositionIndex:    item.Position,
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

	createdAt := ray.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	return s.store.SavePodcastRaySnapshot(
		db.PodcastRayRow{
			ID:              ray.ID,
			SeedItemID:      ray.SeedItemID,
			Title:           ray.Title,
			Mode:            ray.Mode,
			ContentMode:     string(ray.ContentMode),
			SortMode:        string(ray.SortMode),
			ConfigJSON:      string(configJSON),
			IsManualOrder:   ray.IsManualOrder,
			ManualUpdatedAt: ray.ManualUpdatedAt,
			ParentRayID:     ray.ParentRayID,
			Revision:        ray.Revision,
			CreatedAt:       createdAt,
			FolderScope:     ray.FolderScope,
		},
		rows,
	)
}

func cloneRay(ray Ray) Ray {
	out := ray
	out.Items = append([]RayItem(nil), ray.Items...)
	return out
}

func itemsFromRows(rows []db.PodcastItemRow) []Item {
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, itemFromRow(row))
	}
	return items
}

func itemFromRow(row db.PodcastItemRow) Item {
	progress := int(row.CompletedRatio*100 + 0.5)
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	return Item{
		ID:                 row.ID,
		Path:               row.Path,
		Title:              row.Title,
		Author:             row.Author,
		Series:             row.Series,
		Folder:             row.Folder,
		Duration:           row.Duration,
		FileSize:           row.FileSize,
		TranscriptPath:     row.TranscriptPath,
		TranscriptStatus:   row.TranscriptStatus,
		SemanticStatus:     row.SemanticStatus,
		Summary:            row.Summary,
		LastPosition:       row.LastPosition,
		CompletedRatio:     row.CompletedRatio,
		IsCompleted:        row.IsCompleted,
		PlayCount:          row.PlayCount,
		SkipCount:          row.SkipCount,
		LastPlayedAt:       row.LastPlayedAt,
		LastError:          row.LastError,
		ImportedAt:         row.AddedAt,
		ModifiedAt:         row.ModifiedAt,
		SourceType:         row.SourceType,
		SourceURL:          row.SourceURL,
		SourceSite:         row.SourceSite,
		ExternalID:         row.ExternalID,
		DownloadStatus:     row.DownloadStatus,
		DownloadProgress:   row.DownloadProgress,
		DownloadError:      row.DownloadError,
		DownloadAttempts:   row.DownloadAttempts,
		DownloadedAt:       row.DownloadedAt,
		ResumePosition:     ResumePosition(row.LastPosition, row.IsCompleted),
		DurationLabel:      durationLabel(row.Duration),
		ProgressPercentage: progress,
	}
}

func ResumePosition(position float64, isCompleted bool) float64 {
	// Завершённый эпизод или короткое начало — с начала.
	if isCompleted || position < 30 {
		return 0
	}
	return position - 5
}

func durationLabel(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	total := int(seconds + 0.5)
	hours := total / 3600
	minutes := (total % 3600) / 60
	secs := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}
