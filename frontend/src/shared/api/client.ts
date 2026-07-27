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
} from "../../../wailsjs/go/main/App";
import { initialPlaybackState } from "../../entities/playback/model";
import { bootstrapFallback } from "./mockWails";

type WailsApp = Record<string, ((...args: unknown[]) => unknown) | undefined>;

const wailsWindow = () => (globalThis as { window?: { go?: { main?: { App?: WailsApp } } } }).window;

export const hasWails = (): boolean => Boolean(wailsWindow()?.go?.main?.App);

const appCall = (name: string, ...args: unknown[]): unknown => wailsWindow()?.go?.main?.App?.[name]?.(...args);

const stoppedPlayback = { ...initialPlaybackState };

/** Facade over generated Wails bindings. Return types stay loose until Go models are mirrored in entities. */
export const api = {
	bootstrap: () => (hasWails() ? Bootstrap() : bootstrapFallback()),
	snapshot: () => (hasWails() ? Snapshot() : bootstrapFallback()),
	getPlaybackState: () => (hasWails() ? GetPlaybackState() : Promise.resolve(stoppedPlayback)),
	addFolder: () => (hasWails() ? AddFolder() : bootstrapFallback()),
	addFiles: () => (hasWails() ? AddFiles() : bootstrapFallback()),
	addPodcastFolder: () => (hasWails() ? AddPodcastFolder() : bootstrapFallback()),
	addPodcastFiles: () => (hasWails() ? AddPodcastFiles() : bootstrapFallback()),

	addExternalLink: (url: string, libraryType: string) =>
		hasWails() ? AddExternalLink(url, libraryType) : Promise.reject(new Error("Wails runtime unavailable")),
	cancelExternalDownload: (jobId: string) => (hasWails() ? CancelExternalDownload(jobId) : Promise.resolve()),
	retryExternalDownload: (jobId: string) => (hasWails() ? RetryExternalDownload(jobId) : Promise.resolve()),
	deleteExternalItem: (itemId: string, libraryType: string, deleteFile = false) =>
		hasWails() ? DeleteExternalItem(itemId, libraryType, deleteFile) : Promise.resolve(),
	openExternalSource: (url: string) => (hasWails() ? OpenExternalSource(url) : Promise.resolve()),
	getExternalMediaSettings: () =>
		hasWails()
			? GetExternalMediaSettings()
			: Promise.resolve({
					ytDlpPath: "yt-dlp",
					ffmpegPath: "",
					ytDlpDownloadDir: "",
				}),
	saveExternalMediaSettings: (settings: Record<string, unknown>) =>
		hasWails() ? SaveExternalMediaSettings(settings as any) : Promise.resolve(),
	testYtDlp: (path: string) =>
		hasWails() ? TestYtDlp(path) : Promise.resolve({ ok: false, error: "Wails runtime unavailable" }),

	searchPodcasts: (query: string) => (hasWails() ? SearchPodcasts(query) : Promise.resolve([])),
	updatePodcastProgress: (itemId: string, position: number, duration: number) =>
		hasWails()
			? UpdatePodcastProgress(itemId, position, duration)
			: Promise.resolve({ id: itemId, lastPosition: position, duration }),
	playPodcast: (itemId: string) => (hasWails() ? PlayPodcast(itemId) : bootstrapFallback()),
	playPodcastRayItem: (itemId: string) => (hasWails() ? PlayPodcastRayItem(itemId) : bootstrapFallback()),
	nextPodcast: () => (hasWails() ? NextPodcast() : bootstrapFallback()),
	previousPodcast: () => (hasWails() ? PreviousPodcast() : bootstrapFallback()),
	openPodcastRayHistory: (rayId: string) => (hasWails() ? OpenPodcastRayHistory(rayId) : bootstrapFallback()),
	setPodcastRayContentMode: (mode: string) => (hasWails() ? SetPodcastRayContentMode(mode) : bootstrapFallback()),
	setPodcastRaySortMode: (mode: string) => (hasWails() ? SetPodcastRaySortMode(mode) : bootstrapFallback()),
	movePodcastRayItem: (from: number, to: number) => (hasWails() ? MovePodcastRayItem(from, to) : bootstrapFallback()),
	removePodcastRayItem: (itemId: string) => (hasWails() ? RemovePodcastRayItem(itemId) : bootstrapFallback()),
	importPaths: (paths: string[]) =>
		hasWails()
			? appCall("ImportPaths", paths)
			: Promise.resolve({
					inputCount: paths.length,
					audioFound: 0,
					alreadyPresent: 0,
					added: 0,
					skipped: 0,
					errors: [] as string[],
				}),
	importPodcastPaths: (paths: string[]) =>
		hasWails()
			? appCall("ImportPodcastPaths", paths)
			: Promise.resolve({
					inputCount: paths.length,
					audioFound: 0,
					addedOrUpdated: 0,
					skipped: 0,
					errors: [] as string[],
				}),
	searchTracks: (query: string) => (hasWails() ? SearchTracks(query) : []),
	playTrack: (trackId: string) => (hasWails() ? PlayTrack(trackId) : bootstrapFallback()),
	playTrackWithMode: (trackId: string, mode: string) =>
		hasWails() ? PlayTrackWithMode(trackId, mode) : bootstrapFallback(),
	setMusicRayContentMode: (mode: string) => (hasWails() ? SetMusicRayContentMode(mode) : bootstrapFallback()),
	setMusicRaySortMode: (mode: string) => (hasWails() ? SetMusicRaySortMode(mode) : bootstrapFallback()),
	resumeRay: (rayId: string) => (hasWails() ? ResumeRay(rayId) : bootstrapFallback()),
	togglePlay: () => (hasWails() ? TogglePlay() : Promise.resolve(stoppedPlayback)),
	togglePause: () => (hasWails() ? TogglePause() : stoppedPlayback),
	nextTrack: () => (hasWails() ? NextTrack() : bootstrapFallback()),
	previousTrack: () => (hasWails() ? PreviousTrack() : bootstrapFallback()),
	removeFromQueue: (trackId: string) => (hasWails() ? RemoveFromQueue(trackId) : bootstrapFallback()),
	moveQueueItem: (trackId: string, newIndex: number) =>
		hasWails() ? MoveQueueItem(trackId, newIndex) : bootstrapFallback(),
	seek: (positionMs: number) => (hasWails() ? Seek(positionMs) : { positionMs }),
	setVolumePreview: (volume: number) =>
		hasWails() ? appCall("SetVolumePreview", volume) || Promise.resolve({ volume }) : Promise.resolve({ volume }),
	setVolume: (volume: number) => (hasWails() ? SetVolume(volume) : { volume }),
	toggleMute: () => (hasWails() ? ToggleMute() : Promise.resolve(null)),
	setNormalizePodcastVolume: (enabled: boolean) =>
		hasWails() ? SetNormalizePodcastVolume(enabled) : Promise.resolve(null),
	chooseDirectory: (title: string) => (hasWails() ? ChooseDirectory(title) : Promise.resolve("")),
	chooseFile: (title: string, pattern: string) => (hasWails() ? ChooseFile(title, pattern) : Promise.resolve("")),
	getSettings: () =>
		hasWails()
			? GetSettings()
			: Promise.resolve({
					onnxRuntimePath: "",
					miniLMModelDir: "",
					essentiaModelDir: "",
					ffmpegPath: "ffmpeg",
					ffprobePath: "ffprobe",
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
	saveSettings: (payload: Record<string, unknown>) =>
		hasWails() ? SaveSettings(payload as any) : bootstrapFallback(),
	skipToTrackInQueue: (trackId: string) => (hasWails() ? SkipToTrackInQueue(trackId) : bootstrapFallback()),
	testONNXRuntime: (payload: Record<string, unknown>) =>
		hasWails()
			? TestONNXRuntime(payload as any)
			: Promise.resolve({
					ok: false,
					runtimePath: "",
					message: "Wails unavailable",
					latencyMs: 0,
				}),
	testMiniLM: (payload: Record<string, unknown>) =>
		hasWails()
			? TestMiniLM(payload as any)
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
	testFFmpeg: (path: string) => (hasWails() ? TestFFmpeg(path) : Promise.resolve("Wails unavailable")),
	testEssentia: (payload: Record<string, unknown>) =>
		hasWails()
			? TestEssentia(payload as any)
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
					heads: [] as unknown[],
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
	doctorCheck: (component: string, payload: Record<string, unknown>) =>
		hasWails()
			? appCall("DoctorCheck", component, payload)
			: Promise.resolve({
					id: component,
					title: component,
					status: "blocked",
					message: "Wails runtime unavailable",
					repairable: false,
				}),
	doctorRepair: (component: string, payload: Record<string, unknown>) =>
		hasWails()
			? appCall("DoctorRepair", component, payload)
			: Promise.resolve({
					check: {
						id: component,
						title: component,
						status: "blocked",
						message: "Wails runtime unavailable",
						repairable: false,
					},
					patch: {},
				}),
	rayAudit: (trackId: string, mode: string, limit = 20) =>
		hasWails()
			? appCall("RayAudit", trackId, mode, limit)
			: Promise.resolve({ seedTrackId: trackId, mode, rows: [] as unknown[] }),
};
