package library

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/db"
	"ray-player1/internal/logx"
	"ray-player1/internal/modelcontract"
	"ray-player1/internal/onnx"

	"github.com/dhowden/tag"
)

var libLog = logx.New("library")

type Track struct {
	ID                   string          `json:"id"`
	Path                 string          `json:"path"`
	Title                string          `json:"title"`
	Artist               string          `json:"artist"`
	Album                string          `json:"album"`
	ImportedAt           int64           `json:"importedAt"`
	Genre                string          `json:"genre"`
	GenrePrimary         string          `json:"genrePrimary"`
	GenreDetail          string          `json:"genreDetail"`
	GenreTags            []onnx.GenreTag `json:"genreTags"`
	GenreLabel           string          `json:"genreLabel"`
	DurationMs           int             `json:"durationMs"`
	DurationLabel        string          `json:"durationLabel"`
	Folder               string          `json:"folder"`
	FileName             string          `json:"fileName"`
	Tempo                float64         `json:"tempo"`
	BPMPerceived         float64         `json:"bpmPerceived"`
	TempoConfidence      float64         `json:"tempoConfidence"`
	TempoStability       float64         `json:"tempoStability"`
	BPMHalf              float64         `json:"bpmHalf"`
	BPMDouble            float64         `json:"bpmDouble"`
	TempoSource          string          `json:"tempoSource"`
	TempoModelVersion    string          `json:"tempoModelVersion"`
	TempoAnalyzedAt      int64           `json:"tempoAnalyzedAt"`
	TempoError           string          `json:"tempoError"`
	Energy               float64         `json:"energy"`
	Danceability         float64         `json:"danceability"`
	Valence              float64         `json:"valence"`
	Acousticness         float64         `json:"acousticness"`
	Electronicness       float64         `json:"electronicness"`
	Instrumentalness     float64         `json:"instrumentalness"`
	Vocalness            float64         `json:"vocalness"`
	Happy                float64         `json:"happy"`
	Sad                  float64         `json:"sad"`
	Relaxed              float64         `json:"relaxed"`
	Party                float64         `json:"party"`
	Aggressive           float64         `json:"aggressive"`
	TimbreBrightness     float64         `json:"timbreBrightness"`
	Tonality             float64         `json:"tonality"`
	Approachability      float64         `json:"approachability"`
	Engagement           float64         `json:"engagement"`
	Melodicness          float64         `json:"melodicness"`
	Softness             float64         `json:"softness"`
	Heaviness            float64         `json:"heaviness"`
	Dreaminess           float64         `json:"dreaminess"`
	Emotionality         float64         `json:"emotionality"`
	Loudness             float64         `json:"loudness"`
	SpectralCentroid     float64         `json:"spectralCentroid"`
	ZeroCrossingRate     float64         `json:"zeroCrossingRate"`
	RMS                  float64         `json:"rms"`
	SpectralFlatness     float64         `json:"spectralFlatness"`
	SpectralRolloff85    float64         `json:"spectralRolloff85"`
	SpectralFlux         float64         `json:"spectralFlux"`
	OnsetRate            float64         `json:"onsetRate"`
	DynamicRange         float64         `json:"dynamicRange"`
	LowBandRatio         float64         `json:"lowBandRatio"`
	MidBandRatio         float64         `json:"midBandRatio"`
	HighBandRatio        float64         `json:"highBandRatio"`
	ClusterID            int             `json:"clusterId"`
	PlayCount            int             `json:"playCount"`
	SkipCount            int             `json:"skipCount"`
	CompleteCount        int             `json:"completeCount"`
	MetadataSource       string          `json:"metadataSource"`
	AnalyzedLevel        int             `json:"analyzedLevel"`
	AnalysisVersion      int             `json:"analysisVersion"`
	AnalyzedAt           int64           `json:"analyzedAt"`
	AnalysisError        string          `json:"analysisError"`
	EssentiaModelVersion string          `json:"essentiaModelVersion"`
	NormalizedPath       string          `json:"normalizedPath"`
	LibraryRootID        string          `json:"libraryRootId"`
	ImportStatus         string          `json:"importStatus"`
	AnalysisStatus       string          `json:"analysisStatus"`
	FileMissing          bool            `json:"fileMissing"`
	FileSize             int64           `json:"fileSize"`
	FileMTime            int64           `json:"fileMtime"`
	FileInode            string          `json:"fileInode"`
	QuickHash            string          `json:"quickHash"`
	LastSeenAt           int64           `json:"lastSeenAt"`
	LastError            string          `json:"lastError"`
	Embedding            []float32       `json:"-"`
	TextEmbedding        []float32       `json:"-"`
	SearchText           string          `json:"-"`
	PlaybackErrorCount   int             `json:"playbackErrorCount"`
	LastPlaybackError    string          `json:"lastPlaybackError"`
	LastPlaybackErrorAt  int64           `json:"lastPlaybackErrorAt"`

	SourceType       string  `json:"sourceType"`
	SourceURL        string  `json:"sourceUrl"`
	SourceSite       string  `json:"sourceSite"`
	ExternalID       string  `json:"externalId"`
	DownloadStatus   string  `json:"downloadStatus"`
	DownloadProgress float64 `json:"downloadProgress"`
	DownloadError    string  `json:"downloadError"`
	DownloadAttempts int     `json:"downloadAttempts"`
	DownloadedAt     int64   `json:"downloadedAt"`
}

type LibraryStats struct {
	Tracks int `json:"tracks"`
	Roots  int `json:"roots"`
	Errors int `json:"errors"`
}

type analyzedHook func(Track)

type analysisJob struct {
	TrackID string
	Path    string
	Meta    map[string]string
	Source  string
	Force   bool
}

type RebuildProgress struct {
	Index   int    `json:"index"`
	Total   int    `json:"total"`
	TrackID string `json:"trackId"`
	Path    string `json:"path"`
	Stage   string `json:"stage"`
	State   string `json:"state"`
	Message string `json:"message"`
}

type IndexingState struct {
	IsIndexing   bool   `json:"isIndexing"`
	LibraryCount int    `json:"libraryCount"`
	Queued       int    `json:"queued"`
	Processed    int    `json:"processed"`
	Total        int    `json:"total"`
	CurrentPath  string `json:"currentPath,omitempty"`
	Phase        string `json:"phase"`
	StartedAt    int64  `json:"startedAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type Service struct {
	mu            sync.RWMutex
	store         *db.Store
	onnx          *onnx.Engine
	essentia      *onnx.EssentiaEngine
	tempo         *onnx.TempoEngine
	cache         map[string]Track
	order         []string
	analysisQueue chan analysisJob
	inFlight      map[string]bool
	requeue       map[string]analysisJob
	workerCount   int
	hook          analyzedHook
	indexingHook  func(IndexingState)
	indexing      IndexingState

	ctx       context.Context
	cancel    context.CancelFunc
	workers   sync.WaitGroup
	closeOnce sync.Once
	closed    bool
}

func NewService(store *db.Store, engine *onnx.Engine, essentia *onnx.EssentiaEngine, tempo *onnx.TempoEngine) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		store:         store,
		onnx:          engine,
		essentia:      essentia,
		tempo:         tempo,
		cache:         map[string]Track{},
		inFlight:      map[string]bool{},
		requeue:       map[string]analysisJob{},
		analysisQueue: make(chan analysisJob, 512),
		ctx:           ctx,
		cancel:        cancel,
	}
	s.workerCount = 1
	for i := 0; i < s.workerCount; i++ {
		s.workers.Add(1)
		go s.workerLoop()
	}
	return s
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.cancel()
		s.workers.Wait()
	})
}

func (s *Service) UpdateEngines(engine *onnx.Engine, essentia *onnx.EssentiaEngine, tempo *onnx.TempoEngine) {
	s.mu.Lock()
	s.onnx = engine
	s.essentia = essentia
	s.tempo = tempo
	if s.requeue == nil {
		s.requeue = map[string]analysisJob{}
	}
	jobs := s.collectEmbeddingRepairJobsLocked()
	pending := make([]analysisJob, 0, len(jobs))
	for _, job := range jobs {
		if s.inFlight[job.TrackID] {
			s.requeue[job.TrackID] = job
			continue
		}
		pending = append(pending, job)
	}
	s.mu.Unlock()
	if len(jobs) > 0 {
		s.beginIndexing(len(jobs), "repairing")
		libLog.I("queued embedding repairs after engine update count=%d deferred=%d", len(jobs), len(jobs)-len(pending))
	}
	s.enqueueAnalysisJobs(pending)
}

func (s *Service) SetAnalyzedHook(h func(Track)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hook = h
}

func (s *Service) SetIndexingHook(h func(IndexingState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indexingHook = h
}

func (s *Service) IndexingState() IndexingState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.indexing
	st.LibraryCount = len(s.cache)
	return st
}

func (s *Service) collectEmbeddingRepairJobsLocked() []analysisJob {
	jobs := make([]analysisJob, 0)
	for _, track := range s.cache {
		if !needsAnalysisRefresh(track) {
			continue
		}
		jobs = append(jobs, analysisJob{TrackID: track.ID, Path: track.Path, Source: "embedding-repair"})
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].TrackID < jobs[j].TrackID
	})
	return jobs
}

const currentAnalysisVersion = CurrentAnalysisVersion
const currentEssentiaModelVersion = CurrentEssentiaModelVersion

func (s *Service) beginIndexing(total int, phase string) {
	s.mu.Lock()
	now := time.Now().UnixMilli()
	if !s.indexing.IsIndexing {
		s.indexing.StartedAt = now
		s.indexing.Processed = 0
		s.indexing.Total = 0
	}
	s.indexing.IsIndexing = true
	s.indexing.Total += max(0, total)
	s.indexing.Queued += max(0, total)
	if phase != "" {
		s.indexing.Phase = phase
	}
	s.indexing.UpdatedAt = now
	hook := s.indexingHook
	st := s.indexing
	st.LibraryCount = len(s.cache)
	s.mu.Unlock()
	if hook != nil {
		hook(st)
	}
}

func (s *Service) updateIndexingCurrent(path, phase string) {
	s.mu.Lock()
	if !s.indexing.IsIndexing {
		s.mu.Unlock()
		return
	}
	s.indexing.CurrentPath = path
	if phase != "" {
		s.indexing.Phase = phase
	}
	s.indexing.UpdatedAt = time.Now().UnixMilli()
	hook := s.indexingHook
	st := s.indexing
	st.LibraryCount = len(s.cache)
	s.mu.Unlock()
	if hook != nil {
		hook(st)
	}
}

func (s *Service) finishIndexingOne(path string) {
	s.mu.Lock()
	if s.indexing.Queued > 0 {
		s.indexing.Queued--
	}
	s.indexing.Processed++
	s.indexing.CurrentPath = path
	s.indexing.Phase = "analyzing"
	s.indexing.UpdatedAt = time.Now().UnixMilli()
	if s.indexing.Processed >= s.indexing.Total && s.indexing.Total > 0 && s.indexing.Queued == 0 {
		s.indexing.IsIndexing = false
		s.indexing.Phase = "done"
	}
	hook := s.indexingHook
	st := s.indexing
	st.LibraryCount = len(s.cache)
	s.mu.Unlock()
	if hook != nil {
		hook(st)
	}
}

func (s *Service) emitIdleIndexing() {
	s.mu.RLock()
	hook := s.indexingHook
	st := s.indexing
	st.LibraryCount = len(s.cache)
	s.mu.RUnlock()
	if hook != nil {
		hook(st)
	}
}

func needsEmbeddingRepair(track Track) bool {
	if track.AnalyzedLevel <= 0 {
		return false
	}
	return len(track.Embedding) < 1000
}

func needsTempoReanalysis(track Track) bool {
	if track.TempoSource == "" || strings.EqualFold(strings.TrimSpace(track.TempoSource), "error") {
		return true
	}
	if strings.Contains(track.TempoError, "Invalid rank for input") {
		return true
	}
	if track.TempoConfidence <= 0 && track.TempoStability <= 0 {
		return true
	}
	return false
}

func needsAnalysisRefresh(track Track) bool {
	if track.AnalyzedLevel < 2 {
		return true
	}
	if track.AnalysisVersion < currentAnalysisVersion {
		return true
	}
	if strings.TrimSpace(track.EssentiaModelVersion) != currentEssentiaModelVersion {
		return true
	}
	if len(track.Embedding) < 1000 {
		return true
	}
	if needsTempoReanalysis(track) {
		return true
	}
	return false
}

func (s *Service) Load() error {
	rows, err := s.store.ListTracks()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cache = map[string]Track{}
	s.order = s.order[:0]
	for _, row := range rows {
		t := fromRow(row)
		s.cache[t.ID] = t
		s.order = append(s.order, t.ID)
	}
	s.sortLocked()
	jobs := s.collectEmbeddingRepairJobsLocked()
	s.mu.Unlock()
	if len(jobs) > 0 {
		s.beginIndexing(len(jobs), "repairing")
		libLog.I("queued embedding repairs count=%d", len(jobs))
	}
	s.enqueueAnalysisJobs(jobs)
	s.emitIdleIndexing()
	return nil
}

func (s *Service) RepairDurations() error {
	tracks := s.AllTracks()
	if len(tracks) == 0 {
		return nil
	}
	for _, track := range tracks {
		if !needsDurationRepair(track) {
			continue
		}
		ms := quickDurationMs(track.Path)
		if ms <= 0 || ms == track.DurationMs {
			continue
		}
		track.DurationMs = ms
		track.DurationLabel = formatDuration(time.Duration(ms) * time.Millisecond)
		dbStart := time.Now()
		if err := s.Upsert(track); err != nil {
			return err
		}
		libLog.I("rebuild duration repair upsert done id=%s ms=%d emb=%d", track.ID, time.Since(dbStart).Milliseconds(), len(track.Embedding))
	}
	return nil
}

func (s *Service) AllTracks() []Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Track, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.cache[id])
	}
	return out
}

func (s *Service) ReloadFromStore() error {
	rows, err := s.store.ListTracks()
	if err != nil {
		return err
	}

	cache := make(map[string]Track, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		track := TrackFromRow(row)
		cache[track.ID] = track
		order = append(order, track.ID)
	}

	s.mu.Lock()
	s.cache = cache
	s.order = order
	s.mu.Unlock()
	return nil
}

func (s *Service) AnalyzeExternalTrack(
	itemID string,
	path string,
) error {
	_, err := s.ImportPaths([]string{path}, nil)
	return err
}

func (s *Service) UsableTracks() []Track {
	tracks := s.AllTracks()
	out := make([]Track, 0, len(tracks))
	for _, track := range tracks {
		if track.IsUsableForRay() {
			out = append(out, track)
		}
	}
	return out
}

func (s *Service) TrackByID(id string) (Track, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.cache[id]
	return t, ok
}

func (s *Service) Stats() LibraryStats {
	roots, _ := s.store.ListLibraryRoots()
	errors, _ := s.store.ListFileErrors(20)
	return LibraryStats{Tracks: len(s.AllTracks()), Roots: len(roots), Errors: len(errors)}
}

func (s *Service) Upsert(track Track) error {
	row, err := toRow(track)
	if err != nil {
		return err
	}
	if err := s.store.UpsertTrack(row, trigrams(track.SearchText)); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cache[track.ID]; !ok {
		s.order = append(s.order, track.ID)
	}
	s.cache[track.ID] = track
	s.sortLocked()
	return nil
}

func (s *Service) UpdateClusterID(id string, clusterID int) error {
	if err := s.store.UpdateTrackCluster(id, clusterID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	track, ok := s.cache[id]
	if !ok {
		return nil
	}
	track.ClusterID = clusterID
	s.cache[id] = track
	return nil
}

func (s *Service) ImportFolder(folder string) ([]Track, error) {
	libLog.I("import folder start path=%s", folder)
	tracks := []Track{}
	var firstImportErr error
	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			libLog.I("import folder walk error path=%s err=%v", path, walkErr)
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !isAudioExt(filepath.Ext(path)) {
			return nil
		}
		track, importErr := s.ImportFile(path)
		if importErr != nil {
			libLog.I("import file failed path=%s err=%v", path, importErr)
			if firstImportErr == nil {
				firstImportErr = importErr
			}
			return nil
		}
		tracks = append(tracks, track)
		return nil
	})
	if err != nil {
		return tracks, err
	}
	if len(tracks) == 0 {
		if firstImportErr != nil {
			return tracks, firstImportErr
		}
		return tracks, fmt.Errorf("no supported audio files found in %s", folder)
	}
	libLog.I("import folder done path=%s tracks=%d", folder, len(tracks))
	return tracks, nil
}

func (s *Service) ImportFile(path string) (Track, error) { return s.importPath(path, false) }

func (s *Service) ImportVirtualFile(path string, duration time.Duration) (Track, error) {
	return s.importVirtual(path, duration)
}

func (s *Service) RebuildAll(ctx context.Context, progress func(RebuildProgress)) error {
	return s.rebuildTracks(ctx, s.AllTracks(), progress, "rebuild")
}

func (s *Service) RebuildStale(ctx context.Context, progress func(RebuildProgress)) error {
	tracks := s.AllTracks()
	stale := make([]Track, 0, len(tracks))
	for _, track := range tracks {
		if needsAnalysisRefresh(track) {
			stale = append(stale, track)
		}
	}
	return s.rebuildTracks(ctx, stale, progress, "rebuild-stale")
}

func (s *Service) rebuildTracks(ctx context.Context, tracks []Track, progress func(RebuildProgress), mode string) error {
	if len(tracks) == 0 {
		libLog.I("%s skipped empty library", mode)
		return nil
	}
	libLog.I("%s start total=%d", mode, len(tracks))
	for i, existing := range tracks {
		if ctx != nil {
			select {
			case <-ctx.Done():
				libLog.I("%s cancelled index=%d track=%s err=%v", mode, i, existing.ID, ctx.Err())
				return ctx.Err()
			default:
			}
		}
		if progress != nil {
			progress(RebuildProgress{Index: i + 1, Total: len(tracks), TrackID: existing.ID, Path: existing.Path, Stage: "start", State: "running", Message: mode + " track start"})
		}
		meta, metaSource := readMetadata(existing.Path)
		if progress != nil {
			progress(RebuildProgress{Index: i + 1, Total: len(tracks), TrackID: existing.ID, Path: existing.Path, Stage: "metadata", State: "running", Message: "metadata read"})
		}
		track, err := s.analyzeTrack(ctx, analysisJob{TrackID: existing.ID, Path: existing.Path, Meta: meta, Source: metaSource})
		if progress != nil && err != nil {
			progress(RebuildProgress{Index: i + 1, Total: len(tracks), TrackID: existing.ID, Path: existing.Path, Stage: "analyze", State: "error", Message: err.Error()})
		}
		if progress != nil {
			progress(RebuildProgress{Index: i + 1, Total: len(tracks), TrackID: existing.ID, Path: existing.Path, Stage: "upsert", State: "running", Message: "writing track data"})
		}
		dbStart := time.Now()
		if upsertErr := s.Upsert(track); upsertErr != nil {
			libLog.I("%s upsert failed index=%d id=%s path=%s err=%v", mode, i+1, existing.ID, existing.Path, upsertErr)
			if progress != nil {
				progress(RebuildProgress{Index: i + 1, Total: len(tracks), TrackID: existing.ID, Path: existing.Path, Stage: "upsert", State: "error", Message: upsertErr.Error()})
			}
			continue
		}
		libLog.I("%s db upsert done index=%d id=%s ms=%d emb=%d", mode, i+1, existing.ID, time.Since(dbStart).Milliseconds(), len(track.Embedding))
		if progress != nil {
			state := "ok"
			message := track.GenrePrimary
			if err != nil {
				state = "warning"
				message = track.AnalysisError
			}
			progress(RebuildProgress{Index: i + 1, Total: len(tracks), TrackID: existing.ID, Path: existing.Path, Stage: "done", State: state, Message: message})
		}
	}
	libLog.I("%s done total=%d", mode, len(tracks))
	return nil
}

func (s *Service) importVirtual(path string, duration time.Duration) (Track, error) {
	track := makeFallbackTrack(path, duration)
	track.AnalyzedLevel = 2
	if err := s.Upsert(track); err != nil {
		return Track{}, err
	}
	libLog.I("import virtual upserted path=%s id=%s analyzed=%d", path, track.ID, track.AnalyzedLevel)
	return track, nil
}

func (s *Service) importPath(path string, virtual bool) (Track, error) {
	if virtual {
		libLog.I("import virtual path=%s", path)
		return s.importVirtual(path, 4*time.Minute)
	}
	libLog.I("import file start path=%s", path)
	meta, metaSource := readMetadata(path)
	durMs := quickDurationMs(path)
	track := buildTrack(path, meta, metaSource, pendingFeatures(), durMs)
	track.AnalyzedLevel = 1
	track.AnalysisVersion = 0
	track.AnalysisError = ""
	if err := s.Upsert(track); err != nil {
		return Track{}, err
	}
	libLog.I("import file upserted path=%s id=%s analyzed=%d", path, track.ID, track.AnalyzedLevel)
	s.beginIndexing(1, "importing")
	s.enqueueAnalysis(track.ID, path, meta, metaSource)
	return track, nil
}

func (s *Service) enqueueAnalysis(trackID, path string, meta map[string]string, source string) {
	s.enqueueAnalysisJob(analysisJob{TrackID: trackID, Path: path, Meta: meta, Source: source})
}

func (s *Service) enqueueAnalysisJob(job analysisJob) {
	s.mu.Lock()
	if s.closed || s.inFlight[job.TrackID] {
		s.mu.Unlock()
		return
	}
	s.inFlight[job.TrackID] = true
	s.mu.Unlock()

	select {
	case s.analysisQueue <- job:
	case <-s.ctx.Done():
		s.finishAnalysis(job.TrackID)
	}
}

func (s *Service) enqueueAnalysisJobs(jobs []analysisJob) {
	for _, job := range jobs {
		s.enqueueAnalysisJob(job)
	}
}

func safeAnalyzeTrack(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during analysis: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}

func (s *Service) workerLoop() {
	defer s.workers.Done()
	for {
		var job analysisJob
		select {
		case <-s.ctx.Done():
			return
		case job = <-s.analysisQueue:
		}
		s.updateIndexingCurrent(job.Path, "analyzing")
		_ = s.store.MarkTrackAnalysisStatus(job.TrackID, string(AnalysisRunning), "", 1)
		err := safeAnalyzeTrack(func() error {
			defer s.finishAnalysis(job.TrackID)
			defer s.finishIndexingOne(job.Path)
			track, err := s.analyzeTrack(s.ctx, job)
			if err != nil {
				return err
			}
			dbStart := time.Now()
			if err := s.Upsert(track); err != nil {
				return err
			}
			if track.AnalysisStatus == string(AnalysisDone) {
				_ = s.store.MarkTrackReady(track.ID)
			}
			libLog.I("db upsert done id=%s status=%s version=%d ms=%d emb=%d", track.ID, track.AnalysisStatus, track.AnalysisVersion, time.Since(dbStart).Milliseconds(), len(track.Embedding))
			s.mu.RLock()
			hook := s.hook
			s.mu.RUnlock()
			if hook != nil {
				hook(track)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) && s.ctx.Err() != nil {
				libLog.I("analysis canceled id=%s path=%q", job.TrackID, job.Path)
				continue
			}
			libLog.E("analyze failed id=%s path=%q err=%v", job.TrackID, job.Path, err)
			_ = s.store.MarkTrackAnalysisStatus(job.TrackID, string(AnalysisError), err.Error(), 1)
			_ = s.store.AddFileError(db.FileErrorRow{ID: fmt.Sprintf("err-%d", time.Now().UnixNano()), TrackID: job.TrackID, Path: job.Path, LibraryType: "music", Stage: "analyze", Kind: "decode_or_analysis_failed", Message: err.Error(), CreatedAt: time.Now().Unix()})
		}
	}
}

func (s *Service) finishAnalysis(trackID string) {
	var retry analysisJob
	hasRetry := false
	s.mu.Lock()
	delete(s.inFlight, trackID)
	if job, ok := s.requeue[trackID]; ok {
		delete(s.requeue, trackID)
		if !s.closed {
			s.inFlight[trackID] = true
			retry = job
			hasRetry = true
		}
	}
	s.mu.Unlock()
	if !hasRetry {
		return
	}
	select {
	case s.analysisQueue <- retry:
		libLog.I("analysis requeued after engine reload id=%s", retry.TrackID)
	case <-s.ctx.Done():
		s.mu.Lock()
		delete(s.inFlight, retry.TrackID)
		s.mu.Unlock()
	default:
		s.mu.Lock()
		delete(s.inFlight, retry.TrackID)
		s.mu.Unlock()
		libLog.W("analysis requeue skipped id=%s reason=queue-full", retry.TrackID)
	}
}

func (s *Service) analyzeTrack(parent context.Context, job analysisJob) (Track, error) {
	if parent == nil {
		parent = s.ctx
	}
	start := time.Now()
	libLog.D("analyze start id=%s source=%s path=%s", job.TrackID, job.Source, job.Path)

	if !job.Force {
		if existing, found, err := s.store.TrackByPath(job.Path); err == nil && found {
			if existing.AnalysisStatus == string(AnalysisDone) &&
				existing.AnalysisVersion >= currentAnalysisVersion &&
				len(existing.Embedding) == modelcontract.DiscogsEmbeddingSize {
				libLog.D("skip current analysis id=%s version=%d emb=%d", existing.ID, existing.AnalysisVersion, len(existing.Embedding))
				return TrackFromRow(existing), nil
			}
		}
	}

	s.mu.RLock()
	essentia := s.essentia
	onnxEngine := s.onnx
	tempoEngine := s.tempo
	s.mu.RUnlock()
	ctx, cancelAnalyze := context.WithTimeout(parent, 2*time.Minute)
	defer cancelAnalyze()
	features, durMs, featErr := analysis.ExtractWithContext(ctx, job.Path)
	if featErr != nil {
		libLog.I("analysis.Extract failed path=%s err=%v", job.Path, featErr)
		fallback := buildTrack(job.Path, job.Meta, job.Source, pendingFeatures(), quickDurationMs(job.Path))
		fallback.AnalyzedLevel = 1
		fallback.AnalysisVersion = currentAnalysisVersion
		fallback.AnalyzedAt = time.Now().Unix()
		fallback.AnalysisError = featErr.Error()
		fallback.LastError = featErr.Error()
		fallback.AnalysisStatus = string(AnalysisError)
		fallback.ImportStatus = string(ImportReady)
		fallback.EssentiaModelVersion = currentEssentiaModelVersion
		return fallback, featErr
	}
	track := buildTrack(job.Path, job.Meta, job.Source, features, durMs)
	if existing, found, err := s.store.TrackByPath(job.Path); err == nil && found {
		track.ID = existing.ID
	}
	if tempoEngine != nil && tempoEngine.Ready() {
		tempoCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		if tempo, tempoErr := tempoEngine.AnalyzePath(tempoCtx, job.Path); tempoErr != nil {
			track.TempoSource = "error"
			track.TempoError = tempoErr.Error()
			track.TempoAnalyzedAt = time.Now().Unix()
			libLog.I("tempo analyze failed path=%s err=%v", job.Path, tempoErr)
		} else if tempo.Reliable {
			track.Tempo = tempo.BPM
			track.BPMPerceived = tempo.BPMPerceived
			track.TempoConfidence = tempo.Confidence
			track.TempoStability = tempo.Stability
			track.BPMHalf = tempo.BPMHalf
			track.BPMDouble = tempo.BPMDouble
			track.TempoSource = tempo.Source
			track.TempoModelVersion = tempo.ModelVersion
			track.TempoAnalyzedAt = tempo.AnalyzedAt
			track.TempoError = ""
		} else {
			track.Tempo = 0
			track.BPMPerceived = 0
			track.TempoConfidence = 0
			track.TempoStability = 0
			track.TempoSource = "unreliable"
			track.TempoError = ""
			libLog.W("tempo ignored id=%s bpm=%.2f rawStd=%.4f", job.TrackID, tempo.BPM, tempo.RawBPMStd)
		}
		cancel()
	}
	track.AnalyzedAt = time.Now().Unix()
	track.LastError = ""
	track.ImportStatus = string(ImportReady)
	track.FileMissing = false
	track.Embedding = nil // compact DSP features are not a semantic embedding
	semanticReady := false
	var semanticErr error
	genreScore := 0.0
	genreMargin := 0.0
	libLog.I("pre-essentia compactFeatures=%d semanticEmbedding=%d essentia=%v ready=%v", 16, len(track.Embedding), essentia != nil, essentia != nil && essentia.Ready())
	libLog.D("fast features path=%s durMs=%d tempo=%.2f energy=%.3f dance=%.3f valence=%.3f acou=%.3f instr=%.3f", job.Path, durMs, features.Tempo, features.Energy, features.Danceability, features.Valence, features.Acousticness, features.Instrumentalness)
	if essentia != nil && essentia.Ready() {
		melStart := time.Now()
		mel, patches, err := analysis.ExtractMelSpectrogramWithContext(ctx, job.Path)
		if err != nil {
			semanticErr = fmt.Errorf("mel extraction: %w", err)
			libLog.I("mel extract failed path=%s err=%v ms=%d", job.Path, err, time.Since(melStart).Milliseconds())
		} else if patches <= 0 {
			semanticErr = errors.New("mel extraction returned no patches")
			libLog.D("mel extract empty path=%s patches=%d len=%d ms=%d", job.Path, patches, len(mel), time.Since(melStart).Milliseconds())
		} else {
			libLog.D("mel extract done path=%s ms=%d len=%d patches=%d", job.Path, time.Since(melStart).Milliseconds(), len(mel), patches)
			essentiaStart := time.Now()
			if ml, err := essentia.Analyze(ctx, mel, patches); err != nil {
				semanticErr = fmt.Errorf("Essentia inference: %w", err)
				libLog.I("essentia analyze failed path=%s err=%v ms=%d", job.Path, err, time.Since(essentiaStart).Milliseconds())
			} else if len(ml.Embedding) != modelcontract.DiscogsEmbeddingSize {
				semanticErr = fmt.Errorf("Essentia embedding size=%d want=%d", len(ml.Embedding), modelcontract.DiscogsEmbeddingSize)
				libLog.W("essentia invalid embedding path=%s got=%d want=%d ms=%d", job.Path, len(ml.Embedding), modelcontract.DiscogsEmbeddingSize, time.Since(essentiaStart).Milliseconds())
			} else {
				semanticReady = true
				libLog.D("essentia analyze done path=%s ms=%d", job.Path, time.Since(essentiaStart).Milliseconds())
				track.Danceability = blendMetric(track.Danceability, ml.Danceability, 0.7)
				track.Valence = blendMetric(track.Valence, ml.Valence, 0.7)
				track.Acousticness = blendMetric(track.Acousticness, ml.Acousticness, 0.7)
				track.Electronicness = ml.Electronic
				track.Instrumentalness = blendMetric(track.Instrumentalness, ml.Instrumentalness, 0.7)
				track.Vocalness = 1.0 - track.Instrumentalness
				track.Happy = ml.MoodHappy
				track.Sad = ml.MoodSad
				track.Relaxed = ml.MoodRelaxed
				track.Party = ml.MoodParty
				track.Aggressive = ml.MoodAggressive
				track.TimbreBrightness = ml.TimbreBrightness
				track.Tonality = ml.Tonality
				track.Approachability = ml.Approachability
				track.Engagement = ml.Engagement
				track.Melodicness = ml.Melodicness
				track.Softness = ml.Softness
				track.Heaviness = ml.Heaviness
				track.Dreaminess = ml.Dreaminess
				track.Emotionality = ml.Emotionality
				track.Energy = blendMetric(track.Energy, ml.Energy, 0.65)
				genreScore = ml.GenreScore
				genreMargin = ml.GenreMargin
				track.GenreDetail = ml.GenreDetail
				track.GenreTags = append([]onnx.GenreTag{}, ml.GenreTags...)
				track.GenrePrimary = chooseGenre(track.Genre, ml.GenrePrimary)
				if strings.TrimSpace(ml.GenreLabel) != "" {
					track.GenreLabel = ml.GenreLabel
				} else {
					track.GenreLabel = genreLabelFromTags(track.GenreTags, track.GenrePrimary, track.GenreDetail)
				}
				track.Embedding = l2Normalize(append([]float32{}, ml.Embedding...))
				if strings.TrimSpace(ml.GenrePrimary) == "" && ml.GenreScore >= 0.06 {
					libLog.I("essentia empty genre with score path=%s score=%.4f label=%q detail=%q", job.Path, ml.GenreScore, ml.GenreLabel, ml.GenreDetail)
				}
				libLog.I("essentia features path=%s dance=%.2f valence=%.2f acoustic=%.2f electronic=%.2f instrumental=%.2f happy=%.2f sad=%.2f relaxed=%.2f party=%.2f aggressive=%.2f timbre=%.2f tonal=%.2f approachability=%.2f engagement=%.2f melodic=%.2f soft=%.2f heavy=%.2f dream=%.2f emotional=%.2f genre=%q label=%q detail=%q score=%.3f margin=%.3f", job.Path, ml.Danceability, ml.Valence, ml.Acousticness, ml.Electronic, ml.Instrumentalness, ml.MoodHappy, ml.MoodSad, ml.MoodRelaxed, ml.MoodParty, ml.MoodAggressive, ml.TimbreBrightness, ml.Tonality, ml.Approachability, ml.Engagement, ml.Melodicness, ml.Softness, ml.Heaviness, ml.Dreaminess, ml.Emotionality, track.GenrePrimary, track.GenreLabel, track.GenreDetail, ml.GenreScore, ml.GenreMargin)
			}
		}
	}
	applySemanticAnalysisState(&track, semanticReady, semanticErr)
	if strings.TrimSpace(track.GenreLabel) == "" &&
		!strings.EqualFold(track.GenrePrimary, "unknown") {
		track.GenreLabel = track.GenrePrimary
	}
	text := strings.TrimSpace(strings.Join([]string{track.Artist, track.Title, track.Genre, track.Album}, " "))
	if onnxEngine != nil && onnxEngine.Ready() && text != "" {
		textStart := time.Now()
		if vec, err := onnxEngine.Encode(ctx, text); err == nil && len(vec) > 0 {
			track.TextEmbedding = append([]float32{}, vec...)
			libLog.D("onnx text embedding stored path=%s dim=%d audioDim=%d ms=%d", job.Path, len(vec), len(track.Embedding), time.Since(textStart).Milliseconds())
		} else if err != nil {
			libLog.I("onnx encode failed path=%s err=%v", job.Path, err)
		}
	}
	libLog.I("analyze done id=%s title=%q emb=%d genre=%q label=%q detail=%q dance=%.2f energy=%.2f valence=%.2f acoustic=%.2f electronic=%.2f instrumental=%.2f vocal=%.2f happy=%.2f sad=%.2f relaxed=%.2f party=%.2f aggressive=%.2f timbre=%.2f tonal=%.2f approachability=%.2f engagement=%.2f melodic=%.2f soft=%.2f heavy=%.2f dream=%.2f emotional=%.2f genreScore=%.3f genreMargin=%.3f ms=%d", track.ID, track.Title, len(track.Embedding), track.GenrePrimary, track.GenreLabel, track.GenreDetail, track.Danceability, track.Energy, track.Valence, track.Acousticness, track.Electronicness, track.Instrumentalness, track.Vocalness, track.Happy, track.Sad, track.Relaxed, track.Party, track.Aggressive, track.TimbreBrightness, track.Tonality, track.Approachability, track.Engagement, track.Melodicness, track.Softness, track.Heaviness, track.Dreaminess, track.Emotionality, genreScore, genreMargin, time.Since(start).Milliseconds())
	if len(track.Embedding) != 0 && len(track.Embedding) != 1280 {
		return Track{}, fmt.Errorf("refusing to persist invalid embedding id=%s size=%d want=1280", track.ID, len(track.Embedding))
	}
	return track, nil
}

func applySemanticAnalysisState(track *Track, semanticReady bool, semanticErr error) {
	if track == nil {
		return
	}
	if semanticReady && len(track.Embedding) == modelcontract.DiscogsEmbeddingSize {
		track.AnalyzedLevel = 2
		track.AnalysisVersion = currentAnalysisVersion
		track.AnalysisStatus = string(AnalysisDone)
		track.AnalysisError = ""
		track.EssentiaModelVersion = currentEssentiaModelVersion
		return
	}

	track.AnalyzedLevel = 1
	track.AnalysisVersion = 0
	track.EssentiaModelVersion = ""
	track.Embedding = nil
	if semanticErr != nil {
		track.AnalysisStatus = string(AnalysisError)
		track.AnalysisError = semanticErr.Error()
		return
	}
	track.AnalysisStatus = string(AnalysisQueued)
	track.AnalysisError = "semantic model unavailable; waiting for runtime/model"
}

func readMetadata(path string) (map[string]string, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "filename"
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, "filename"
	}
	return map[string]string{"title": strings.TrimSpace(m.Title()), "artist": strings.TrimSpace(m.Artist()), "album": strings.TrimSpace(m.Album()), "genre": strings.TrimSpace(m.Genre())}, "tag"
}

func buildTrack(path string, meta map[string]string, source string, features analysis.Features, durMs int) Track {
	base := filepath.Base(path)
	fallbackTitle, fallbackArtist := parseName(strings.TrimSuffix(base, filepath.Ext(base)))
	h := sha1.Sum([]byte(path))
	id := hex.EncodeToString(h[:8])
	title, artist, album, genre := fallbackTitle, fallbackArtist, "Local Library", "Unknown"
	if meta != nil {
		if strings.TrimSpace(meta["title"]) != "" {
			title = meta["title"]
		}
		if strings.TrimSpace(meta["artist"]) != "" {
			artist = meta["artist"]
		}
		if strings.TrimSpace(meta["album"]) != "" {
			album = meta["album"]
		}
		if cleanedGenre := sanitizeMetadataGenre(meta["genre"]); cleanedGenre != "" {
			genre = cleanedGenre
		}
	}
	if durMs <= 0 {
		durMs = int((4 * time.Minute) / time.Millisecond)
	}
	searchText := strings.ToLower(strings.Join([]string{title, artist, album, genre, base, filepath.Dir(path)}, " "))
	return Track{ID: id, Path: path, Title: title, Artist: artist, Album: album, Genre: genre, GenrePrimary: "", DurationMs: durMs, DurationLabel: formatDuration(time.Duration(durMs) * time.Millisecond), Folder: filepath.Dir(path), FileName: base, Tempo: features.Tempo, BPMPerceived: features.BPMPerceived, TempoConfidence: features.TempoConfidence, TempoStability: features.TempoStability, BPMHalf: features.BPMHalf, BPMDouble: features.BPMDouble, TempoSource: features.TempoSource, TempoModelVersion: features.TempoModel, TempoAnalyzedAt: features.TempoAnalyzedAt, TempoError: features.TempoError, Energy: features.Energy, Danceability: features.Danceability, Valence: features.Valence, Acousticness: features.Acousticness, Instrumentalness: features.Instrumentalness, Loudness: features.Loudness, SpectralCentroid: features.SpectralCentroid, ZeroCrossingRate: features.ZeroCrossingRate, RMS: features.RMS, SpectralFlatness: features.SpectralFlatness, SpectralRolloff85: features.SpectralRolloff85, SpectralFlux: features.SpectralFlux, OnsetRate: features.OnsetRate, DynamicRange: features.DynamicRange, LowBandRatio: features.LowBandRatio, MidBandRatio: features.MidBandRatio, HighBandRatio: features.HighBandRatio, MetadataSource: source, AnalysisVersion: 0, AnalyzedAt: 0, AnalysisError: "", EssentiaModelVersion: "", NormalizedPath: path, ImportStatus: string(ImportReady), AnalysisStatus: string(AnalysisNone), Embedding: features.Embedding, TextEmbedding: nil, SearchText: searchText}
}

func makeFallbackTrack(path string, duration time.Duration) Track {
	return buildTrack(path, nil, "mock", mockFeatures(path), int(duration/time.Millisecond))
}

func TrackFromRow(r db.TrackRow) Track {
	staleAnalysis := r.AnalysisVersion < currentAnalysisVersion ||
		strings.TrimSpace(r.EssentiaModelVersion) != currentEssentiaModelVersion
	embedding := r.Embedding
	if staleAnalysis {
		embedding = nil
	} else if len(embedding) != 0 &&
		len(embedding) != modelcontract.DiscogsEmbeddingSize {
		libLog.W(
			"ignore invalid stored embedding id=%s size=%d",
			r.ID,
			len(embedding),
		)
		embedding = nil
	}

	tags := make([]onnx.GenreTag, 0)
	if strings.TrimSpace(r.GenreTagsJSON) != "" {
		_ = json.Unmarshal([]byte(r.GenreTagsJSON), &tags)
	}
	track := Track{ID: r.ID, Path: r.Path, Title: r.Title, Artist: r.Artist, Album: r.Album, Genre: r.Genre, GenrePrimary: r.GenrePrimary, GenreDetail: r.GenreDetail, GenreTags: tags, DurationMs: r.DurationMs, DurationLabel: r.DurationLabel, Folder: r.Folder, FileName: r.FileName, Tempo: r.Tempo, BPMPerceived: r.BPMPerceived, TempoConfidence: r.TempoConfidence, TempoStability: r.TempoStability, BPMHalf: r.BPMHalf, BPMDouble: r.BPMDouble, TempoSource: r.TempoSource, TempoModelVersion: r.TempoModelVersion, TempoAnalyzedAt: r.TempoAnalyzedAt, TempoError: r.TempoError, Energy: r.Energy, Danceability: r.Danceability, Valence: r.Valence, Acousticness: r.Acousticness, Electronicness: r.Electronicness, Instrumentalness: r.Instrumentalness, Vocalness: r.Vocalness, Happy: r.Happy, Sad: r.Sad, Relaxed: r.Relaxed, Party: r.Party, Aggressive: r.Aggressive, TimbreBrightness: r.TimbreBrightness, Tonality: r.Tonality, Approachability: r.Approachability, Engagement: r.Engagement, Melodicness: r.Melodicness, Softness: r.Softness, Heaviness: r.Heaviness, Dreaminess: r.Dreaminess, Emotionality: r.Emotionality, Loudness: r.Loudness, SpectralCentroid: r.SpectralCentroid, ZeroCrossingRate: r.ZeroCrossingRate, RMS: r.RMS, SpectralFlatness: r.SpectralFlatness, SpectralRolloff85: r.SpectralRolloff85, SpectralFlux: r.SpectralFlux, OnsetRate: r.OnsetRate, DynamicRange: r.DynamicRange, LowBandRatio: r.LowBandRatio, MidBandRatio: r.MidBandRatio, HighBandRatio: r.HighBandRatio, ClusterID: r.ClusterID, PlayCount: r.PlayCount, SkipCount: r.SkipCount, CompleteCount: r.CompleteCount, MetadataSource: r.MetadataSource, AnalyzedLevel: r.AnalyzedLevel, AnalysisVersion: r.AnalysisVersion, AnalyzedAt: r.AnalyzedAt, AnalysisError: r.AnalysisError, EssentiaModelVersion: r.EssentiaModelVersion, NormalizedPath: r.NormalizedPath, LibraryRootID: r.LibraryRootID, ImportStatus: r.ImportStatus, AnalysisStatus: r.AnalysisStatus, FileMissing: r.FileMissing, FileSize: r.FileSize, FileMTime: r.FileMTime, FileInode: r.FileInode, QuickHash: r.QuickHash, LastSeenAt: r.LastSeenAt, LastError: r.LastError, Embedding: embedding, TextEmbedding: r.TextEmbedding, PlaybackErrorCount: r.PlaybackErrorCount, LastPlaybackError: r.LastPlaybackError, LastPlaybackErrorAt: r.LastPlaybackErrorAt, ImportedAt: r.AddedAt, SourceType: r.SourceType, SourceURL: r.SourceURL, SourceSite: r.SourceSite, ExternalID: r.ExternalID, DownloadStatus: r.DownloadStatus, DownloadProgress: r.DownloadProgress, DownloadError: r.DownloadError, DownloadAttempts: r.DownloadAttempts, DownloadedAt: r.DownloadedAt, SearchText: strings.ToLower(strings.Join([]string{r.Title, r.Artist, r.Album, r.Genre, r.FileName, r.Folder}, " "))}
	if staleAnalysis {
		track.GenrePrimary = ""
		track.GenreDetail = ""
		track.GenreTags = nil
		track.GenreLabel = sanitizeMetadataGenre(track.Genre)
		if track.GenreLabel == "" {
			track.GenreLabel = "Unknown"
		}
		track.Happy = 0
		track.Sad = 0
		track.Relaxed = 0
		track.Party = 0
		track.Aggressive = 0
		track.Electronicness = 0
		track.TimbreBrightness = 0
		track.Tonality = 0
		track.Approachability = 0.5
		track.Engagement = 0.5
		track.Melodicness = 0
		track.Softness = 0
		track.Heaviness = 0
		track.Dreaminess = 0
		track.Emotionality = 0
		track.TempoConfidence = 0
		track.TempoStability = 0
		return track
	}
	if len(track.GenreTags) > 0 {
		track.GenrePrimary = track.GenreTags[0].Label
		track.GenreDetail = track.GenreTags[0].Detail
	}
	track.GenreLabel = strings.TrimSpace(r.GenreLabel)
	if track.GenreLabel == "" || strings.EqualFold(track.GenreLabel, "unknown") || strings.EqualFold(track.GenreLabel, "score") {
		track.GenreLabel = genreLabelFromTags(track.GenreTags, track.GenrePrimary, track.GenreDetail)
	}
	if track.GenreLabel == "" && track.GenrePrimary != "" {
		track.GenreLabel = track.GenrePrimary
	}
	return track
}

func fromRow(r db.TrackRow) Track { return TrackFromRow(r) }

func toRow(t Track) (db.TrackRow, error) {
	genreTagsJSON := ""
	if len(t.GenreTags) > 0 {
		data, err := json.Marshal(t.GenreTags)
		if err != nil {
			return db.TrackRow{}, err
		}
		genreTagsJSON = string(data)
	}
	return db.TrackRow{ID: t.ID, Path: t.Path, Title: t.Title, Artist: t.Artist, Album: t.Album, Genre: t.Genre, GenrePrimary: t.GenrePrimary, GenreDetail: t.GenreDetail, GenreTagsJSON: genreTagsJSON, GenreLabel: t.GenreLabel, DurationMs: t.DurationMs, DurationLabel: t.DurationLabel, Folder: t.Folder, FileName: t.FileName, Tempo: t.Tempo, BPMPerceived: t.BPMPerceived, TempoConfidence: t.TempoConfidence, TempoStability: t.TempoStability, BPMHalf: t.BPMHalf, BPMDouble: t.BPMDouble, TempoSource: t.TempoSource, TempoModelVersion: t.TempoModelVersion, TempoAnalyzedAt: t.TempoAnalyzedAt, TempoError: t.TempoError, Energy: t.Energy, Danceability: t.Danceability, Valence: t.Valence, Acousticness: t.Acousticness, Electronicness: t.Electronicness, Instrumentalness: t.Instrumentalness, Vocalness: t.Vocalness, Happy: t.Happy, Sad: t.Sad, Relaxed: t.Relaxed, Party: t.Party, Aggressive: t.Aggressive, TimbreBrightness: t.TimbreBrightness, Tonality: t.Tonality, Approachability: t.Approachability, Engagement: t.Engagement, Melodicness: t.Melodicness, Softness: t.Softness, Heaviness: t.Heaviness, Dreaminess: t.Dreaminess, Emotionality: t.Emotionality, Loudness: t.Loudness, SpectralCentroid: t.SpectralCentroid, ZeroCrossingRate: t.ZeroCrossingRate, RMS: t.RMS, SpectralFlatness: t.SpectralFlatness, SpectralRolloff85: t.SpectralRolloff85, SpectralFlux: t.SpectralFlux, OnsetRate: t.OnsetRate, DynamicRange: t.DynamicRange, LowBandRatio: t.LowBandRatio, MidBandRatio: t.MidBandRatio, HighBandRatio: t.HighBandRatio, ClusterID: t.ClusterID, PlayCount: t.PlayCount, SkipCount: t.SkipCount, CompleteCount: t.CompleteCount, MetadataSource: t.MetadataSource, AnalyzedLevel: t.AnalyzedLevel, AnalysisVersion: t.AnalysisVersion, AnalyzedAt: t.AnalyzedAt, AnalysisError: t.AnalysisError, EssentiaModelVersion: t.EssentiaModelVersion, NormalizedPath: t.NormalizedPath, LibraryRootID: t.LibraryRootID, ImportStatus: t.ImportStatus, AnalysisStatus: t.AnalysisStatus, FileMissing: t.FileMissing, FileSize: t.FileSize, FileMTime: t.FileMTime, FileInode: t.FileInode, QuickHash: t.QuickHash, LastSeenAt: t.LastSeenAt, LastError: t.LastError, Embedding: t.Embedding, TextEmbedding: t.TextEmbedding}, nil
}

func buildTags(t Track) string {
	return strings.ToLower(strings.Join([]string{t.Title, t.Artist, t.Album, t.Genre, t.FileName}, " "))
}

func chooseGenre(metaGenre, mlGenre string) string {
	// Essentia owns model-output quality and minimum-evidence gates. Parent
	// ambiguity is intentionally preserved for the multi-label Discogs head;
	// recommend.genreTrust() uses the numeric tag margin to reduce its weight.
	// Do not infer or rewrite ML genres from title/artist/file-name text here.
	mlGenre = strings.TrimSpace(strings.ReplaceAll(mlGenre, "---", " / "))
	if mlGenre != "" {
		return mlGenre
	}
	if metaGenre = sanitizeMetadataGenre(metaGenre); metaGenre != "" {
		return metaGenre
	}
	return "Unknown"
}

var metadataGenreDomain = regexp.MustCompile(
	`(?i)(?:https?://|www\.|[a-z0-9-]+\.(?:ru|kz|com|net|org|ua|me)(?:\b|/))`,
)

func sanitizeMetadataGenre(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	switch lower {
	case "unknown", "unk", "other", "genre", "n/a", "none", "-":
		return ""
	}
	if metadataGenreDomain.MatchString(value) {
		return ""
	}
	return value
}

func genreLabelFromTags(tags []onnx.GenreTag, primary, detail string) string {
	labels := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		label := strings.TrimSpace(tag.Label)
		if label == "" {
			label = strings.TrimSpace(tag.Detail)
		}
		if label == "" || strings.EqualFold(label, "unknown") {
			continue
		}
		if seen[strings.ToLower(label)] {
			continue
		}
		seen[strings.ToLower(label)] = true
		labels = append(labels, label)
	}
	if len(labels) > 0 {
		if len(labels) > 3 {
			labels = labels[:3]
		}
		return strings.Join(labels, ", ")
	}
	if detail = strings.TrimSpace(detail); detail != "" {
		return strings.ReplaceAll(detail, "---", " / ")
	}
	return strings.TrimSpace(primary)
}

func trigrams(s string) []string {
	s = normalizeForSearch(s)
	if len(s) < 3 {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	m := map[string]struct{}{}
	out := []string{}
	for i := 0; i <= len(s)-3; i++ {
		g := s[i : i+3]
		if _, ok := m[g]; !ok {
			m[g] = struct{}{}
			out = append(out, g)
		}
	}
	return out
}

func normalizeForSearch(s string) string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ", "/", " ", "\\", " ", "ё", "е")
	s = replacer.Replace(s)
	for _, noise := range []string{"320kbps", "official", "track", "audio"} {
		s = strings.ReplaceAll(s, noise, " ")
	}
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func (s *Service) sortLocked() {
	sort.SliceStable(s.order, func(i, j int) bool {
		return strings.ToLower(s.cache[s.order[i]].Title) < strings.ToLower(s.cache[s.order[j]].Title)
	})
}

var dashSplit = regexp.MustCompile(`\s[-—/]\s|\s[-—]\s|\s/\s`)

func parseName(name string) (string, string) {
	parts := dashSplit.Split(name, -1)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(name), "Unknown Artist"
}

func formatDuration(d time.Duration) string {
	secs := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

func pendingFeatures() analysis.Features {
	// A real file that has not been analyzed has unknown audio semantics. Never
	// synthesize recommendation features from its path, title or file name.
	return analysis.Features{}
}

func mockFeatures(seed string) analysis.Features {
	return analysis.Features{Tempo: mockMetric(seed, 80, 138), Energy: mockUnit(seed, 11), Danceability: mockUnit(seed, 21), Valence: mockUnit(seed, 31), Acousticness: mockUnit(seed, 41), Instrumentalness: mockUnit(seed, 51), Loudness: -12 + mockUnit(seed, 61)*8, Embedding: []float32{float32(mockUnit(seed, 11)), float32(mockUnit(seed, 21)), float32(mockUnit(seed, 31)), float32(mockUnit(seed, 41))}}
}

func quickDurationMs(path string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ms, err := analysis.DecodeAudioDuration(ctx, path)
	if err != nil || ms <= 0 {
		return analysis.EstimateAudioDuration(path, 0, 0)
	}
	return ms
}

func needsDurationRepair(track Track) bool {
	if track.DurationMs <= 0 {
		return true
	}
	if track.DurationMs == 30000 {
		return true
	}
	if track.DurationLabel == "0:30" {
		return true
	}
	return false
}

func l2Normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return vec
	}
	norm := float32(1.0 / sqrt(sum))
	for i := range vec {
		vec[i] *= norm
	}
	return vec
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 1
	}
	x := v
	for i := 0; i < 8; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}

func blendMetric(base, ml, mlWeight float64) float64 {
	if ml <= 0 {
		return base
	}
	return base*(1-mlWeight) + ml*mlWeight
}

func mockUnit(seed string, salt byte) float64 {
	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i]) + int(salt)
	}
	return float64(sum%100) / 100.0
}

func mockMetric(seed string, min, max float64) float64 { return min + mockUnit(seed, 7)*(max-min) }

func isAudioExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp3", ".flac", ".wav", ".ogg", ".oga", ".m4a", ".aac", ".opus", ".aiff", ".aif", ".wma":
		return true
	default:
		return false
	}
}

func ensureNotUsed(v any) { _, _ = sql.ErrNoRows, v }
