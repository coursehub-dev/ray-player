package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ray-player1/internal/analysis"
	"ray-player1/internal/appstate"
	"ray-player1/internal/audio"
	"ray-player1/internal/db"
	"ray-player1/internal/deps"
	"ray-player1/internal/emoflow"
	"ray-player1/internal/events"
	"ray-player1/internal/externalmedia"
	"ray-player1/internal/library"
	"ray-player1/internal/logx"
	"ray-player1/internal/onnx"
	"ray-player1/internal/podcast"
	"ray-player1/internal/rays"
	"ray-player1/internal/recommend"
	"ray-player1/internal/search"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var appLog = logx.New("App")

const (
	metaPlayerMuted            = "player.muted"
	metaPlayerLastNonZeroVol   = "player.last_nonzero_volume"
	metaNormalizePodcastVolume = "podcast.normalize_volume"
)

type App struct {
	ctx             context.Context
	store           *db.Store
	state           *appstate.Store
	library         *library.Service
	podcasts        *podcast.Service
	podcastHistory  *podcast.HistoryService
	search          *search.Service
	rec             *recommend.Service
	events          *events.Service
	rays            *rays.Service
	audio           *audio.Service
	rayBuildMu      sync.RWMutex
	lastRayBuildKey string
	lastRayBuildAt  time.Time
	lastRayQueue    []rays.QueueItem

	playRequestMu     sync.Mutex
	playRequestSeq    uint64
	playRequestCancel context.CancelFunc

	playbackEpoch atomic.Uint64

	rayBuildState appstate.RayBuildState

	onnx     *onnx.Engine
	essentia *onnx.EssentiaEngine
	tempo    *onnx.TempoEngine

	reindexMu        sync.Mutex
	reindexRunning   bool
	reclusterMu      sync.Mutex
	reclusterTimer   *time.Timer
	reclusterRunning bool
	reclusterPending bool
	milestones       playbackMilestones

	sessionMu          sync.Mutex
	lastSessionPersist time.Time

	podcastPlayback podcast.Playback

	lastPodcastProgressAt time.Time
	lastPodcastPositionMs int

	externalDownloads *externalmedia.Worker

	lifecycleMu  sync.Mutex
	runCtx       context.Context
	runCancel    context.CancelFunc
	backgroundWG sync.WaitGroup
	shuttingDown bool
	shutdownOnce sync.Once
}

type playbackMilestones struct {
	TrackID  string
	Marked30 bool
	Marked60 bool
	Marked50 bool
	Marked80 bool
}

type BootstrapPayload struct {
	Library           []library.Track          `json:"library"`
	Podcasts          []podcast.Item           `json:"podcasts"`
	PodcastRay        podcast.Ray              `json:"podcastRay"`
	PodcastPlayback   podcast.Playback         `json:"podcastPlayback"`
	PodcastHistory    []podcast.HistoryItem    `json:"podcastHistory"`
	PodcastRays       []podcast.RayHistoryItem `json:"podcastRays"`
	Current           appstate.PlayerState     `json:"current"`
	History           []events.HistoryItem     `json:"history"`
	Rays              []rays.RaySummary        `json:"rays"`
	Queue             []rays.QueueItem         `json:"queue"`
	MusicRay          rays.Ray                 `json:"musicRay"`
	LibraryStat       library.LibraryStats     `json:"libraryStat"`
	Roots             []library.LibraryRoot    `json:"roots,omitempty"`
	ImportErrors      []library.FileError      `json:"importErrors,omitempty"`
	EmoFlow           emoflow.UIState          `json:"emoFlow"`
	EmoFlowUISettings emoflow.UISettings       `json:"emoFlowUiSettings"`
	RayBuild          appstate.RayBuildState   `json:"rayBuild"`
}

type SettingsPayload struct {
	OnnxRuntimePath        string             `json:"onnxRuntimePath"`
	MiniLMModelDir         string             `json:"miniLMModelDir"`
	EssentiaModelDir       string             `json:"essentiaModelDir"`
	FFmpegPath             string             `json:"ffmpegPath"`
	FFprobePath            string             `json:"ffprobePath"`
	StoragePath            string             `json:"storagePath"`
	RepeatRay              bool               `json:"repeatRay"`
	ExtendRay              bool               `json:"extendRay"`
	EmoFlowUI              emoflow.UISettings `json:"emoFlowUi"`
	NormalizePodcastVolume bool               `json:"normalizePodcastVolume"`
}

type ImportResult = library.ImportSummary

type PlaybackErrorKind string

const (
	PlaybackFileMissing      PlaybackErrorKind = "file_missing"
	PlaybackPermissionDenied PlaybackErrorKind = "permission_denied"
	PlaybackDecodeError      PlaybackErrorKind = "decode_error"
	PlaybackUnsupportedCodec PlaybackErrorKind = "unsupported_codec"
	PlaybackDeviceError      PlaybackErrorKind = "audio_device_error"
	PlaybackTimeout          PlaybackErrorKind = "timeout"
	PlaybackUnknown          PlaybackErrorKind = "unknown"
)

type PlaybackFailure struct {
	TrackID string            `json:"trackId"`
	Path    string            `json:"path"`
	Title   string            `json:"title"`
	Kind    PlaybackErrorKind `json:"kind"`
	Error   string            `json:"error"`
	At      int64             `json:"at"`
}

type ModelCheckResult struct {
	Name        string   `json:"name"`
	ModelPath   string   `json:"modelPath"`
	MetaPath    string   `json:"metaPath"`
	Present     bool     `json:"present"`
	Loaded      bool     `json:"loaded"`
	Message     string   `json:"message"`
	InputName   string   `json:"inputName"`
	OutputName  string   `json:"outputName"`
	InputShape  []string `json:"inputShape"`
	OutputShape []string `json:"outputShape"`
}

type RuntimeTestResult struct {
	OK          bool   `json:"ok"`
	RuntimePath string `json:"runtimePath"`
	Message     string `json:"message"`
	LatencyMS   int64  `json:"latencyMs"`
}

type MiniLMTestResult struct {
	OK            bool   `json:"ok"`
	RuntimePath   string `json:"runtimePath"`
	ModelDir      string `json:"modelDir"`
	ModelPath     string `json:"modelPath"`
	TokenizerPath string `json:"tokenizerPath"`
	Message       string `json:"message"`
	LatencyMS     int64  `json:"latencyMs"`
	EmbeddingDim  int    `json:"embeddingDim"`
}

type EssentiaTestResult struct {
	OK          bool               `json:"ok"`
	RuntimePath string             `json:"runtimePath"`
	ModelDir    string             `json:"modelDir"`
	Base        ModelCheckResult   `json:"base"`
	Genre       ModelCheckResult   `json:"genre"`
	Heads       []ModelCheckResult `json:"heads"`
	Message     string             `json:"message"`
	LatencyMS   int64              `json:"latencyMs"`
}

type DebugReindexResult struct {
	Started bool   `json:"started"`
	Busy    bool   `json:"busy"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

func NewApp() *App {
	store, err := db.Open("ray-player1")
	if err != nil {
		panic(err)
	}
	appSettings, _ := store.GetAppState()
	var engine *onnx.Engine
	var essentiaEngine *onnx.EssentiaEngine
	var tempoEngine *onnx.TempoEngine

	runtimePath := strings.TrimSpace(appSettings.OnnxRuntimePath)
	modelDir, modelDirErr := onnx.ResolveEssentiaModelDir(
		appSettings.EssentiaModelDir,
	)
	if modelDirErr != nil {
		appLog.W("Essentia model bundle unavailable: %v", modelDirErr)
		modelDir = ""
	}
	miniLMDir, miniLMDirErr := onnx.ResolveMiniLMModelDir(
		appSettings.MiniLMModelDir,
	)
	if miniLMDirErr != nil {
		appLog.W("MiniLM model bundle unavailable: %v", miniLMDirErr)
		miniLMDir = ""
	}
	applyMediaToolPaths(appSettings.FFmpegPath, appSettings.FFprobePath)
	if miniLMDir != "" {
		if eng, initErr := onnx.New(runtimePath, miniLMDir); initErr == nil {
			engine = eng
			appLog.I("MiniLM engine ready modelDir=%q", miniLMDir)
		} else {
			appLog.W("MiniLM engine unavailable modelDir=%q err=%v", miniLMDir, initErr)
		}
	}
	if modelDir != "" {
		if mlEng, initErr := onnx.NewEssentiaEngine(runtimePath, modelDir); initErr == nil {
			essentiaEngine = mlEng
			appLog.I("essentia engine created OK modelDir=%q", modelDir)
		} else {
			appLog.E("essentia engine FAILED modelDir=%q err=%v", modelDir, initErr)
		}
		if tempoEng, initErr := onnx.NewTempoEngine(runtimePath, modelDir); initErr == nil {
			tempoEngine = tempoEng
			appLog.I("tempo engine ready modelDir=%q", modelDir)
		} else {
			appLog.W("tempo engine unavailable modelDir=%q err=%v", modelDir, initErr)
		}
	}

	lib := library.NewService(store, engine, essentiaEngine, tempoEngine)
	podcastSvc := podcast.NewService(store)
	podcastHistory := podcast.NewHistoryService(store)
	raySvc := rays.NewService(store, lib)
	state := appstate.NewStore(store)
	evt := events.NewService(store, lib)
	app := &App{
		store:          store,
		state:          state,
		library:        lib,
		podcasts:       podcastSvc,
		podcastHistory: podcastHistory,
		search:         search.NewService(store),
		rec:            recommend.NewService(evt, raySvc),
		events:         evt,
		rays:           raySvc,
		audio:          audio.NewService(state, evt),
		onnx:           engine,
		essentia:       essentiaEngine,
		tempo:          tempoEngine,
	}

	app.externalDownloads = externalmedia.NewWorker(
		store,
		externalmedia.Hooks{
			Settings: app.GetExternalMediaSettings,
			Emit: func(event string, payload externalmedia.JobDTO) {
				if app.ctx != nil {
					wruntime.EventsEmit(
						app.ctx,
						event,
						payload,
					)
				}
			},
			OnMusicReady: func(
				itemID string,
				path string,
			) error {
				err := app.library.AnalyzeExternalTrack(
					itemID,
					path,
				)
				_ = app.library.ReloadFromStore()
				return err
			},
			OnPodcastReady: func(
				itemID string,
				path string,
			) error {
				return nil
			},
			OnLibraryChanged: func() {
				app.refreshExternalLibraries()
				app.pushSnapshot()
			},
		},
	)
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.lifecycleMu.Lock()
	a.runCtx, a.runCancel = context.WithCancel(ctx)
	a.shuttingDown = false
	a.lifecycleMu.Unlock()

	if a.podcastHistory != nil {
		if err := a.podcastHistory.Recover(); err != nil {
			appLog.W("recover podcast history: %v", err)
		}
	}

	if a.externalDownloads != nil {
		a.externalDownloads.Start()
	}

	a.library.SetAnalyzedHook(func(track library.Track) {
		if len(track.Embedding) != 1280 {
			appLog.W("skip recluster invalid embedding track=%s emb=%d", track.ID, len(track.Embedding))
			return
		}
		a.ScheduleRecluster()
		a.pushSnapshot()
		wruntime.EventsEmit(a.ctx, "library:analyzed", track.ID)
	})
	a.library.SetIndexingHook(func(st library.IndexingState) {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "indexing:update", st)
		}
		if !st.IsIndexing && st.Phase == "done" {
			a.ScheduleRecluster()
		}
	})
	_ = a.library.Load()
	if !shouldDeferRecluster(false, a.library.IndexingState()) {
		a.RunReclusterSingleflight()
	}
	a.audio.SetOnEnded(a.handlePlaybackEnded)
	a.audio.SetOnStarted(a.handlePlaybackStarted)

	go func() {
		startedAt := time.Now()
		if err := a.audio.Warmup(); err != nil {
			appLog.W(
				"audio warmup failed err=%v",
				err,
			)
			return
		}
		appLog.I(
			"audio warmup completed ms=%d",
			time.Since(startedAt).Milliseconds(),
		)
	}()

	a.restorePlaybackSession()
	a.loadVolumeState()

	// Загружаем настройку нормализации подкастов.
	if normSettings := a.GetSettings(); normSettings.NormalizePodcastVolume {
		a.audio.SetPodcastNormalization(true)
	}

	a.launchBackground(func(ctx context.Context) {
		a.playbackTicker(ctx)
	})
	a.pushSnapshot()

	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "indexing:update", a.library.IndexingState())
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.shutdownOnce.Do(func() {
		a.shutdownNow(ctx)
	})
}

func (a *App) shutdownNow(ctx context.Context) {
	_ = ctx

	if a.externalDownloads != nil {
		a.externalDownloads.Close()
	}
	a.cancelPlayRequest()
	a.stopBackgroundWork()

	st := a.state.Get()
	if isPodcastTrackID(st.CurrentTrackID) && a.podcastHistory != nil {
		positionMs := a.audio.GetPositionMs()
		if positionMs < 0 {
			positionMs = st.PositionMs
		}
		_ = a.podcastHistory.Finish(
			st.CurrentTrackID,
			float64(positionMs)/1000,
			float64(st.DurationMs)/1000,
			"application_close",
		)
	}

	a.persistPlaybackSession(true)
	a.persistCurrentRayState()
	if a.audio != nil {
		a.audio.SetOnStarted(nil)
		a.audio.SetOnEnded(nil)
	}

	if a.library != nil {
		a.library.Close()
	}

	if a.onnx != nil {
		_ = a.onnx.Close()
	}
	if a.essentia != nil {
		_ = a.essentia.Close()
	}
	if a.tempo != nil {
		_ = a.tempo.Close()
	}
	_ = a.store.Close()
}

func (a *App) launchBackground(run func(context.Context)) bool {
	a.lifecycleMu.Lock()
	if a.shuttingDown {
		a.lifecycleMu.Unlock()
		return false
	}
	if a.runCtx == nil {
		a.runCtx, a.runCancel = context.WithCancel(context.Background())
	}
	ctx := a.runCtx
	a.backgroundWG.Add(1)
	a.lifecycleMu.Unlock()

	go func() {
		defer a.backgroundWG.Done()
		run(ctx)
	}()
	return true
}

func (a *App) stopBackgroundWork() {
	a.lifecycleMu.Lock()
	a.shuttingDown = true
	cancel := a.runCancel
	a.runCancel = nil
	a.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}

	a.reclusterMu.Lock()
	if a.reclusterTimer != nil {
		a.reclusterTimer.Stop()
		a.reclusterTimer = nil
	}
	a.reclusterPending = false
	a.reclusterMu.Unlock()

	a.backgroundWG.Wait()
}

func (a *App) hydrateQueueItems(queue []rays.QueueItem) []rays.QueueItem {
	if len(queue) == 0 || a.library == nil {
		return queue
	}
	out := append([]rays.QueueItem{}, queue...)
	for i := range out {
		if out[i].Track.ID != "" {
			continue
		}
		track, ok := a.library.TrackByID(out[i].TrackID)
		if !ok {
			continue
		}
		out[i].Track = track
		if out[i].Title == "" {
			out[i].Title = track.Title
		}
		if out[i].Subtitle == "" {
			out[i].Subtitle = track.Artist
		}
		if out[i].DurationLabel == "" {
			out[i].DurationLabel = track.DurationLabel
		}
	}
	return out
}

func (a *App) Bootstrap() BootstrapPayload {
	roots, _ := a.library.ListRoots()
	errs, _ := a.library.ListFileErrors(50)
	current := a.state.Get()
	current.Queue = a.hydrateQueueItems(current.Queue)
	buildState := a.GetRayBuildState()
	queue := visibleRayQueue(
		a.hydrateQueueItems(a.rays.CurrentQueue()),
		buildState,
	)
	podcasts, _ := a.podcasts.List(500)
	podcastHistory, _ := a.podcastHistory.List(200)
	podcastRays, _ := a.podcastHistory.RayList(100)
	return BootstrapPayload{
		Library:           a.library.AllTracks(),
		Podcasts:          podcasts,
		PodcastRay:        a.podcasts.CurrentRay(),
		PodcastPlayback:   a.podcastPlayback,
		PodcastHistory:    podcastHistory,
		PodcastRays:       podcastRays,
		Current:           current,
		History:           a.events.History(),
		Rays:              a.rays.Summaries(),
		Queue:             queue,
		MusicRay:          a.rays.CurrentRay(),
		LibraryStat:       a.library.Stats(),
		Roots:             roots,
		ImportErrors:      errs,
		EmoFlow:           a.GetCurrentEmoFlow(),
		EmoFlowUISettings: a.getEmoFlowSettings(),
		RayBuild:          buildState,
	}
}

func visibleRayQueue(
	queue []rays.QueueItem,
	build appstate.RayBuildState,
) []rays.QueueItem {
	if build.Status == appstate.RayBuildBuilding {
		return []rays.QueueItem{}
	}
	return queue
}

func (a *App) Snapshot() BootstrapPayload { return a.Bootstrap() }

func (a *App) GetPlaybackState() appstate.PlayerState {
	return a.state.Get()
}

func (a *App) rayBuildKey(
	seedID, mode, currentRayID string,
) string {
	return strings.TrimSpace(seedID) + "|" + strings.TrimSpace(mode) + "|" + strings.TrimSpace(currentRayID)
}

func (a *App) beginPlayRequest() (
	context.Context,
	uint64,
) {
	a.playRequestMu.Lock()
	defer a.playRequestMu.Unlock()

	if a.playRequestCancel != nil {
		a.playRequestCancel()
	}

	a.playRequestSeq++
	seq := a.playRequestSeq

	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	a.playRequestCancel = cancel

	return ctx, seq
}

func (a *App) isCurrentPlayRequest(seq uint64) bool {
	a.playRequestMu.Lock()
	defer a.playRequestMu.Unlock()
	return seq == a.playRequestSeq
}

func (a *App) finishPlayRequest(seq uint64) {
	a.playRequestMu.Lock()
	defer a.playRequestMu.Unlock()

	if seq != a.playRequestSeq {
		return
	}

	if a.playRequestCancel != nil {
		a.playRequestCancel()
		a.playRequestCancel = nil
	}
}

func (a *App) cancelPlayRequest() {
	a.playRequestMu.Lock()
	defer a.playRequestMu.Unlock()

	a.playRequestSeq++
	if a.playRequestCancel != nil {
		a.playRequestCancel()
		a.playRequestCancel = nil
	}
}

func (a *App) beginPlaybackEpoch() uint64 {
	return a.playbackEpoch.Add(1)
}

func (a *App) currentPlaybackEpoch() uint64 {
	return a.playbackEpoch.Load()
}

func (a *App) playbackEpochIsCurrent(epoch uint64) bool {
	return epoch != 0 && a.playbackEpoch.Load() == epoch
}

func (a *App) playbackTargetIsCurrent(
	epoch uint64,
	itemID string,
) bool {
	if !a.playbackEpochIsCurrent(epoch) {
		return false
	}
	state := a.state.Get()
	return state.CurrentTrackID == itemID
}

func (a *App) playbackTargetIsCurrentOrEmpty(
	epoch uint64,
	itemID string,
) bool {
	if !a.playbackEpochIsCurrent(epoch) {
		return false
	}
	state := a.state.Get()
	return state.CurrentTrackID == "" ||
		state.CurrentTrackID == itemID ||
		isPodcastTrackID(itemID)
}

func (a *App) setRayBuildState(
	next appstate.RayBuildState,
) {
	a.rayBuildMu.Lock()
	a.rayBuildState = next
	a.rayBuildMu.Unlock()

	if a.ctx != nil {
		wruntime.EventsEmit(
			a.ctx,
			"ray:build-state",
			next,
		)
	}
}

func (a *App) beginRayBuild(
	requestID uint64,
	seedTrackID string,
) {
	a.setRayBuildState(appstate.RayBuildState{
		Status:      appstate.RayBuildBuilding,
		SeedTrackID: seedTrackID,
		RequestID:   requestID,
		StartedAt:   time.Now().UnixMilli(),
	})
}

func (a *App) finishRayBuild(
	requestID uint64,
	seedTrackID string,
) {
	a.rayBuildMu.RLock()
	current := a.rayBuildState
	a.rayBuildMu.RUnlock()

	if current.RequestID != requestID {
		return
	}

	current.Status = appstate.RayBuildReady
	current.SeedTrackID = seedTrackID
	current.FinishedAt = time.Now().UnixMilli()
	current.LastError = ""
	a.setRayBuildState(current)
}

func (a *App) failRayBuild(
	requestID uint64,
	seedTrackID string,
	err error,
) {
	a.rayBuildMu.RLock()
	current := a.rayBuildState
	a.rayBuildMu.RUnlock()

	if current.RequestID != requestID {
		return
	}

	current.Status = appstate.RayBuildError
	current.SeedTrackID = seedTrackID
	current.FinishedAt = time.Now().UnixMilli()
	current.LastError = err.Error()
	a.setRayBuildState(current)
}

func (a *App) GetRayBuildState() appstate.RayBuildState {
	a.rayBuildMu.RLock()
	defer a.rayBuildMu.RUnlock()
	return a.rayBuildState
}

func (a *App) shouldReuseRayBuild(seedID, mode, currentRayID string) (bool, []rays.QueueItem) {
	a.rayBuildMu.Lock()
	defer a.rayBuildMu.Unlock()
	key := a.rayBuildKey(seedID, mode, currentRayID)
	now := time.Now()
	if key == a.lastRayBuildKey && len(a.lastRayQueue) > 0 && now.Sub(a.lastRayBuildAt) < 30*time.Second {
		return true, append([]rays.QueueItem{}, a.lastRayQueue...)
	}
	return false, nil
}

func (a *App) rememberRayBuild(seedID, mode, currentRayID string, queue []rays.QueueItem) {
	a.rayBuildMu.Lock()
	defer a.rayBuildMu.Unlock()
	a.lastRayBuildKey = a.rayBuildKey(seedID, mode, currentRayID)
	a.lastRayBuildAt = time.Now()
	a.lastRayQueue = append([]rays.QueueItem{}, queue...)
}

func (a *App) GetSettings() SettingsPayload {
	row, err := a.store.GetAppState()
	if err != nil {
		return SettingsPayload{RepeatRay: true, EmoFlowUI: emoflow.DefaultSettings()}
	}
	payload := SettingsPayload{
		OnnxRuntimePath:  row.OnnxRuntimePath,
		MiniLMModelDir:   row.MiniLMModelDir,
		EssentiaModelDir: row.EssentiaModelDir,
		FFmpegPath:       row.FFmpegPath,
		FFprobePath:      row.FFprobePath,
		StoragePath:      a.store.StoragePath(),
		RepeatRay:        row.RepeatRay,
		ExtendRay:        row.ExtendRay,
		EmoFlowUI: emoflow.UISettings{
			Enabled:              row.EmoFlowUIEnabled,
			Intensity:            row.EmoFlowUIIntensity,
			AnimateDuringTrack:   row.EmoFlowUIAnimateTrack,
			RespectReducedMotion: row.EmoFlowUIRespectReduced,
		},
	}
	if value, err2 := a.store.GetMeta(metaNormalizePodcastVolume); err2 == nil {
		payload.NormalizePodcastVolume = value == "1" || strings.EqualFold(value, "true")
	}
	return payload
}

// SetNormalizePodcastVolume включает/выключает нормализацию громкости для подкастов.
func (a *App) SetNormalizePodcastVolume(enabled bool) (SettingsPayload, error) {
	if err := a.store.SetMeta(
		metaNormalizePodcastVolume,
		strconv.FormatBool(enabled),
	); err != nil {
		return SettingsPayload{}, err
	}
	a.audio.SetPodcastNormalization(enabled)

	// Если сейчас играет подкаст — перезапускаем поток с новым filter chain.
	state := a.state.Get()
	if isPodcastTrackID(state.CurrentTrackID) {
		if err := a.restartPodcastForNormalization(state); err != nil {
			appLog.W("restart podcast for normalization err=%v", err)
		}
	}
	return a.GetSettings(), nil
}

func (a *App) restartPodcastForNormalization(state appstate.PlayerState) error {
	item, err := a.podcasts.ItemByID(state.CurrentTrackID)
	if err != nil {
		return err
	}

	positionMs := a.audio.GetPositionMs()
	if positionMs < 0 {
		positionMs = state.PositionMs
	}
	if positionMs < 0 {
		positionMs = 0
	}

	wasPlaying := state.Status == appstate.PlaybackPlaying

	track := podcast.AsTrack(item)
	if err := a.audio.PlayFreshPodcast(track); err != nil {
		return err
	}

	if positionMs > 0 {
		if err := a.audio.Seek(positionMs); err != nil {
			appLog.W("seek after normalize restart err=%v", err)
		}
	}

	if !wasPlaying {
		a.audio.Pause()
	}

	state.PositionMs = positionMs
	if wasPlaying {
		state.Status = appstate.PlaybackPlaying
	} else {
		state.Status = appstate.PlaybackPaused
	}
	a.state.Replace(state)
	a.pushSnapshot()
	return nil
}

func resolveRuntimePath(primary, fallback string) string {
	runtimePath := strings.TrimSpace(primary)
	if runtimePath != "" {
		return runtimePath
	}
	return strings.TrimSpace(fallback)
}

func (a *App) TestONNXRuntime(payload SettingsPayload) RuntimeTestResult {
	start := time.Now()
	configured := resolveRuntimePath(payload.OnnxRuntimePath, func() string {
		row, err := a.store.GetAppState()
		if err != nil {
			return ""
		}
		return row.OnnxRuntimePath
	}())
	result := RuntimeTestResult{}
	runtimePath, err := onnx.ResolveRuntimeLibraryPath(configured)
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	result.RuntimePath = runtimePath
	if err := onnx.TestRuntime(runtimePath); err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	result.OK = true
	result.Message = "ONNX Runtime доступен"
	result.LatencyMS = time.Since(start).Milliseconds()
	return result
}

func (a *App) TestMiniLM(payload SettingsPayload) MiniLMTestResult {
	start := time.Now()
	configuredRuntime := resolveRuntimePath(payload.OnnxRuntimePath, func() string {
		row, err := a.store.GetAppState()
		if err != nil {
			return ""
		}
		return row.OnnxRuntimePath
	}())
	configuredModelDir := strings.TrimSpace(payload.MiniLMModelDir)
	if configuredModelDir == "" {
		row, err := a.store.GetAppState()
		if err == nil {
			configuredModelDir = strings.TrimSpace(row.MiniLMModelDir)
		}
	}
	result := MiniLMTestResult{}
	runtimePath, err := onnx.ResolveRuntimeLibraryPath(configuredRuntime)
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	result.RuntimePath = runtimePath
	modelDir, err := onnx.ResolveMiniLMModelDir(configuredModelDir)
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	result.ModelDir = modelDir
	modelPath, tokenizerPath, err := onnx.ResolveModelFiles(modelDir)
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	result.ModelPath = modelPath
	result.TokenizerPath = tokenizerPath
	engine, err := onnx.New(runtimePath, modelDir)
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	vec, err := engine.Encode(ctx, "ray player model validation")
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	result.OK = true
	result.EmbeddingDim = len(vec)
	result.Message = fmt.Sprintf("MiniLM загружен, embedding=%d", len(vec))
	result.LatencyMS = time.Since(start).Milliseconds()
	return result
}

func (a *App) TestFFmpeg(path string) (string, error) {
	ffmpeg, ffprobe, err := deps.ResolveFFmpegTools(path, "")
	if err != nil {
		return "", err
	}
	line, err := audio.CheckFFmpeg(ffmpeg)
	if err != nil {
		return "", err
	}
	probeLine, err := audio.CheckFFmpeg(ffprobe)
	if err != nil {
		return "", fmt.Errorf("ffprobe unavailable: %w", err)
	}
	return line + " | " + probeLine, nil
}

func (a *App) DoctorCheck(component string, payload SettingsPayload) deps.Check {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return deps.CheckComponent(ctx, component, doctorSettings(payload))
}

func (a *App) DoctorRepair(component string, payload SettingsPayload) deps.RepairResult {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	return deps.RepairComponent(ctx, component, doctorSettings(payload))
}

func doctorSettings(payload SettingsPayload) deps.Settings {
	return deps.Settings{
		ONNXRuntimePath:  strings.TrimSpace(payload.OnnxRuntimePath),
		MiniLMModelDir:   strings.TrimSpace(payload.MiniLMModelDir),
		EssentiaModelDir: strings.TrimSpace(payload.EssentiaModelDir),
		FFmpegPath:       strings.TrimSpace(payload.FFmpegPath),
		FFprobePath:      strings.TrimSpace(payload.FFprobePath),
	}
}

func (a *App) TestEssentia(payload SettingsPayload) EssentiaTestResult {
	start := time.Now()
	configuredRuntime := resolveRuntimePath(payload.OnnxRuntimePath, func() string {
		row, err := a.store.GetAppState()
		if err != nil {
			return ""
		}
		return row.OnnxRuntimePath
	}())
	configuredModelDir := strings.TrimSpace(payload.EssentiaModelDir)
	if configuredModelDir == "" {
		row, err := a.store.GetAppState()
		if err == nil {
			configuredModelDir = strings.TrimSpace(row.EssentiaModelDir)
		}
	}
	result := EssentiaTestResult{}
	runtimePath, err := onnx.ResolveRuntimeLibraryPath(configuredRuntime)
	if err != nil {
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	modelDir, err := onnx.ResolveEssentiaModelDir(configuredModelDir)
	if err != nil {
		result.RuntimePath = runtimePath
		result.Message = err.Error()
		result.LatencyMS = time.Since(start).Milliseconds()
		return result
	}
	probe, probeErr := onnx.ProbeEssentia(runtimePath, modelDir)
	result = EssentiaTestResult{RuntimePath: runtimePath, ModelDir: modelDir, Message: probe.Message}
	result.Base = ModelCheckResult(probe.Base)
	result.Genre = ModelCheckResult(probe.Genre)
	result.Heads = make([]ModelCheckResult, 0, len(probe.Heads))
	loaded := 0
	for _, item := range probe.Heads {
		if item.Loaded {
			loaded++
		}
		result.Heads = append(result.Heads, ModelCheckResult(item))
	}
	result.OK = probeErr == nil && result.Base.Loaded && result.Genre.Loaded && loaded > 0
	if probeErr != nil && result.Message == "" {
		result.Message = probeErr.Error()
	}
	result.LatencyMS = time.Since(start).Milliseconds()
	if result.Message == "" {
		result.Message = fmt.Sprintf("Essentia base+genre loaded; %d/%d heads loaded", loaded, len(result.Heads))
	}
	return result
}

func (a *App) SaveSettings(payload SettingsPayload) (BootstrapPayload, error) {
	payload.OnnxRuntimePath = strings.TrimSpace(payload.OnnxRuntimePath)
	payload.MiniLMModelDir = strings.TrimSpace(payload.MiniLMModelDir)
	payload.EssentiaModelDir = strings.TrimSpace(payload.EssentiaModelDir)
	payload.FFmpegPath = strings.TrimSpace(payload.FFmpegPath)
	payload.FFprobePath = strings.TrimSpace(payload.FFprobePath)
	payload.EmoFlowUI = emoflow.NormalizeSettings(payload.EmoFlowUI)
	if !payload.RepeatRay && !payload.ExtendRay {
		payload.RepeatRay = false
	}
	if payload.FFmpegPath == "" {
		payload.FFmpegPath = "ffmpeg"
	}
	if payload.FFprobePath == "" {
		payload.FFprobePath = analysis.FFprobePath()
	}
	newText, newEssentia, newTempo, err := prepareModelEngines(
		payload.OnnxRuntimePath,
		payload.MiniLMModelDir,
		payload.EssentiaModelDir,
	)
	if err != nil {
		return BootstrapPayload{}, err
	}
	committed := false
	defer func() {
		if !committed {
			closeModelEngines(newText, newEssentia, newTempo)
		}
	}()

	if err := a.store.SetAppSettings(db.AppStateRow{OnnxRuntimePath: payload.OnnxRuntimePath, MiniLMModelDir: payload.MiniLMModelDir, EssentiaModelDir: payload.EssentiaModelDir, FFmpegPath: payload.FFmpegPath, FFprobePath: payload.FFprobePath, RepeatRay: payload.RepeatRay, ExtendRay: payload.ExtendRay, EmoFlowUIEnabled: payload.EmoFlowUI.Enabled, EmoFlowUIIntensity: payload.EmoFlowUI.Intensity, EmoFlowUIAnimateTrack: payload.EmoFlowUI.AnimateDuringTrack, EmoFlowUIRespectReduced: payload.EmoFlowUI.RespectReducedMotion}); err != nil {
		return BootstrapPayload{}, err
	}

	oldText, oldEssentia, oldTempo := a.onnx, a.essentia, a.tempo
	a.onnx, a.essentia, a.tempo = newText, newEssentia, newTempo
	a.library.UpdateEngines(newText, newEssentia, newTempo)
	committed = true
	closeModelEngines(oldText, oldEssentia, oldTempo)

	applyMediaToolPaths(payload.FFmpegPath, payload.FFprobePath)
	a.pushSnapshot()
	a.emitEmoFlowUpdate()
	return a.Bootstrap(), nil
}

func applyMediaToolPaths(configuredFFmpeg, configuredFFprobe string) {
	ffmpegPath := strings.TrimSpace(configuredFFmpeg)
	ffprobePath := strings.TrimSpace(configuredFFprobe)
	if resolvedFFmpeg, resolvedFFprobe, err := deps.ResolveFFmpegTools(ffmpegPath, ffprobePath); err == nil {
		ffmpegPath = resolvedFFmpeg
		ffprobePath = resolvedFFprobe
	} else {
		appLog.W("ffmpeg tools unresolved configuredFFmpeg=%q configuredFFprobe=%q err=%v", configuredFFmpeg, configuredFFprobe, err)
	}
	analysis.SetFFmpegPath(ffmpegPath)
	analysis.SetFFprobePath(ffprobePath)
	audio.SetFFmpegPath(ffmpegPath)
	audio.SetFFprobePath(ffprobePath)
}

func prepareModelEngines(
	runtimePath,
	miniLMConfigured,
	essentiaConfigured string,
) (*onnx.Engine, *onnx.EssentiaEngine, *onnx.TempoEngine, error) {
	miniLMDir, miniErr := onnx.ResolveMiniLMModelDir(miniLMConfigured)
	if miniErr != nil && strings.TrimSpace(miniLMConfigured) != "" {
		return nil, nil, nil, miniErr
	}
	if miniErr != nil {
		miniLMDir = ""
	}
	essentiaDir, essentiaErr := onnx.ResolveEssentiaModelDir(essentiaConfigured)
	if essentiaErr != nil && strings.TrimSpace(essentiaConfigured) != "" {
		return nil, nil, nil, essentiaErr
	}
	if essentiaErr != nil {
		essentiaDir = ""
	}

	var text *onnx.Engine
	var essentiaEngine *onnx.EssentiaEngine
	var tempoEngine *onnx.TempoEngine
	cleanup := func() { closeModelEngines(text, essentiaEngine, tempoEngine) }

	var err error
	if miniLMDir != "" {
		text, err = onnx.New(runtimePath, miniLMDir)
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("load MiniLM: %w", err)
		}
	}
	if essentiaDir != "" {
		essentiaEngine, err = onnx.NewEssentiaEngine(runtimePath, essentiaDir)
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("load Essentia: %w", err)
		}
		tempoEngine, err = onnx.NewTempoEngine(runtimePath, essentiaDir)
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("load tempo model: %w", err)
		}
	}
	return text, essentiaEngine, tempoEngine, nil
}

func closeModelEngines(
	text *onnx.Engine,
	essentiaEngine *onnx.EssentiaEngine,
	tempoEngine *onnx.TempoEngine,
) {
	if text != nil {
		_ = text.Close()
	}
	if essentiaEngine != nil {
		_ = essentiaEngine.Close()
	}
	if tempoEngine != nil {
		_ = tempoEngine.Close()
	}
}

func (a *App) DebugReindexLibrary() DebugReindexResult {
	a.reindexMu.Lock()
	if a.reindexRunning {
		a.reindexMu.Unlock()
		return DebugReindexResult{Started: false, Busy: true, Message: "reindex already running"}
	}
	a.reindexRunning = true
	a.reindexMu.Unlock()

	a.reclusterMu.Lock()
	if a.reclusterTimer != nil {
		a.reclusterTimer.Stop()
		a.reclusterTimer = nil
	}
	a.reclusterPending = false
	a.reclusterMu.Unlock()

	tracks := a.library.AllTracks()
	result := DebugReindexResult{Started: true, Busy: true, Total: len(tracks), Message: fmt.Sprintf("reindex queued: %d tracks", len(tracks))}
	appLog.I("debug reindex requested total=%d ctxNil=%t", len(tracks), a.ctx == nil)
	started := a.launchBackground(func(ctx context.Context) {
		defer func() {
			a.reindexMu.Lock()
			a.reindexRunning = false
			a.reindexMu.Unlock()
		}()
		if err := ctx.Err(); err != nil {
			appLog.I("reindex aborted before start err=%v", err)
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "app:reindex:done", map[string]any{"ok": false, "message": err.Error(), "total": len(tracks)})
			}
			return
		}
		progress := func(p library.RebuildProgress) {
			appLog.I("reindex progress %d/%d track=%s stage=%s state=%s msg=%s", p.Index, p.Total, p.TrackID, p.Stage, p.State, p.Message)
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "app:reindex:progress", p)
			}
		}
		err := a.library.RebuildStale(ctx, progress)
		if err != nil {
			appLog.I("reindex failed err=%v", err)
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "app:reindex:done", map[string]any{"ok": false, "message": err.Error(), "total": len(tracks)})
			}
			return
		}
		if err := ctx.Err(); err != nil {
			appLog.I("reindex cancelled after rebuild err=%v", err)
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "app:reindex:done", map[string]any{"ok": false, "message": err.Error(), "total": len(tracks)})
			}
			return
		}
		// RebuildStale emits one analyzed event per track. Re-clustering the
		// partially refreshed library on every debounce made cluster IDs churn
		// during reindex. Keep the batch marked busy and cluster exactly once
		// from the coherent final set before the deferred unlock exposes idle.
		a.RunReclusterSingleflight()
		a.pushSnapshot()
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "app:reindex:done", map[string]any{"ok": true, "message": fmt.Sprintf("reindexed %d tracks", len(tracks)), "total": len(tracks)})
		}
		appLog.I("debug reindex complete total=%d", len(tracks))
	})
	if !started {
		a.reindexMu.Lock()
		a.reindexRunning = false
		a.reindexMu.Unlock()
		return DebugReindexResult{Started: false, Busy: false, Total: len(tracks), Message: "application is shutting down"}
	}
	return result
}

func (a *App) ChooseDirectory(title string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
}

func (a *App) ChooseFile(title string, filterPattern string) (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   title,
		Filters: []wruntime.FileFilter{{DisplayName: "File", Pattern: filterPattern}},
	})
}

func (a *App) AddFolder() (BootstrapPayload, error) {
	folder, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Выберите папку с музыкой"})
	if err != nil {
		return BootstrapPayload{}, err
	}
	if strings.TrimSpace(folder) == "" {
		return a.Bootstrap(), nil
	}
	_, err = a.ImportPaths([]string{folder})
	if err != nil {
		return BootstrapPayload{}, err
	}
	a.ScheduleRecluster()
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) AddFiles() (BootstrapPayload, error) {
	files, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Выберите аудиофайлы", Filters: []wruntime.FileFilter{{DisplayName: "Audio", Pattern: "*.mp3;*.flac;*.wav;*.ogg;*.m4a;*.aac;*.opus;*.wma;*.aiff;*.aif"}}})
	if err != nil {
		return BootstrapPayload{}, err
	}
	_, err = a.ImportPaths(files)
	if err != nil {
		return BootstrapPayload{}, err
	}
	a.ScheduleRecluster()
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) ImportPaths(paths []string) (ImportResult, error) {
	result, err := a.library.ImportPaths(paths, func(progress library.ImportProgress) {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "import:progress", progress)
		}
	})
	if err != nil {
		return result, err
	}
	if result.Added > 0 || result.Updated > 0 {
		a.ScheduleRecluster()
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "import:result", result)
		wruntime.EventsEmit(a.ctx, "indexing:update", a.library.IndexingState())
	}
	a.pushSnapshot()
	return result, nil
}

func (a *App) AddPodcastFolder() (BootstrapPayload, error) {
	folder, err := wruntime.OpenDirectoryDialog(
		a.ctx,
		wruntime.OpenDialogOptions{
			Title: "Выберите папку с подкастами",
		},
	)
	if err != nil {
		return BootstrapPayload{}, err
	}
	if strings.TrimSpace(folder) == "" {
		return a.Bootstrap(), nil
	}

	if _, err := a.ImportPodcastPaths([]string{folder}); err != nil {
		return BootstrapPayload{}, err
	}
	return a.Bootstrap(), nil
}

func (a *App) AddPodcastFiles() (BootstrapPayload, error) {
	files, err := wruntime.OpenMultipleFilesDialog(
		a.ctx,
		wruntime.OpenDialogOptions{
			Title: "Выберите выпуски подкастов",
			Filters: []wruntime.FileFilter{{
				DisplayName: "Audio",
				Pattern:     "*.mp3;*.flac;*.wav;*.ogg;*.m4a;*.aac;*.opus;*.wma;*.aiff;*.aif",
			}},
		},
	)
	if err != nil {
		return BootstrapPayload{}, err
	}

	if _, err := a.ImportPodcastPaths(files); err != nil {
		return BootstrapPayload{}, err
	}
	return a.Bootstrap(), nil
}

func (a *App) ImportPodcastPaths(
	paths []string,
) (podcast.ImportResult, error) {
	result, err := a.podcasts.ImportPaths(paths)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "podcasts:updated", result)
	}
	a.pushSnapshot()
	return result, err
}

func (a *App) SearchPodcasts(query string) ([]podcast.Item, error) {
	return a.podcasts.Search(query, 100)
}

func (a *App) UpdatePodcastProgress(
	itemID string,
	positionSeconds float64,
	durationSeconds float64,
) (podcast.Item, error) {
	item, err := a.podcasts.UpdateProgress(
		itemID,
		positionSeconds,
		durationSeconds,
	)
	if err == nil {
		a.pushSnapshot()
	}
	return item, err
}

func (a *App) PlayPodcast(
	itemID string,
) (BootstrapPayload, error) {
	epoch := a.beginPlaybackEpoch()

	item, err := a.podcasts.ItemByID(itemID)
	if err != nil {
		return BootstrapPayload{}, err
	}
	if item.SourceType == "yt_dlp" &&
		item.DownloadStatus != "ready" {
		return BootstrapPayload{}, fmt.Errorf(
			"подкаст ещё не готов: статус загрузки %s",
			item.DownloadStatus,
		)
	}
	if strings.TrimSpace(item.Path) == "" {
		return BootstrapPayload{}, errors.New("podcast path is empty")
	}

	ray := a.podcasts.CurrentRay()
	needsNewRay :=
		ray.ID == "" ||
			ray.SeedItemID != itemID ||
			!podcastRayContains(ray, itemID)
	if needsNewRay {
		ray, err = a.podcasts.BuildRay(itemID, 20)
		if err != nil {
			return BootstrapPayload{}, err
		}
	}

	item, err = a.podcasts.SelectRayItem(itemID)
	if err != nil {
		return BootstrapPayload{}, err
	}

	a.syncPodcastPlaybackFromRay(ray)

	if err := a.playPodcastItem(
		item,
		a.podcasts.CurrentRay().ID,
		true,
		epoch,
		"library",
	); err != nil {
		return BootstrapPayload{}, err
	}

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) PlayPodcastRayItem(
	itemID string,
) (BootstrapPayload, error) {
	epoch := a.beginPlaybackEpoch()

	item, err := a.podcasts.SelectRayItem(itemID)
	if err != nil {
		return BootstrapPayload{}, err
	}

	if err := a.playPodcastItem(
		item,
		a.podcasts.CurrentRay().ID,
		true,
		epoch,
		"ray",
	); err != nil {
		return BootstrapPayload{}, err
	}

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) NextPodcast() (BootstrapPayload, error) {
	item, ok := a.podcasts.NextRayItem()
	if !ok {
		return a.Bootstrap(), nil
	}
	epoch := a.beginPlaybackEpoch()

	if err := a.playPodcastItem(
		item,
		a.podcasts.CurrentRay().ID,
		true,
		epoch,
		"ray_auto",
	); err != nil {
		return BootstrapPayload{}, err
	}

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) PreviousPodcast() (BootstrapPayload, error) {
	item, ok := a.podcasts.PreviousRayItem()
	if !ok {
		return a.Bootstrap(), nil
	}
	epoch := a.beginPlaybackEpoch()

	if err := a.playPodcastItem(
		item,
		a.podcasts.CurrentRay().ID,
		true,
		epoch,
		"ray_previous",
	); err != nil {
		return BootstrapPayload{}, err
	}

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) SetPodcastRayContentMode(
	mode string,
) (BootstrapPayload, error) {
	ray, err := a.podcasts.SetCurrentRayContentMode(
		podcast.RayContentMode(mode),
	)
	if err != nil {
		return BootstrapPayload{}, err
	}

	a.syncPodcastPlaybackFromRay(ray)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) SetPodcastRaySortMode(
	mode string,
) (BootstrapPayload, error) {
	ray, err := a.podcasts.SetCurrentRaySortMode(
		podcast.RaySortMode(mode),
	)
	if err != nil {
		return BootstrapPayload{}, err
	}

	a.syncPodcastPlaybackFromRay(ray)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) MovePodcastRayItem(
	from int,
	to int,
) (BootstrapPayload, error) {
	ray, err := a.podcasts.MoveCurrentRayItem(from, to)
	if err != nil {
		return BootstrapPayload{}, err
	}

	a.syncPodcastPlaybackFromRay(ray)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) RemovePodcastRayItem(
	itemID string,
) (BootstrapPayload, error) {
	ray, err := a.podcasts.RemoveCurrentRayItem(itemID)
	if err != nil {
		return BootstrapPayload{}, err
	}

	a.syncPodcastPlaybackFromRay(ray)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) OpenPodcastRayHistory(rayID string) (BootstrapPayload, error) {
	st := a.state.Get()
	currentItemID := ""
	if isPodcastTrackID(st.CurrentTrackID) {
		currentItemID = st.CurrentTrackID
	}

	ray, err := a.podcasts.OpenSavedRay(rayID, currentItemID)
	if err != nil {
		return BootstrapPayload{}, err
	}

	a.syncPodcastPlaybackFromRay(ray)

	if currentItemID != "" {
		st.QueueID = ray.ID
		st.RayID = ray.ID
		st.CurrentRayID = ray.ID
		st.RaySeedTrackID = ray.SeedItemID
		st.QueueLength = len(ray.Items)
		st.QueueIndex = queueIndexByPodcastID(ray.Items, currentItemID)
		a.state.Replace(st)
	}

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func queueIndexByPodcastID(items []podcast.RayItem, itemID string) int {
	for index, item := range items {
		if item.Item.ID == itemID {
			return index
		}
	}
	return -1
}

func (a *App) syncPodcastPlaybackFromRay(
	ray podcast.Ray,
) {
	if a.podcastPlayback.ItemID == "" {
		return
	}

	a.podcastPlayback.RayID = ray.ID
	a.podcastPlayback.QueueLength = len(ray.Items)
	a.podcastPlayback.QueueIndex = -1
	for index, item := range ray.Items {
		if item.Item.ID == a.podcastPlayback.ItemID {
			a.podcastPlayback.QueueIndex = index
			break
		}
	}
}

func (a *App) playPodcastItem(
	item podcast.Item,
	rayID string,
	countPlay bool,
	epoch uint64,
	source string,
) error {
	if !a.playbackEpochIsCurrent(epoch) {
		return context.Canceled
	}

	track := podcast.AsTrack(item)

	previous := a.state.Get()
	if a.podcastHistory != nil &&
		isPodcastTrackID(previous.CurrentTrackID) &&
		previous.CurrentTrackID != item.ID {
		previousPosition := a.audio.GetPositionMs()
		if previousPosition < 0 {
			previousPosition = previous.PositionMs
		}
		_ = a.podcastHistory.Finish(
			previous.CurrentTrackID,
			float64(previousPosition)/1000,
			float64(previous.DurationMs)/1000,
			"switch_item",
		)
	}

	if source == "" {
		source = "manual"
	}

	a.logCurrentSkip("podcast_switch")
	a.persistCurrentRayState()

	if err := a.audio.PlayFreshPodcast(track); err != nil {
		return err
	}

	if !a.playbackEpochIsCurrent(epoch) {
		return context.Canceled
	}

	resumeMs := int(item.ResumePosition * 1000)
	if resumeMs > 0 {
		if err := a.audio.Seek(resumeMs); err != nil {
			return err
		}
	}

	if !a.playbackEpochIsCurrent(epoch) {
		return context.Canceled
	}

	ray := a.podcasts.CurrentRay()
	queueIndex := -1
	for index, rayItem := range ray.Items {
		if rayItem.Item.ID == item.ID {
			queueIndex = index
			break
		}
	}

	a.podcastPlayback = podcast.Playback{
		ItemID:      item.ID,
		RayID:       rayID,
		QueueIndex:  queueIndex,
		QueueLength: len(ray.Items),
		ResumeMs:    resumeMs,
		DurationMs:  track.DurationMs,
		Title:       item.Title,
		Author:      item.Author,
	}

	st := a.state.Get()
	st.Status = appstate.PlaybackPlaying
	st.CurrentTrackID = item.ID
	st.CurrentPath = item.Path
	st.CurrentTitle = item.Title
	st.CurrentArtist = item.Author
	st.CurrentSub = "Подкаст"
	st.DurationMs = track.DurationMs
	st.DurationLabel = item.DurationLabel
	st.PositionMs = resumeMs
	st.PositionLabel = formatPlaybackPosition(resumeMs)
	st.QueueID = rayID
	st.QueueIndex = queueIndex
	st.QueueLength = len(ray.Items)
	st.RayID = rayID
	st.CurrentRayID = rayID
	st.RaySeedTrackID = ray.SeedItemID
	st.Queue = nil
	st.LastError = ""

	if !a.playbackTargetIsCurrentOrEmpty(epoch, item.ID) {
		return context.Canceled
	}
	a.state.Replace(st)

	if countPlay {
		_ = a.store.IncrementPodcastPlayCount(item.ID)
	}
	if a.podcastHistory != nil {
		if err := a.podcastHistory.Begin(
			item,
			rayID,
			source,
			float64(resumeMs)/1000,
		); err != nil {
			appLog.W("begin podcast history item=%s: %v", item.ID, err)
		}
	}
	a.emitPlaybackUpdate()
	return nil
}

func podcastRayContains(ray podcast.Ray, itemID string) bool {
	for _, item := range ray.Items {
		if item.Item.ID == itemID {
			return true
		}
	}
	return false
}

func (a *App) SearchTracks(query string) []search.Result { return a.search.Query(query, 50) }

func (a *App) PlayTrack(trackID string) (BootstrapPayload, error) {
	return a.PlayTrackWithMode(trackID, "")
}

func (a *App) beginTrackLoading(
	track library.Track,
	queue []rays.QueueItem,
	rayID string,
	raySeedTrackID string,
) {
	st := a.state.Get()
	st.Status = appstate.PlaybackLoading
	st.CurrentTrackID = track.ID
	st.CurrentPath = track.Path
	st.CurrentTitle = track.Title
	st.CurrentArtist = track.Artist
	st.DurationMs = track.DurationMs
	st.DurationLabel = track.DurationLabel
	st.PositionMs = 0
	st.PositionLabel = "0:00"
	st.Queue = append([]rays.QueueItem(nil), queue...)
	st.QueueID = rayID
	st.RayID = rayID
	st.CurrentRayID = rayID
	st.RaySeedTrackID = raySeedTrackID
	st.QueueIndex = queueIndexByTrackID(queue, track.ID)
	st.QueueLength = len(queue)
	st.LastError = ""

	a.state.Replace(st)
	a.emitPlaybackUpdate()
}

func queueIndexByTrackID(
	queue []rays.QueueItem,
	trackID string,
) int {
	for index, item := range queue {
		if item.TrackID == trackID {
			return index
		}
	}
	return -1
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (a *App) setPendingRaySeed(
	track library.Track,
) {
	st := a.state.Get()
	st.Status = appstate.PlaybackLoading
	st.CurrentTrackID = track.ID
	st.CurrentPath = track.Path
	st.CurrentTitle = track.Title
	st.CurrentArtist = track.Artist
	st.CurrentSub = "строим луч…"
	st.DurationMs = track.DurationMs
	st.DurationLabel = track.DurationLabel
	st.PositionMs = 0
	st.PositionLabel = "0:00"
	st.QueueID = ""
	st.QueueIndex = 0
	st.QueueLength = 1
	st.RayID = ""
	st.CurrentRayID = ""
	st.RaySeedTrackID = track.ID
	st.Queue = nil
	st.LastError = ""

	a.state.Replace(st)
	a.emitPlaybackUpdate()

	if a.ctx != nil {
		wruntime.EventsEmit(
			a.ctx,
			"ray:building",
			map[string]any{
				"seedTrackId": track.ID,
			},
		)
	}
}

func (a *App) PlayTrackWithMode(
	trackID,
	mode string,
) (BootstrapPayload, error) {
	requestCtx, requestSeq := a.beginPlayRequest()
	defer a.finishPlayRequest(requestSeq)

	startedAt := time.Now()
	track, ok := a.library.TrackByID(trackID)
	if !ok {
		return BootstrapPayload{},
			fmt.Errorf("track not found: %s", trackID)
	}

	appLog.I(
		"play request begin seq=%d seed=%s mode=%s",
		requestSeq,
		track.ID,
		mode,
	)

	a.beginRayBuild(requestSeq, track.ID)

	a.logCurrentSkip("play_track_switch")
	a.persistCurrentRayState()
	currentRayID := a.state.Get().CurrentRayID
	current := a.state.Get()
	currentQueue := a.rays.CurrentQueue()
	if current.CurrentTrackID == track.ID &&
		(len(current.Queue) > 0 || len(currentQueue) > 0) {
		if !a.isCurrentPlayRequest(requestSeq) {
			return a.Bootstrap(), nil
		}

		if err := a.playTrackSafe(track, false); err != nil {
			return BootstrapPayload{}, err
		}
		rayID := current.CurrentRayID
		if strings.TrimSpace(rayID) == "" {
			if len(currentQueue) == 0 {
				currentQueue = current.Queue
			}
			rayID = a.rays.Activate(track, currentQueue)
		}
		a.state.SetCurrent(track, rayID, a.rays.CurrentQueue(), 0)
		a.state.SetRaySeed(a.rays.CurrentRay().SeedTrackID)
		_ = a.events.MarkPlay(track)
		a.persistPlaybackSession(true)
		a.finishRayBuild(requestSeq, track.ID)
		a.pushSnapshot()
		return a.Bootstrap(), nil
	}

	if reuse, cached := a.shouldReuseRayBuild(
		track.ID,
		mode,
		currentRayID,
	); reuse {
		if !a.isCurrentPlayRequest(requestSeq) {
			return a.Bootstrap(), nil
		}

		if err := a.playTrackSafe(track, false); err != nil {
			return BootstrapPayload{}, err
		}
		rayID := a.rays.Activate(track, cached)
		a.rememberRayBuild(track.ID, mode, currentRayID, cached)
		a.state.SetCurrent(track, rayID, a.rays.CurrentQueue(), 0)
		a.state.SetRaySeed(track.ID)
		_ = a.events.MarkPlay(track)
		a.persistPlaybackSession(true)
		a.finishRayBuild(requestSeq, track.ID)
		a.pushSnapshot()
		return a.Bootstrap(), nil
	}

	a.setPendingRaySeed(track)

	if !a.isCurrentPlayRequest(requestSeq) {
		return a.Bootstrap(), nil
	}

	if err := a.playTrackSafe(track, false); err != nil {
		if !a.isCurrentPlayRequest(requestSeq) {
			return a.Bootstrap(), nil
		}
		return BootstrapPayload{}, err
	}

	tracks := a.usableTracksForRay(track)
	queue, err := a.rec.BuildRayWithModeContext(
		requestCtx,
		track,
		tracks,
		currentRayID,
		mode,
	)

	if err != nil {
		if errors.Is(err, recommend.ErrRayBuildCanceled) ||
			!a.isCurrentPlayRequest(requestSeq) {
			appLog.I(
				"play request superseded seq=%d seed=%s ms=%d",
				requestSeq,
				track.ID,
				time.Since(startedAt).Milliseconds(),
			)
			return a.Bootstrap(), nil
		}

		a.failRayBuild(requestSeq, track.ID, err)
		return BootstrapPayload{},
			fmt.Errorf("build ray: %w", err)
	}

	a.finishRayBuild(requestSeq, track.ID)

	if !a.isCurrentPlayRequest(requestSeq) {
		appLog.I(
			"discard stale ray seq=%d seed=%s",
			requestSeq,
			track.ID,
		)
		return a.Bootstrap(), nil
	}

	a.rememberRayBuild(
		track.ID,
		mode,
		currentRayID,
		queue,
	)

	rayID := a.rays.Activate(track, queue)
	a.state.SetCurrent(
		track,
		rayID,
		a.rays.CurrentQueue(),
		0,
	)
	a.state.SetRaySeed(track.ID)
	_ = a.events.MarkPlay(track)
	a.persistPlaybackSession(true)
	a.pushSnapshot()

	appLog.I(
		"play request committed seq=%d seed=%s ray=%s tracks=%d queue=%d ms=%d",
		requestSeq,
		track.ID,
		rayID,
		len(tracks),
		len(queue),
		time.Since(startedAt).Milliseconds(),
	)

	return a.Bootstrap(), nil
}

func (a *App) RayAudit(trackID, mode string, limit int) (recommend.RayAuditResult, error) {
	track, ok := a.library.TrackByID(trackID)
	if !ok {
		return recommend.RayAuditResult{}, fmt.Errorf("track not found: %s", trackID)
	}
	return a.rec.AuditRay(track, a.usableTracksForRay(track), mode, limit), nil
}

func (a *App) ResumeRay(rayID string) (BootstrapPayload, error) {
	current := a.state.Get()
	if current.CurrentRayID == rayID && current.Playing {
		a.pushSnapshot()
		return a.Bootstrap(), nil
	}
	ray, ok := a.rays.Resume(rayID)
	if !ok {
		return BootstrapPayload{}, fmt.Errorf("ray not found: %s", rayID)
	}
	track, ok := a.library.TrackByID(ray.CurrentTrackID)
	if !ok {
		return BootstrapPayload{}, fmt.Errorf("track not found: %s", ray.CurrentTrackID)
	}
	if current.CurrentRayID != "" && current.CurrentRayID != rayID {
		a.logCurrentSkip("resume_other_ray")
	}
	a.persistCurrentRayState()
	if err := a.playTrackSafe(track, true); err != nil {
		return BootstrapPayload{}, err
	}
	if ray.PositionMs > 0 {
		_ = a.audio.Seek(ray.PositionMs)
	}
	a.state.SetCurrent(track, ray.ID, ray.Queue, ray.PositionMs)
	a.state.SetRaySeed(ray.SeedTrackID)
	if current.CurrentTrackID != track.ID || !current.Playing {
		_ = a.events.MarkPlay(track)
	}
	_ = a.events.MarkProgress(track, "resume_ray", ray.PositionMs)
	a.rewardCurrentQueueItem("resume_ray", 0.40)
	a.persistPlaybackSession(true)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) TogglePlay() (appstate.PlayerState, error) {
	st := a.state.Get()

	switch st.Status {
	case appstate.PlaybackPlaying:
		appLog.I("pause current track=%s", st.CurrentTrackID)

		if !a.audio.Pause() {
			return a.setPlaybackError(
				st,
				fmt.Errorf(
					"cannot pause current track: audio stream is not loaded",
				),
			)
		}

		if position := a.audio.GetPositionMs(); position >= 0 {
			st.PositionMs = position
			st.PositionLabel = formatPlaybackPosition(position)
		}
		st.Status = appstate.PlaybackPaused
		st.LastError = ""
		if a.podcastHistory != nil && isPodcastTrackID(st.CurrentTrackID) {
			_ = a.podcastHistory.Pause(
				st.CurrentTrackID,
				float64(st.PositionMs)/1000,
				float64(st.DurationMs)/1000,
			)
		}

	case appstate.PlaybackPaused:
		if st.CurrentTrackID == "" {
			appLog.E("play requested but no current track/queue")
			return st, fmt.Errorf("no current track to resume")
		}

		if a.audio.HasActiveStream() {
			appLog.I(
				"resume loaded stream id=%s position=%d",
				st.CurrentTrackID,
				st.PositionMs,
			)

			if !a.audio.Resume() {
				return a.setPlaybackError(
					st,
					fmt.Errorf(
						"cannot resume current audio stream: %s",
						st.CurrentTrackID,
					),
				)
			}

			st.Status = appstate.PlaybackPlaying
			st.LastError = ""
			break
		}

		appLog.I(
			"resume requires stream restore id=%s position=%d",
			st.CurrentTrackID,
			st.PositionMs,
		)
		if err := a.restartCurrentPlayback(st); err != nil {
			return a.setPlaybackError(st, err)
		}

		st = a.state.Get()
		st.Status = appstate.PlaybackPlaying
		st.LastError = ""

	case appstate.PlaybackStopped:
		if st.CurrentTrackID == "" {
			appLog.E("play requested but no current track/queue")
			return st, fmt.Errorf("no current track to resume")
		}

		appLog.I(
			"restart stopped current id=%s position=%d",
			st.CurrentTrackID,
			st.PositionMs,
		)
		if err := a.restartCurrentPlayback(st); err != nil {
			return a.setPlaybackError(st, err)
		}

		st = a.state.Get()
		st.Status = appstate.PlaybackPlaying
		st.LastError = ""

	case appstate.PlaybackError:
		if st.CurrentTrackID == "" {
			return st, fmt.Errorf(
				"playback is in error state: %s",
				st.LastError,
			)
		}

		appLog.I(
			"retry playback after error id=%s error=%q",
			st.CurrentTrackID,
			st.LastError,
		)

		if err := a.restartCurrentPlayback(st); err != nil {
			return a.setPlaybackError(st, err)
		}

		st = a.state.Get()
		st.Status = appstate.PlaybackPlaying
		st.LastError = ""

	case appstate.PlaybackLoading:
		return st, nil
	}

	a.state.Replace(st)

	if st.Status == appstate.PlaybackPaused &&
		isPodcastTrackID(st.CurrentTrackID) {
		_, _ = a.podcasts.UpdateProgress(
			st.CurrentTrackID,
			float64(st.PositionMs)/1000,
			float64(st.DurationMs)/1000,
		)
		a.lastPodcastProgressAt = time.Now()
		a.lastPodcastPositionMs = st.PositionMs
	} else if st.Status != appstate.PlaybackPlaying {
		a.persistCurrentRayState()
	}

	a.persistPlaybackSession(true)
	a.pushSnapshot()
	return a.state.Get(), nil
}

type currentPlaybackKind string

const (
	currentPlaybackMusic   currentPlaybackKind = "music"
	currentPlaybackPodcast currentPlaybackKind = "podcast"
)

func playbackKindForID(id string) currentPlaybackKind {
	if isPodcastTrackID(id) {
		return currentPlaybackPodcast
	}
	return currentPlaybackMusic
}

func (a *App) restartCurrentPlayback(
	st appstate.PlayerState,
) error {
	if st.CurrentTrackID == "" {
		return fmt.Errorf("no current track to restart")
	}

	positionMs := st.PositionMs
	if current := a.audio.GetPositionMs(); current > positionMs {
		positionMs = current
	}

	if playbackKindForID(st.CurrentTrackID) == currentPlaybackPodcast {
		item, err := a.podcasts.ItemByID(st.CurrentTrackID)
		if err != nil {
			return fmt.Errorf(
				"current podcast not found: %s: %w",
				st.CurrentTrackID,
				err,
			)
		}

		rayID := st.CurrentRayID
		if rayID == "" {
			rayID = a.podcasts.CurrentRay().ID
		}

		epoch := a.currentPlaybackEpoch()
		if epoch == 0 {
			epoch = a.beginPlaybackEpoch()
		}

		if err := a.playPodcastItem(item, rayID, false, epoch, "resume"); err != nil {
			return err
		}

		if positionMs > 0 {
			if err := a.audio.Seek(positionMs); err != nil {
				return fmt.Errorf(
					"restore podcast position %d ms: %w",
					positionMs,
					err,
				)
			}
		}

		restored := a.state.Get()
		restored.PositionMs = positionMs
		restored.PositionLabel = formatPlaybackPosition(positionMs)
		restored.Status = appstate.PlaybackPlaying
		restored.LastError = ""
		a.state.Replace(restored)

		return nil
	}

	track, ok := a.library.TrackByID(st.CurrentTrackID)
	if !ok {
		return fmt.Errorf(
			"current track not found: %s",
			st.CurrentTrackID,
		)
	}
	if err := validateTrackForPlayback(track); err != nil {
		return err
	}

	if err := a.playTrackSafe(track, true); err != nil {
		return err
	}
	if positionMs > 0 {
		if err := a.audio.Seek(positionMs); err != nil {
			return fmt.Errorf(
				"restore track position %d ms: %w",
				positionMs,
				err,
			)
		}
	}

	restored := a.state.Get()
	restored.PositionMs = positionMs
	restored.PositionLabel = formatPlaybackPosition(positionMs)
	restored.Status = appstate.PlaybackPlaying
	restored.LastError = ""
	a.state.Replace(restored)

	return nil
}

func isPodcastTrackID(id string) bool {
	return podcast.IsItemID(id)
}

func formatPlaybackPosition(positionMs int) string {
	if positionMs < 0 {
		positionMs = 0
	}
	hours := positionMs / 3_600_000
	minutes := (positionMs / 60_000) % 60
	seconds := (positionMs / 1_000) % 60
	if hours > 0 {
		return fmt.Sprintf(
			"%d:%02d:%02d",
			hours,
			minutes,
			seconds,
		)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// Deprecated: kept temporarily for existing generated bindings.
func (a *App) TogglePause() appstate.PlayerState {
	st, _ := a.TogglePlay()
	return st
}

func (a *App) NextTrack() (BootstrapPayload, error) {
	return a.advanceToNextTrack(true)
}

func (a *App) PreviousTrack() BootstrapPayload {
	prevItem, ok := a.rays.Previous()
	if !ok {
		return a.Bootstrap()
	}
	prevTrack, ok := a.library.TrackByID(prevItem.TrackID)
	if !ok {
		return a.Bootstrap()
	}

	current := a.state.Get()
	queue := a.rays.CurrentQueue()
	a.beginTrackLoading(
		prevTrack,
		queue,
		current.RayID,
		current.RaySeedTrackID,
	)

	if err := a.playTrackSafe(prevTrack, false); err != nil {
		return a.Bootstrap()
	}

	st := a.state.Get()
	st.Queue = queue
	st.QueueIndex = queueIndexByTrackID(
		queue,
		prevTrack.ID,
	)
	st.QueueLength = len(queue)
	a.state.Replace(st)

	_ = a.events.MarkPlay(prevTrack)
	a.persistPlaybackSession(true)
	a.pushSnapshot()
	return a.Bootstrap()
}

func (a *App) RemoveFromQueue(trackID string) BootstrapPayload {
	a.rays.Remove(trackID)
	st := a.state.Get()
	st.Queue = a.rays.CurrentQueue()
	a.state.Replace(st)
	a.pushSnapshot()
	return a.Bootstrap()
}

func (a *App) MoveQueueItem(trackID string, newIndex int) (BootstrapPayload, error) {
	ray, err := a.rays.Move(trackID, newIndex)
	if err != nil {
		return BootstrapPayload{}, err
	}

	st := a.state.Get()
	st.Queue = append(
		[]rays.QueueItem(nil),
		ray.Queue...,
	)
	st.QueueLength = len(st.Queue)
	a.state.Replace(st)

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) SetMusicRayContentMode(
	mode string,
) (BootstrapPayload, error) {
	currentRay := a.rays.CurrentRay()
	if currentRay.ID == "" {
		return BootstrapPayload{}, fmt.Errorf(
			"music ray is not built",
		)
	}

	contentMode := rays.NormalizeContentMode(mode)

	seed, ok := a.library.TrackByID(
		currentRay.SeedTrackID,
	)
	if !ok {
		return BootstrapPayload{}, fmt.Errorf(
			"ray seed not found: %s",
			currentRay.SeedTrackID,
		)
	}

	requestCtx, requestSeq := a.beginPlayRequest()
	defer a.finishPlayRequest(requestSeq)

	items, err := a.rec.BuildRayWithModeContext(
		requestCtx,
		seed,
		a.library.AllTracks(),
		"",
		string(contentMode),
	)
	if err != nil {
		return BootstrapPayload{}, err
	}
	if !a.isCurrentPlayRequest(requestSeq) {
		return a.Bootstrap(), nil
	}

	newRay, err := a.rays.ReplaceWithRebuiltRay(
		currentRay,
		contentMode,
		items,
	)
	if err != nil {
		return BootstrapPayload{}, err
	}

	st := a.state.Get()
	st.Queue = append(
		[]rays.QueueItem(nil),
		newRay.Queue...,
	)
	st.QueueLength = len(st.Queue)
	a.state.Replace(st)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) SetMusicRaySortMode(
	mode string,
) (BootstrapPayload, error) {
	ray, err := a.rays.SetSortMode(
		rays.NormalizeSortMode(mode),
	)
	if err != nil {
		return BootstrapPayload{}, err
	}

	st := a.state.Get()
	st.Queue = append(
		[]rays.QueueItem(nil),
		ray.Queue...,
	)
	st.QueueLength = len(st.Queue)
	a.state.Replace(st)

	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) SkipToTrackInQueue(trackID string) (BootstrapPayload, error) {
	requestCtx, requestSeq := a.beginPlayRequest()
	defer a.finishPlayRequest(requestSeq)

	_ = requestCtx

	track, ok := a.library.TrackByID(trackID)
	if !ok {
		return BootstrapPayload{}, fmt.Errorf("track not found: %s", trackID)
	}

	current := a.state.Get()
	previousTrackID := current.CurrentTrackID
	rayID := current.RayID
	seedTrackID := current.RaySeedTrackID

	if !a.rays.JumpToTrack(trackID) {
		return BootstrapPayload{}, fmt.Errorf(
			"track not found in current ray: %s",
			trackID,
		)
	}

	queue := a.rays.CurrentQueue()
	a.logCurrentSkip("jump_in_queue")

	a.beginTrackLoading(
		track,
		queue,
		rayID,
		seedTrackID,
	)

	if err := a.playTrackSafe(track, false); err != nil {
		if previousTrackID != "" {
			_ = a.rays.JumpToTrack(previousTrackID)
		}
		return BootstrapPayload{}, err
	}

	if !a.isCurrentPlayRequest(requestSeq) {
		return a.Bootstrap(), nil
	}

	st := a.state.Get()
	st.Queue = queue
	st.QueueIndex = queueIndexByTrackID(queue, track.ID)
	st.QueueLength = len(queue)
	st.PositionMs = 0
	st.PositionLabel = "0:00"
	a.state.Replace(st)

	a.state.SetRaySeed(a.rays.CurrentRay().SeedTrackID)
	_ = a.events.MarkPlay(track)
	a.persistPlaybackSession(true)
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) Seek(positionMs int) appstate.PlayerState {
	st := a.state.Get()
	appLog.I("seek track=%s positionMs=%d", st.CurrentTrackID, positionMs)

	if positionMs < 0 {
		positionMs = 0
	}

	durationMs := st.DurationMs
	if durationMs <= 0 {
		durationMs = a.audio.GetDurationMs()
	}
	if durationMs > 0 && positionMs > durationMs {
		positionMs = durationMs
	}

	fromPosition := st.PositionMs
	if err := a.audio.Seek(positionMs); err != nil {
		appLog.I("seek failed track=%s positionMs=%d err=%v", st.CurrentTrackID, positionMs, err)
		return st
	}

	actual := a.audio.GetPositionMs()
	if actual < 0 ||
		(positionMs > 0 && actual == 0) {
		actual = positionMs
	}

	st.PositionMs = actual
	st.PositionLabel = fmt.Sprintf("%d:%02d", actual/60000, (actual/1000)%60)
	if durationMs > 0 {
		st.DurationMs = durationMs
		st.DurationLabel = fmt.Sprintf(
			"%d:%02d",
			durationMs/60000,
			(durationMs/1000)%60,
		)
	}
	a.state.Replace(st)

	if isPodcastTrackID(st.CurrentTrackID) {
		_, _ = a.podcasts.UpdateProgress(
			st.CurrentTrackID,
			float64(actual)/1000,
			float64(st.DurationMs)/1000,
		)
		if a.podcastHistory != nil {
			_ = a.podcastHistory.Tick(
				st.CurrentTrackID,
				float64(actual)/1000,
				float64(st.DurationMs)/1000,
				st.Status == appstate.PlaybackPlaying,
			)
		}
	} else {
		_ = a.events.MarkSeek(
			st.CurrentTrackID,
			fromPosition,
			actual,
		)
		a.persistCurrentRayState()
		a.persistPlaybackSession(true)
	}

	a.pushSnapshot()
	appLog.I("seek ok track=%s requestedMs=%d actualMs=%d", st.CurrentTrackID, positionMs, actual)
	return st
}

func (a *App) SetVolumePreview(volume float64) appstate.PlayerState {
	// Preview: меняем gain без записи в БД (дёрганье слайдером).
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	a.audio.SetVolume(volume)
	st := a.state.Get()
	st.Volume = volume
	if volume <= 0 {
		st.Muted = true
	} else {
		st.Muted = false
		st.LastNonZeroVolume = volume
	}
	a.state.ReplaceTransient(st)
	a.pushSnapshot()
	return st
}

func (a *App) SetVolume(volume float64) appstate.PlayerState {
	st := a.state.Get()
	appLog.I("volume set track=%s volume=%.3f", st.CurrentTrackID, volume)

	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}

	st.Volume = volume
	if volume <= 0 {
		st.Muted = true
	} else {
		st.Muted = false
		st.LastNonZeroVolume = volume
	}
	st = appstate.NormalizeVolumeState(st)
	a.state.Replace(st)

	a.audio.SetVolume(appstate.EffectiveVolume(st))
	a.saveVolumeState(st)
	a.pushSnapshot()
	return st
}

// ToggleMute включает/выключает беззвучный режим.
// При отключении mute восстанавливает последнюю ненулевую громкость.
func (a *App) ToggleMute() appstate.PlayerState {
	st := appstate.NormalizeVolumeState(a.state.Get())

	if st.Muted || st.Volume <= 0 {
		st.Volume = appstate.RestoreVolume(st)
		st.LastNonZeroVolume = st.Volume
		st.Muted = false
	} else {
		st.LastNonZeroVolume = st.Volume
		st.Muted = true
	}

	st = appstate.NormalizeVolumeState(st)
	a.state.Replace(st)
	a.audio.SetVolume(appstate.EffectiveVolume(st))
	a.saveVolumeState(st)
	a.pushSnapshot()
	return st
}

// saveVolumeState сохраняет mute и lastNonZeroVolume в meta-хранилище.
func (a *App) saveVolumeState(st appstate.PlayerState) {
	_ = a.store.SetMeta(metaPlayerMuted, strconv.FormatBool(st.Muted))
	_ = a.store.SetMeta(metaPlayerLastNonZeroVol,
		strconv.FormatFloat(st.LastNonZeroVolume, 'f', 4, 64))
}

// loadVolumeState восстанавливает mute и lastNonZeroVolume из meta-хранилища.
func (a *App) loadVolumeState() {
	st := a.state.Get()

	if value, err := a.store.GetMeta(metaPlayerMuted); err == nil {
		st.Muted = value == "1" || strings.EqualFold(value, "true")
	}
	if value, err := a.store.GetMeta(metaPlayerLastNonZeroVol); err == nil {
		if parsed, err2 := strconv.ParseFloat(value, 64); err2 == nil {
			st.LastNonZeroVolume = parsed
		}
	}

	st = appstate.NormalizeVolumeState(st)
	a.state.Replace(st)
	a.audio.SetVolume(appstate.EffectiveVolume(st))
}

func (a *App) getEmoFlowSettings() emoflow.UISettings {
	row, err := a.store.GetAppState()
	if err != nil {
		return emoflow.DefaultSettings()
	}
	return emoflow.NormalizeSettings(emoflow.UISettings{Enabled: row.EmoFlowUIEnabled, Intensity: row.EmoFlowUIIntensity, AnimateDuringTrack: row.EmoFlowUIAnimateTrack, RespectReducedMotion: row.EmoFlowUIRespectReduced})
}

func (a *App) GetCurrentEmoFlow() emoflow.UIState {
	st := a.state.Get()
	if st.CurrentTrackID == "" {
		return emoflow.UIState{}
	}
	if isPodcastTrackID(st.CurrentTrackID) {
		return emoflow.UIState{}
	}
	current, ok := a.library.TrackByID(st.CurrentTrackID)
	if !ok {
		return emoflow.UIState{}
	}
	queue := a.rays.CurrentQueue()
	currentIndex := -1
	for i, item := range queue {
		if item.TrackID == st.CurrentTrackID || item.IsCurrent {
			currentIndex = i
			break
		}
	}
	var previous *library.Track
	var next *library.Track
	if currentIndex > 0 {
		if track, ok := a.library.TrackByID(queue[currentIndex-1].TrackID); ok {
			previous = &track
		}
	}
	if currentIndex >= 0 && currentIndex+1 < len(queue) {
		if track, ok := a.library.TrackByID(queue[currentIndex+1].TrackID); ok {
			next = &track
		}
	}
	return emoflow.BuildState(current, previous, next, queue, a.getEmoFlowSettings())
}

func (a *App) emitEmoFlowUpdate() {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "emoflow:update", a.GetCurrentEmoFlow())
	}
}

func (a *App) pushSnapshot() {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "app:snapshot", a.Bootstrap())
		a.emitPlaybackUpdate()
		a.emitEmoFlowUpdate()
	}
}

func (a *App) emitPlaybackUpdate() {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "playback:update", a.state.Get())
}

func (a *App) handlePlaybackStarted(
	track library.Track,
	backend string,
) {
	st := a.state.Get()

	if st.CurrentTrackID != track.ID {
		appLog.I(
			"ignore stale first sample track=%s current=%s backend=%s",
			track.ID,
			st.CurrentTrackID,
			backend,
		)
		return
	}

	st.Status = appstate.PlaybackPlaying
	st.LastError = ""
	a.state.Replace(st)

	_ = a.events.MarkPlaybackStarted(track)
	a.emitPlaybackUpdate()

	appLog.I(
		"playback first sample track=%s backend=%s",
		track.ID,
		backend,
	)

	a.prepareNextQueueTrack()
}

func (a *App) prepareNextQueueTrack() {
	st := a.state.Get()
	queue := st.Queue
	if len(queue) == 0 {
		queue = a.rays.CurrentQueue()
	}

	index := queueIndexByTrackID(
		queue,
		st.CurrentTrackID,
	)
	if index < 0 || index+1 >= len(queue) {
		return
	}

	nextID := queue[index+1].TrackID
	next, ok := a.library.TrackByID(nextID)
	if !ok {
		return
	}

	a.audio.Prepare(next)
}

func (a *App) persistPlaybackSession(force bool) {
	a.sessionMu.Lock()
	if !force && time.Since(a.lastSessionPersist) < 5*time.Second {
		a.sessionMu.Unlock()
		return
	}
	a.lastSessionPersist = time.Now()
	a.sessionMu.Unlock()

	st := a.state.Get()
	if st.CurrentTrackID == "" {
		return
	}

	if isPodcastTrackID(st.CurrentTrackID) {
		positionMs := a.audio.GetPositionMs()
		if positionMs < 0 {
			positionMs = st.PositionMs
		}

		now := time.Now()
		if !force &&
			now.Sub(a.lastPodcastProgressAt) < 5*time.Second &&
			absInt(positionMs-a.lastPodcastPositionMs) < 5000 {
			return
		}

		_, _ = a.podcasts.UpdateProgress(
			st.CurrentTrackID,
			float64(positionMs)/1000,
			float64(st.DurationMs)/1000,
		)
		if a.podcastHistory != nil {
			_ = a.podcastHistory.Tick(
				st.CurrentTrackID,
				float64(positionMs)/1000,
				float64(st.DurationMs)/1000,
				st.Status == appstate.PlaybackPlaying,
			)
		}

		a.lastPodcastProgressAt = now
		a.lastPodcastPositionMs = positionMs
		return
	}

	queueJSON, err := a.rays.FrozenQueueJSON()
	if err != nil && len(st.Queue) > 0 {
		data, marshalErr := json.Marshal(rays.PlaybackQueue{
			ID:             st.QueueID,
			Kind:           "ray",
			Items:          st.Queue,
			Index:          st.QueueIndex,
			RayID:          st.RayID,
			RaySeedTrackID: st.RaySeedTrackID,
			UpdatedAt:      time.Now().UnixMilli(),
		})
		if marshalErr == nil {
			queueJSON = string(data)
			err = nil
		}
	}
	if err != nil {
		appLog.W("persist playback session queue marshal failed: %v", err)
	}

	position := st.PositionMs
	if st.Status == appstate.PlaybackPlaying {
		if current := a.audio.GetPositionMs(); current >= 0 {
			position = current
		}
	}

	err = a.store.SavePlaybackSession(db.PlaybackSessionRow{
		ID:             "last",
		Status:         string(st.Status),
		CurrentTrackID: st.CurrentTrackID,
		CurrentPath:    st.CurrentPath,
		PositionMs:     position,
		DurationMs:     st.DurationMs,
		QueueID:        st.QueueID,
		QueueIndex:     st.QueueIndex,
		QueueJSON:      queueJSON,
		RayID:          st.RayID,
		RaySeedTrackID: st.RaySeedTrackID,
		UpdatedAt:      time.Now().UnixMilli(),
		LastError:      st.LastError,
	})
	if err != nil {
		appLog.W("persist playback session failed: %v", err)
	}
}

func (a *App) restorePlaybackSession() {
	snap, err := a.store.LoadLastPlaybackSession()
	if err != nil {
		appLog.W("restore playback session failed: %v", err)
		a.state.Replace(appstate.PlayerState{
			Status:     appstate.PlaybackStopped,
			QueueIndex: -1,
			Volume:     0.58,
		})
		return
	}

	if snap.CurrentTrackID == "" || strings.TrimSpace(snap.QueueJSON) == "" {
		_ = a.state.Load(a.library, a.rays)
		st := a.state.Get()
		if st.CurrentTrackID != "" {
			st.Status = appstate.PlaybackPaused
		}
		a.state.Replace(st)
		return
	}

	if err := a.rays.RestoreFrozenQueue(
		snap.QueueJSON,
		snap.CurrentTrackID,
	); err != nil {
		appLog.W("restore playback frozen queue failed: %v", err)
		a.state.Replace(appstate.PlayerState{
			Status:     appstate.PlaybackError,
			QueueIndex: -1,
			LastError:  err.Error(),
			Volume:     0.58,
		})
		return
	}

	track, ok := a.library.TrackByID(snap.CurrentTrackID)
	if !ok {
		err := fmt.Errorf("restored track not found: %s", snap.CurrentTrackID)
		appLog.W("%v", err)
		a.state.Replace(appstate.PlayerState{
			Status:     appstate.PlaybackError,
			QueueIndex: -1,
			LastError:  err.Error(),
			Volume:     0.58,
		})
		return
	}
	if err := validateTrackForPlayback(track); err != nil {
		appLog.W(
			"restore failed missing file path=%q err=%v",
			track.Path,
			err,
		)
		a.state.Replace(appstate.PlayerState{
			Status:         appstate.PlaybackError,
			CurrentTrackID: track.ID,
			CurrentPath:    track.Path,
			CurrentTitle:   track.Title,
			CurrentArtist:  track.Artist,
			LastError:      err.Error(),
			QueueIndex:     -1,
			Volume:         0.58,
		})
		return
	}

	queue := a.rays.CurrentQueue()
	a.state.SetCurrent(track, snap.RayID, queue, snap.PositionMs)
	st := a.state.Get()
	st.Status = appstate.PlaybackPaused
	st.RayID = snap.RayID
	st.CurrentRayID = snap.RayID
	st.RaySeedTrackID = snap.RaySeedTrackID
	st.QueueID = snap.QueueID
	st.QueueIndex = snap.QueueIndex
	st.QueueLength = len(queue)
	st.Playing = false
	a.state.Replace(st)

	appLog.I(
		"restore session queueId=%s index=%d currentTrack=%s seed=%s",
		st.QueueID,
		st.QueueIndex,
		st.CurrentTrackID,
		st.RaySeedTrackID,
	)
}

func (a *App) setPlaybackError(
	st appstate.PlayerState,
	err error,
) (appstate.PlayerState, error) {
	st.Status = appstate.PlaybackError
	st.LastError = err.Error()
	a.state.Replace(st)
	a.persistPlaybackSession(true)
	a.pushSnapshot()
	return a.state.Get(), err
}

func (a *App) ScheduleRecluster() {
	a.lifecycleMu.Lock()
	stopping := a.shuttingDown
	a.lifecycleMu.Unlock()
	if stopping {
		return
	}
	indexing := library.IndexingState{}
	if a.library != nil {
		indexing = a.library.IndexingState()
	}
	if shouldDeferRecluster(a.isReindexRunning(), indexing) {
		return
	}

	a.reclusterMu.Lock()
	defer a.reclusterMu.Unlock()
	if a.reclusterTimer != nil {
		a.reclusterTimer.Stop()
	}
	a.reclusterTimer = time.AfterFunc(5*time.Second, func() {
		a.launchBackground(func(ctx context.Context) {
			if ctx.Err() != nil {
				return
			}
			a.RunReclusterSingleflight()
		})
	})
}

func shouldDeferRecluster(reindexRunning bool, indexing library.IndexingState) bool {
	return reindexRunning || indexing.IsIndexing
}

func (a *App) isReindexRunning() bool {
	a.reindexMu.Lock()
	running := a.reindexRunning
	a.reindexMu.Unlock()
	return running
}

func (a *App) RunReclusterSingleflight() {
	a.reclusterMu.Lock()
	if a.reclusterRunning {
		a.reclusterPending = true
		a.reclusterMu.Unlock()
		return
	}
	a.reclusterRunning = true
	a.reclusterMu.Unlock()
	a.reclusterNow()
	a.reclusterMu.Lock()
	rerun := a.reclusterPending
	a.reclusterPending = false
	a.reclusterRunning = false
	a.reclusterMu.Unlock()
	if rerun {
		a.ScheduleRecluster()
	}
}

func (a *App) reclusterNow() {
	tracks := a.library.AllTracks()
	appLog.I("recluster start tracks=%d", len(tracks))
	a.rec.Recluster(tracks)
	for _, t := range tracks {
		existing, ok := a.library.TrackByID(t.ID)
		if ok && existing.ClusterID == t.ClusterID {
			continue
		}
		appLog.I("recluster update track=%s cluster=%d", t.ID, t.ClusterID)
		_ = a.library.UpdateClusterID(t.ID, t.ClusterID)
	}
	appLog.I("recluster done tracks=%d", len(tracks))
}

func (a *App) advanceToNextTrack(manual bool) (BootstrapPayload, error) {
	return a.advanceToNextTrackWithFlags(manual, false)
}

func (a *App) advanceToNextTrackWithFlags(manual bool, skipMarkPlay bool) (BootstrapPayload, error) {
	st := a.state.Get()
	prevTrackID := st.CurrentTrackID
	nextItem, ok := a.rays.Next()
	if !ok {
		return a.pauseAtEnd(), nil
	}
	nextTrack, ok := a.library.TrackByID(nextItem.TrackID)
	if !ok {
		return a.pauseAtEnd(), nil
	}
	if manual {
		a.logCurrentSkip("manual_next")
	}

	current := a.state.Get()
	queue := a.rays.CurrentQueue()
	a.beginTrackLoading(
		nextTrack,
		queue,
		current.RayID,
		current.RaySeedTrackID,
	)

	if err := a.playTrackSafe(nextTrack, false); err != nil {
		return a.pauseAtEnd(), err
	}

	st = a.state.Get()
	st.Queue = queue
	st.QueueIndex = queueIndexByTrackID(queue, nextTrack.ID)
	st.QueueLength = len(queue)
	a.state.Replace(st)

	a.persistPlaybackSession(true)
	if !skipMarkPlay {
		_ = a.events.MarkPlay(nextTrack)
	}
	if prevTrackID != "" && prevTrackID != nextTrack.ID {
		_ = a.events.RecordTransition(
			prevTrackID,
			nextTrack.ID,
			nextItem.Strategy,
			nextItem.Bucket,
			nextItem.Insight.Transition,
			nextItem.Insight.EnergyDirection,
			nextItem.Insight.Bridge,
			nextItem.Insight.Discovery,
		)
	}
	if manual {
		a.persistCurrentRayState()
	}
	a.pushSnapshot()
	return a.Bootstrap(), nil
}

func (a *App) pauseAtEnd() BootstrapPayload {
	st := a.state.Get()
	if st.CurrentTrackID == "" {
		return a.Bootstrap()
	}
	track, ok := a.library.TrackByID(st.CurrentTrackID)
	if ok {
		_ = a.events.MarkTransitionCompleted(track)
	}
	st.Playing = false
	if st.DurationMs > 0 {
		st.PositionMs = st.DurationMs
		st.PositionLabel = fmt.Sprintf("%d:%02d", st.DurationMs/60000, (st.DurationMs/1000)%60)
	}
	st.CurrentSub = "трек завершён"
	st.Queue = a.rays.CurrentQueue()
	a.state.Replace(st)
	_ = a.rays.UpdateState(st.CurrentRayID, st.CurrentTrackID, st.PositionMs, st.CurrentSub)
	a.pushSnapshot()
	return a.Bootstrap()
}

func (a *App) handlePlaybackEnded(track library.Track, reason audio.PlaybackEndReason) {
	st := a.state.Get()
	if st.CurrentTrackID != track.ID {
		return
	}

	if isPodcastTrackID(track.ID) {
		positionMs := a.audio.GetPositionMs()
		if reason == audio.PlaybackEndNatural {
			// После EOF декодер может вернуть position=0.
			// Для естественного завершения берём известную длительность.
			if track.DurationMs > 0 {
				positionMs = track.DurationMs
			} else if st.DurationMs > 0 {
				positionMs = st.DurationMs
			}
		}
		_, _ = a.podcasts.UpdateProgress(
			track.ID,
			float64(positionMs)/1000,
			float64(track.DurationMs)/1000,
		)
		if a.podcastHistory != nil {
			endReason := "playback_stopped"
			if reason == audio.PlaybackEndNatural {
				endReason = "natural_end"
			}
			_ = a.podcastHistory.Finish(
				track.ID,
				float64(positionMs)/1000,
				float64(track.DurationMs)/1000,
				endReason,
			)
		}

		if reason == audio.PlaybackEndNatural {
			if next, ok := a.podcasts.NextRayItem(); ok {
				epoch := a.beginPlaybackEpoch()
				_ = a.playPodcastItem(
					next,
					a.podcasts.CurrentRay().ID,
					true,
					epoch,
					"ray_auto",
				)
			}
		}
		a.pushSnapshot()
		return
	}

	if reason != audio.PlaybackEndNatural {
		appLog.I("technical playback end track=%s reason=%s", track.ID, reason)
		position := a.audio.GetPositionMs()
		if position <= 0 {
			position = st.PositionMs
		}
		_ = a.events.MarkTechnicalSkip(track, string(reason), position)
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "emoflow:technical_skip", map[string]any{
				"trackId": track.ID,
				"title":   track.Title,
				"reason":  string(reason),
			})
			wruntime.EventsEmit(a.ctx, "playback:failed", PlaybackFailure{
				TrackID: track.ID,
				Path:    track.Path,
				Title:   track.Title,
				Kind:    PlaybackDecodeError,
				Error:   string(reason),
				At:      time.Now().UnixMilli(),
			})
		}
		a.advanceAfterTechnicalSkip(st)
		return
	}
	a.rewardCurrentQueueItem("play_complete", 1.0)
	if _, err := a.advanceToNextTrack(false); err == nil {
		return
	}
	a.advanceAfterNaturalEnd(st)
}

func (a *App) advanceAfterTechnicalSkip(st appstate.PlayerState) {
	if _, err := a.advanceToNextTrackWithFlags(false, true); err == nil {
		return
	}
	settings := a.GetSettings()
	if settings.ExtendRay && a.rays.Remaining() <= 2 {
		a.tryExtendCurrentRay()
		if _, err := a.advanceToNextTrackWithFlags(false, true); err == nil {
			return
		}
	}
	if settings.RepeatRay {
		if item, ok := a.rays.RepeatToStart(); ok {
			if repeatTrack, found := a.library.TrackByID(item.TrackID); found && a.playTrackSafe(repeatTrack, false) == nil {
				a.state.SetCurrent(repeatTrack, st.CurrentRayID, a.rays.CurrentQueue(), 0)
				_ = a.events.MarkPlay(repeatTrack)
				a.pushSnapshot()
				return
			}
		}
	}
	_ = a.pauseAtEnd()
}

func (a *App) advanceAfterNaturalEnd(st appstate.PlayerState) {
	settings := a.GetSettings()
	if settings.ExtendRay && a.rays.Remaining() <= 2 {
		a.tryExtendCurrentRay()
		if _, err := a.advanceToNextTrack(false); err == nil {
			return
		}
	}
	if settings.RepeatRay {
		if item, ok := a.rays.RepeatToStart(); ok {
			if repeatTrack, found := a.library.TrackByID(item.TrackID); found && a.playTrackSafe(repeatTrack, false) == nil {
				a.state.SetCurrent(repeatTrack, st.CurrentRayID, a.rays.CurrentQueue(), 0)
				_ = a.events.MarkPlay(repeatTrack)
				a.pushSnapshot()
				return
			}
		}
	}
	_ = a.pauseAtEnd()
}

func (a *App) usableTracksForRay(seed library.Track) []library.Track {
	tracks := a.library.UsableTracks()
	for _, track := range tracks {
		if track.ID == seed.ID {
			return tracks
		}
	}
	return append([]library.Track{seed}, tracks...)
}

func (a *App) findTrackByPath(path string) *library.Track {
	for _, track := range a.library.AllTracks() {
		if strings.EqualFold(track.Path, path) {
			copy := track
			return &copy
		}
	}
	return nil
}

func isAudioFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".flac", ".wav", ".m4a", ".aac", ".ogg", ".opus", ".wma", ".aiff", ".aif":
		return true
	default:
		return false
	}
}

func expandAudioFiles(paths []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, root := range paths {
		if strings.TrimSpace(root) == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if isAudioFile(root) && !seen[root] {
				seen[root] = true
				out = append(out, root)
			}
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				name := strings.ToLower(d.Name())
				if name == ".git" || name == "node_modules" || name == "__macosx" {
					return filepath.SkipDir
				}
				return nil
			}
			if isAudioFile(path) && !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out, nil
}

func validateTrackForPlayback(track library.Track) error {
	if track.SourceType == "yt_dlp" &&
		track.DownloadStatus != "ready" {
		return fmt.Errorf(
			"трек ещё не готов: статус загрузки %s",
			track.DownloadStatus,
		)
	}
	if strings.TrimSpace(track.Path) == "" {
		return errors.New("track path is empty")
	}
	if track.ImportStatus != string(library.ImportReady) && track.ImportStatus != "" {
		return fmt.Errorf("track not ready: %s", track.ImportStatus)
	}
	if track.FileMissing {
		return fmt.Errorf("file marked missing")
	}
	info, err := os.Stat(track.Path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is directory")
	}
	if info.Size() <= 0 {
		return fmt.Errorf("file is empty")
	}
	return nil
}

func classifyPlaybackError(err error) PlaybackErrorKind {
	if err == nil {
		return PlaybackUnknown
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission"):
		return PlaybackPermissionDenied
	case strings.Contains(msg, "no such file") || strings.Contains(msg, "not exist"):
		return PlaybackFileMissing
	case strings.Contains(msg, "unsupported"):
		return PlaybackUnsupportedCodec
	case strings.Contains(msg, "timeout"):
		return PlaybackTimeout
	case strings.Contains(msg, "device") || strings.Contains(msg, "speaker"):
		return PlaybackDeviceError
	case strings.Contains(msg, "decode") || strings.Contains(msg, "ffmpeg") || strings.Contains(msg, "invalid"):
		return PlaybackDecodeError
	default:
		return PlaybackUnknown
	}
}

func (a *App) playTrackSafe(track library.Track, resume bool) error {
	if err := validateTrackForPlayback(track); err != nil {
		a.handlePlaybackFailure(track, classifyPlaybackError(err), err)
		return err
	}
	var err error
	if resume {
		err = a.audio.Play(track)
	} else {
		err = a.audio.PlayFresh(track)
	}
	if err != nil {
		a.handlePlaybackFailure(track, classifyPlaybackError(err), err)
		return err
	}
	_ = a.store.MarkPlaybackSucceeded(track.ID)
	if !resume {
		_ = a.events.MarkTransitionStarted(track, 0)
	}
	return nil
}

func (a *App) handlePlaybackFailure(track library.Track, kind PlaybackErrorKind, err error) {
	failure := PlaybackFailure{TrackID: track.ID, Path: track.Path, Title: track.Title, Kind: kind, Error: err.Error(), At: time.Now().UnixMilli()}
	appLog.E("⏭️ skipping track id=%s title=%q kind=%s path=%q err=%v", track.ID, track.Title, kind, track.Path, err)
	_ = a.store.MarkPlaybackFailed(track.ID, string(kind), err.Error())
	if kind == PlaybackFileMissing {
		_ = a.store.MarkTrackMissing(track.ID)
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "playback:failed", failure)
	}
}

func (a *App) tryExtendCurrentRay() {
	st := a.state.Get()
	seed, ok := a.library.TrackByID(st.CurrentTrackID)
	if !ok {
		return
	}
	items := a.rec.ExtendRay(recommend.ExtendRayRequest{Seed: seed, ExistingTrackIDs: a.rays.TrackIDs(), Mode: "", Count: 6, Library: a.usableTracksForRay(seed)})
	if len(items) == 0 {
		return
	}
	a.rays.Append(items)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "queue:updated", a.rays.CurrentQueue())
	}
}

func (a *App) persistCurrentRayState() {
	st := a.state.Get()

	rayID := strings.TrimSpace(st.CurrentRayID)
	if rayID == "" {
		appLog.D("skip persist ray state without ray track=%s position=%d", st.CurrentTrackID, st.PositionMs)
		return
	}

	appLog.I("persist ray state ray=%s track=%s statePos=%d", rayID, st.CurrentTrackID, st.PositionMs)
	if st.CurrentTrackID == "" {
		return
	}
	position := a.audio.GetPositionMs()
	if position <= 0 {
		position = st.PositionMs
	}
	resumeLabel := fmt.Sprintf("продолжить с %d:%02d", position/60000, (position/1000)%60)
	_ = a.rays.UpdateState(st.CurrentRayID, st.CurrentTrackID, position, resumeLabel)
	a.state.SetPlaybackPosition(position, true)
}

func skipRewardForSource(source string, posMs int, durationMs int) float64 {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "play_track_switch", "manual_switch", "user_selected_track", "user_click":
		if posMs < 3000 {
			return 0
		}
		return -0.10
	case "jump_in_queue":
		if durationMs > 0 {
			ratio := float64(posMs) / float64(durationMs)
			if ratio >= 0.35 {
				return -0.08
			}
			if ratio >= 0.20 {
				return -0.15
			}
		}
		if posMs > 60000 {
			return -0.10
		}
		return -0.25
	case "technical_skip", "decode_error", "playback_error":
		return 0
	default:
		if durationMs > 0 {
			ratio := float64(posMs) / float64(durationMs)
			if ratio >= 0.50 {
				return -0.10
			}
			if ratio >= 0.25 {
				return -0.25
			}
		}
		if posMs < 10000 {
			return -0.75
		}
		return -0.35
	}
}

func (a *App) logCurrentSkip(source string) {
	st := a.state.Get()
	if !st.Playing || st.CurrentTrackID == "" {
		return
	}
	position := a.audio.GetPositionMs()
	if position <= 0 {
		position = st.PositionMs
	}
	track, ok := a.library.TrackByID(st.CurrentTrackID)
	if !ok {
		return
	}
	appLog.I("skip track=%s source=%s pos=%d duration=%d", st.CurrentTrackID, source, position, track.DurationMs)
	reward := skipRewardForSource(source, position, track.DurationMs)
	if reward == 0 {
		appLog.I("skip reward ignored source=%s track=%s pos=%d duration=%d", source, st.CurrentTrackID, position, track.DurationMs)
		_ = a.events.MarkSkip(track, position, source)
		return
	}
	if position < 30000 || (track.DurationMs > 0 && float64(position)/float64(track.DurationMs) < 0.30) {
		_ = a.events.MarkTransitionSkippedEarly(track, position)
	}
	eventType := a.events.MarkSkip(track, position, source)
	a.rewardCurrentQueueItem(eventType, reward)
}

func (a *App) playbackTicker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastPersist := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		st := a.state.Get()
		if !st.Playing || st.CurrentTrackID == "" {
			continue
		}
		a.persistPlaybackSession(false)
		position := a.audio.GetPositionMs()
		if position <= 0 {
			continue
		}
		a.trackPlaybackMilestones(st.CurrentTrackID, position, st.DurationMs)
		persist := time.Since(lastPersist) >= 5*time.Second
		a.state.SetPlaybackPosition(position, persist)
		if persist {
			resumeLabel := fmt.Sprintf("продолжить с %d:%02d", position/60000, (position/1000)%60)
			_ = a.rays.UpdateState(st.CurrentRayID, st.CurrentTrackID, position, resumeLabel)
			lastPersist = time.Now()
		}
		a.pushSnapshot()
	}
}

func (a *App) rewardCurrentQueueItem(source string, reward float64) {
	item, ok := a.rays.CurrentItem()
	if !ok {
		return
	}
	bucket := item.Bucket
	if item.Insight.Bucket != "" {
		bucket = item.Insight.Bucket
	}
	strategy := item.Strategy
	if item.Insight.Strategy != "" {
		strategy = item.Insight.Strategy
	}
	if strategy != "" && strategy != "seed" {
		_ = a.events.RewardStrategy(strategy, reward)
	}
	if bucket != "" && bucket != "seed" {
		_ = a.events.RewardStrategy("bucket:"+bucket, reward*0.5)
	}
	if key := a.currentTransitionRewardKey(); key != "" {
		_ = a.events.RewardStrategy(key, reward)
	}
	appLog.I("reward source=%s track=%s strategy=%s bucket=%s reward=%.2f", source, item.TrackID, strategy, bucket, reward)
}

func (a *App) currentTransitionRewardKey() string {
	queue := a.rays.CurrentQueue()
	if len(queue) < 2 {
		return ""
	}
	idx := -1
	for i, item := range queue {
		if item.IsCurrent {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return ""
	}
	prevTrack, okPrev := a.library.TrackByID(queue[idx-1].TrackID)
	currTrack, okCurr := a.library.TrackByID(queue[idx].TrackID)
	if !okPrev || !okCurr {
		return ""
	}
	return recommend.TransitionRewardKey(prevTrack, currTrack)
}

func (a *App) trackPlaybackMilestones(trackID string, positionMs, durationMs int) {
	if trackID == "" || durationMs <= 0 {
		return
	}
	if a.milestones.TrackID != trackID {
		a.milestones = playbackMilestones{TrackID: trackID}
	}
	track, ok := a.library.TrackByID(trackID)
	if !ok {
		return
	}
	progress := float64(positionMs) / float64(durationMs)
	if progress >= 0.30 && !a.milestones.Marked30 {
		a.milestones.Marked30 = true
		_ = a.events.MarkProgress(track, "play_30s", positionMs)
		_ = a.events.MarkTransitionSurvived30(track, positionMs)
		a.rewardCurrentQueueItem("play_30s", 0.10)
	}
	if positionMs >= 60000 && !a.milestones.Marked60 {
		a.milestones.Marked60 = true
		_ = a.events.MarkTransitionSurvived60(track, positionMs)
		a.rewardCurrentQueueItem("transition_survived_60s", 0.25)
	}
	if progress >= 0.50 && !a.milestones.Marked50 {
		a.milestones.Marked50 = true
		_ = a.events.MarkProgress(track, "play_half", positionMs)
		a.rewardCurrentQueueItem("play_half", 0.35)
	}
	if progress >= 0.80 && !a.milestones.Marked80 {
		a.milestones.Marked80 = true
		_ = a.events.MarkProgress(track, "play_80", positionMs)
		a.rewardCurrentQueueItem("play_80", 0.7)
	}
}
