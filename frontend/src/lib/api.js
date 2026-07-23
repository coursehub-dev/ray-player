import {
	AddExternalLink,
	AddFiles,
	AddFolder,
	AddPodcastFiles,
	AddPodcastFolder,
	Bootstrap,
	CancelExternalDownload,
	DeleteExternalItem,
	GetExternalMediaSettings,
	GetPlaybackState,
	NextPodcast,
	OpenExternalSource,
	OpenPodcastRayHistory,
	PlayPodcast,
	PlayPodcastRayItem,
	PreviousPodcast,
	MovePodcastRayItem,
	RemovePodcastRayItem,
	RetryExternalDownload,
	SaveExternalMediaSettings,
	SearchPodcasts,
	SetPodcastRayContentMode,
	SetPodcastRaySortMode,
	TestYtDlp,
	UpdatePodcastProgress,
	MoveQueueItem,
	NextTrack,
	PlayTrack,
	PlayTrackWithMode,
	SetMusicRayContentMode,
	SetMusicRaySortMode,
	PreviousTrack,
	RemoveFromQueue,
	ResumeRay,
	SearchTracks,
	Seek,
	SetVolume,
	ToggleMute,
	SetNormalizePodcastVolume,
	Snapshot,
	TogglePlay,
	TogglePause,
	ChooseDirectory,
	ChooseFile,
	GetSettings,
	SaveSettings,
	SkipToTrackInQueue,
	TestEssentia,
	TestMiniLM,
	TestONNXRuntime,
	DebugReindexLibrary,
	TestFFmpeg,
} from "../../wailsjs/go/main/App";
import { bootstrapFallback } from "./mockWails";

const hasWails = () => Boolean(globalThis?.window?.go?.main?.App);
const appCall = (name, ...args) =>
	globalThis?.window?.go?.main?.App?.[name]?.(...args);

const stoppedPlayback = {
	status: "stopped",
	currentTrackId: "",
	currentPath: "",
	positionMs: 0,
	durationMs: 0,
	queueId: "",
	queueIndex: -1,
	queueLength: 0,
	rayId: "",
	raySeedTrackId: "",
	updatedAt: 0,
	lastError: "",
};

export const api = {
	bootstrap: () => (hasWails() ? Bootstrap() : bootstrapFallback()),
	snapshot: () => (hasWails() ? Snapshot() : bootstrapFallback()),
	getPlaybackState: () =>
		hasWails() ? GetPlaybackState() : Promise.resolve(stoppedPlayback),
	addFolder: () => (hasWails() ? AddFolder() : bootstrapFallback()),
	addFiles: () => (hasWails() ? AddFiles() : bootstrapFallback()),
	addPodcastFolder: () =>
		hasWails() ? AddPodcastFolder() : bootstrapFallback(),
	addPodcastFiles: () => (hasWails() ? AddPodcastFiles() : bootstrapFallback()),

	addExternalLink: (url, libraryType) =>
		hasWails()
			? AddExternalLink(url, libraryType)
			: Promise.reject(new Error("Wails runtime unavailable")),
	cancelExternalDownload: (jobId) =>
		hasWails() ? CancelExternalDownload(jobId) : Promise.resolve(),
	retryExternalDownload: (jobId) =>
		hasWails() ? RetryExternalDownload(jobId) : Promise.resolve(),
	deleteExternalItem: (itemId, libraryType, deleteFile = false) =>
		hasWails()
			? DeleteExternalItem(itemId, libraryType, deleteFile)
			: Promise.resolve(),
	openExternalSource: (url) =>
		hasWails() ? OpenExternalSource(url) : Promise.resolve(),
	getExternalMediaSettings: () =>
		hasWails()
			? GetExternalMediaSettings()
			: Promise.resolve({
					ytDlpPath: "yt-dlp",
					ffmpegPath: "",
					ytDlpDownloadDir: "",
				}),
	saveExternalMediaSettings: (settings) =>
		hasWails() ? SaveExternalMediaSettings(settings) : Promise.resolve(),
	testYtDlp: (path) =>
		hasWails()
			? TestYtDlp(path)
			: Promise.resolve({ ok: false, error: "Wails runtime unavailable" }),

	searchPodcasts: (query) =>
		hasWails() ? SearchPodcasts(query) : Promise.resolve([]),
	updatePodcastProgress: (itemId, position, duration) =>
		hasWails()
			? UpdatePodcastProgress(itemId, position, duration)
			: Promise.resolve({ id: itemId, lastPosition: position, duration }),
	playPodcast: (itemId) =>
		hasWails() ? PlayPodcast(itemId) : bootstrapFallback(),
	playPodcastRayItem: (itemId) =>
		hasWails() ? PlayPodcastRayItem(itemId) : bootstrapFallback(),
	nextPodcast: () => (hasWails() ? NextPodcast() : bootstrapFallback()),
	previousPodcast: () => (hasWails() ? PreviousPodcast() : bootstrapFallback()),
	openPodcastRayHistory: (rayId) =>
		hasWails() ? OpenPodcastRayHistory(rayId) : bootstrapFallback(),
	setPodcastRayContentMode: (mode) =>
		hasWails() ? SetPodcastRayContentMode(mode) : bootstrapFallback(),
	setPodcastRaySortMode: (mode) =>
		hasWails() ? SetPodcastRaySortMode(mode) : bootstrapFallback(),
	movePodcastRayItem: (from, to) =>
		hasWails() ? MovePodcastRayItem(from, to) : bootstrapFallback(),
	removePodcastRayItem: (itemId) =>
		hasWails() ? RemovePodcastRayItem(itemId) : bootstrapFallback(),
	importPaths: (paths) =>
		hasWails()
			? appCall("ImportPaths", paths)
			: Promise.resolve({
					inputCount: paths.length,
					audioFound: 0,
					alreadyPresent: 0,
					added: 0,
					skipped: 0,
					errors: [],
				}),
	importPodcastPaths: (paths) =>
		hasWails()
			? appCall("ImportPodcastPaths", paths)
			: Promise.resolve({
					inputCount: paths.length,
					audioFound: 0,
					addedOrUpdated: 0,
					skipped: 0,
					errors: [],
				}),
	searchTracks: (query) => (hasWails() ? SearchTracks(query) : []),
	playTrack: (trackId) =>
		hasWails() ? PlayTrack(trackId) : bootstrapFallback(),
	playTrackWithMode: (trackId, mode) =>
		hasWails() ? PlayTrackWithMode(trackId, mode) : bootstrapFallback(),
	setMusicRayContentMode: (mode) =>
		hasWails() ? SetMusicRayContentMode(mode) : bootstrapFallback(),
	setMusicRaySortMode: (mode) =>
		hasWails() ? SetMusicRaySortMode(mode) : bootstrapFallback(),
	resumeRay: (rayId) => (hasWails() ? ResumeRay(rayId) : bootstrapFallback()),
	togglePlay: () =>
		hasWails() ? TogglePlay() : Promise.resolve(stoppedPlayback),
	togglePause: () => (hasWails() ? TogglePause() : stoppedPlayback),
	nextTrack: () => (hasWails() ? NextTrack() : bootstrapFallback()),
	previousTrack: () => (hasWails() ? PreviousTrack() : bootstrapFallback()),
	removeFromQueue: (trackId) =>
		hasWails() ? RemoveFromQueue(trackId) : bootstrapFallback(),
	moveQueueItem: (trackId, newIndex) =>
		hasWails() ? MoveQueueItem(trackId, newIndex) : bootstrapFallback(),
	seek: (positionMs) => (hasWails() ? Seek(positionMs) : { positionMs }),
	setVolumePreview: (volume) =>
		hasWails()
			? appCall("SetVolumePreview", volume) || Promise.resolve({ volume })
			: Promise.resolve({ volume }),
	setVolume: (volume) => (hasWails() ? SetVolume(volume) : { volume }),
	toggleMute: () => (hasWails() ? ToggleMute() : Promise.resolve(null)),
	setNormalizePodcastVolume: (enabled) =>
		hasWails() ? SetNormalizePodcastVolume(enabled) : Promise.resolve(null),
	chooseDirectory: (title) =>
		hasWails() ? ChooseDirectory(title) : Promise.resolve(""),
	chooseFile: (title, pattern) =>
		hasWails() ? ChooseFile(title, pattern) : Promise.resolve(""),
	getSettings: () =>
		hasWails()
			? GetSettings()
			: Promise.resolve({
					onnxRuntimePath: "",
					miniLMModelDir: "",
					essentiaModelDir: "",
					ffmpegPath: "ffmpeg",
					storagePath: "",
					repeatRay: true,
					extendRay: false,
					emoFlowUi: {
						enabled: true,
						intensity: 1,
						animateDuringTrack: true,
						respectReducedMotion: true,
					},
				}),
	saveSettings: (payload) =>
		hasWails() ? SaveSettings(payload) : bootstrapFallback(),
	skipToTrackInQueue: (trackId) =>
		hasWails() ? SkipToTrackInQueue(trackId) : bootstrapFallback(),
	testONNXRuntime: (payload) =>
		hasWails()
			? TestONNXRuntime(payload)
			: Promise.resolve({
					ok: false,
					runtimePath: "",
					message: "Wails unavailable",
					latencyMs: 0,
				}),
	testMiniLM: (payload) =>
		hasWails()
			? TestMiniLM(payload)
			: Promise.resolve({
					ok: false,
					runtimePath: "",
					modelDir: "",
					modelPath: "",
					tokenizerPath: "",
					message: "Wails unavailable",
					latencyMs: 0,
					embeddingDim: 0,
				}),
	testFFmpeg: (path) =>
		hasWails() ? TestFFmpeg(path) : Promise.resolve("Wails unavailable"),
	testEssentia: (payload) =>
		hasWails()
			? TestEssentia(payload)
			: Promise.resolve({
					ok: false,
					runtimePath: "",
					modelDir: "",
					base: {
						name: "base",
						modelPath: "",
						metaPath: "",
						present: false,
						loaded: false,
						message: "Wails unavailable",
					},
					genre: {
						name: "genre",
						modelPath: "",
						metaPath: "",
						present: false,
						loaded: false,
						message: "Wails unavailable",
					},
					heads: [],
					message: "Wails unavailable",
					latencyMs: 0,
				}),
	debugReindexLibrary: () =>
		hasWails()
			? DebugReindexLibrary()
			: Promise.resolve({
					started: false,
					busy: false,
					total: 0,
					message: "Wails unavailable",
				}),
	rayAudit: (trackId, mode, limit = 20) =>
		hasWails()
			? appCall("RayAudit", trackId, mode, limit)
			: Promise.resolve({ seedTrackId: trackId, mode, rows: [] }),
};
