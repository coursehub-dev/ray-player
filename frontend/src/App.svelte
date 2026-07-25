<script>
import { onMount, tick } from "svelte";
import {
	screen,
	state,
	playbackState,
	rayBuildState,
	searchQuery,
	searchResults,
	reindexStatus,
	indexingState,
	toast,
	bootstrap,
	runSearch,
	syncPayload,
	unbindSnapshotEvents,
} from "./stores/app";
import { cssVariables, emoFlowState, syncEmoFlowFromPayload } from "./stores/emoflow";
import IconButton from "./components/IconButton.svelte";
import UIButton from "./components/UIButton.svelte";
import UISlider from "./components/UISlider.svelte";
import TrackMetaLine from "./components/TrackMetaLine.svelte";
import RayTrackRow from "./components/RayTrackRow.svelte";
import RayBuildSkeleton from "./components/RayBuildSkeleton.svelte";
import PodcastProgressBar from "./components/PodcastProgressBar.svelte";
import AddLinkModal from "./components/AddLinkModal.svelte";
import DoctorModal from "./components/DoctorModal.svelte";
import SettingsSwitch from "./components/SettingsSwitch.svelte";
import { api } from "./lib/api";
import { isPodcastItemId } from "./lib/mediaIdentity";
import { hasPlaybackSelection, resolvePlayerTitle, resolveVisualMode } from "./lib/playerUi";
import { externalDownloads, putExternalDownload, mergedDownloadState } from "./stores/externalDownloads";
import {
	Search,
	History,
	Settings,
	Play,
	Pause,
	LoaderCircle,
	SkipForward,
	SkipBack,
	Volume,
	Volume1,
	Volume2,
	VolumeX,
	Repeat,
	FolderPlus,
	FilePlus2,
	Sparkles,
	ListMusic,
	X,
	Mic,
	Music2,
	CheckCircle2,
	FileText,
	GripVertical,
	Trash2,
	Link,
	Stethoscope,
} from "@lucide/svelte";

let appState = {
	library: [],
	podcasts: [],
	podcastRay: {
		id: "",
		seedItemId: "",
		title: "",
		mode: "",
		folderScope: "",
		currentIndex: -1,
		items: [],
	},
	podcastPlayback: {
		itemId: "",
		rayId: "",
		queueIndex: -1,
		queueLength: 0,
	},
	podcastHistory: [],
	podcastRays: [],
	current: { volume: 0.58, queue: [] },
	history: [],
	rays: [],
	queue: [],
	musicRay: {
		id: "",
		seedTrackId: "",
		title: "",
		contentMode: "stable",
		sortMode: "recommended",
		isManualOrder: false,
		manualUpdatedAt: 0,
		parentRayId: "",
		revision: 1,
		items: [],
		currentIndex: -1,
	},
	libraryStat: { tracks: 0 },
};
let playback = {
	status: "stopped",
	currentTrackId: "",
	positionMs: 0,
	durationMs: 0,
	queueId: "",
	queueIndex: -1,
	queueLength: 0,
	rayId: "",
	raySeedTrackId: "",
	lastError: "",
};

let libraryMode = "music";
let rayBuild = {
	status: "idle",
	seedTrackId: "",
	requestId: 0,
	startedAt: 0,
	finishedAt: 0,
	lastError: "",
};
let currentScreen = "search";
let query = "";
let results = [];
let podcastResults = [];
let modeSwitchBusy = false;
let seekValue = 0;
let stableDurationMs = 0;
let seekInFlight = false;
let volumeValue = 0.58;
let volumePreview = null;
let volumeMuteBusy = false;

// displayedVolume: во время drag показывает preview,
// при mute показывает 0, иначе — реальную громкость.
$: displayedVolume = volumePreview !== null ? volumePreview : appState.current?.muted ? 0 : volumeValue;

$: volumeIconLevel =
	displayedVolume <= 0 ? "muted" : displayedVolume < 0.34 ? "low" : displayedVolume < 0.67 ? "medium" : "high";
let podcastRayUpdating = false;
let draggedPodcastRayIndex = -1;
let podcastRayDropIndex = -1;

let musicRayUpdating = false;
let draggedMusicTrackId = "";
let draggedMusicRayIndex = -1;
let musicRayDropIndex = -1;
let internalRayDrag = false;

const podcastAccentStyle = [
	"--accent:#22b8cf",
	"--accent-rgb:34,184,207",
	"--accent-strong:#38cfe5",
	"--accent-soft:#168ba3",
	"--accent-contrast:#061518",
	"--accent-glow:rgba(34,184,207,0.20)",
	"--accent-glow-strong:rgba(34,184,207,0.34)",
	"--mode-watermark-color:rgba(79,209,224,0.92)",
].join(";");

const defaultMusicAccentStyle = [
	"--accent:#22b8cf",
	"--accent-rgb:34,184,207",
	"--accent-strong:#38cfe5",
	"--accent-soft:#168ba3",
	"--accent-contrast:#061518",
	"--accent-glow:rgba(34,184,207,0.20)",
	"--accent-glow-strong:rgba(34,184,207,0.34)",
	"--mode-watermark-color:rgba(79,209,224,0.92)",
].join(";");

let showSettings = false;
let showDoctor = false;
let showAdvanced = false;
let dragDepth = 0;
let isDragging = false;
let searchInputEl;
let contextMenu = null;
let settingsPayload = {
	onnxRuntimePath: "",
	miniLMModelDir: "",
	essentiaModelDir: "",
	ffmpegPath: "ffmpeg",
	ffprobePath: "ffprobe",
	storagePath: "",
	emoFlowUi: {
		enabled: true,
		intensity: 1,
		animateDuringTrack: true,
		respectReducedMotion: true,
	},
};
let isTestingRuntime = false;
let isTestingMiniLM = false;
let isTestingEssentia = false;
let isTestingFFmpeg = false;
let runtimeTestResult = null;
let miniLMTestResult = null;
let essentiaTestResult = null;
let ffmpegTestResult = null;
let selectedRayMode = "";
let showInsight = false;
let auditRows = [];
let addLinkOpen = false;
let addLinkSubmitting = false;
let addLinkError = "";
let externalSettings = {
	ytDlpPath: "yt-dlp",
	ffmpegPath: "",
	ytDlpDownloadDir: "",
};
let ytDlpCheck = null;
let ytDlpChecking = false;
let externalSettingsSaving = false;

let libraryPlayRequestSeq = 0;

$: emoFlowIntensityPercent = Math.round((settingsPayload.emoFlowUi?.intensity ?? 1) * 100);
const setEmoFlowIntensityPercent = (val) => {
	if (!settingsPayload.emoFlowUi) settingsPayload.emoFlowUi = {};
	settingsPayload.emoFlowUi.intensity = Math.max(0, Math.min(1, val / 100));
};

const probeLabel = (probe) => {
	if (!probe) return "";
	const input = Array.isArray(probe.inputShape) && probe.inputShape.length ? probe.inputShape.join("×") : "";
	const output = Array.isArray(probe.outputShape) && probe.outputShape.length ? probe.outputShape.join("×") : "";
	return [probe.inputName, input, "→", probe.outputName, output].filter(Boolean).join(" ");
};

const unsubscribeState = state.subscribe((v) => {
	appState = v;

	const incomingDuration = Number(v.current?.durationMs) || 0;
	if (incomingDuration > 0) {
		stableDurationMs = incomingDuration;
	}

	if (!seekInFlight) {
		const duration = incomingDuration || stableDurationMs;
		seekValue =
			duration > 0 ? Math.max(0, Math.min(100, Math.round(((v.current?.positionMs || 0) / duration) * 100))) : 0;
	}

	// При mute используем LastNonZeroVolume чтобы slider вернулся
	// к нужной позиции при unmute, а не показывал 0.
	const rawVol = v.current?.volume || 0.58;
	volumeValue = rawVol > 0 ? rawVol : v.current?.lastNonZeroVolume || 0.58;
});
const unsubscribePlayback = playbackState.subscribe((value) => {
	playback = value;
});
const unsubscribeRayBuild = rayBuildState.subscribe((value) => {
	rayBuild = value;
});
const unsubscribeScreen = screen.subscribe((v) => (currentScreen = v));
const unsubscribeQuery = searchQuery.subscribe((v) => (query = v));
const unsubscribeResults = searchResults.subscribe((v) => (results = v));

const buildCssVars = (vars) =>
	Object.entries(vars || {})
		.map(([key, value]) => `${key}: ${value}`)
		.join("; ");

const searchCurrentLibrary = async (value = query) => {
	query = value;

	if (libraryMode === "podcast") {
		podcastResults = value.trim() === "" ? [...(appState.podcasts || [])] : await api.searchPodcasts(value);
		return;
	}

	if (value.trim() === "") {
		await runSearch("");
		results = $searchResults || [];
		return;
	}

	await runSearch(value);
	results = $searchResults || [];
};

const setLibraryMode = async (mode) => {
	if (mode === libraryMode || modeSwitchBusy) {
		return;
	}

	modeSwitchBusy = true;
	try {
		const keepScreen = currentScreen === "history" || currentScreen === "rays";

		libraryMode = mode;
		query = "";
		if (!keepScreen) {
			setScreen("search");
		}

		if (mode === "podcast") {
			podcastResults = [...(appState.podcasts || [])];
			results = [];
		} else {
			podcastResults = [];
			await runSearch("");
			results = $searchResults || [];
		}

		await tick();
		searchInputEl?.focus();
	} finally {
		modeSwitchBusy = false;
	}
};

const toggleLibraryMode = () => setLibraryMode(libraryMode === "podcast" ? "music" : "podcast");

const podcastMeta = (item) => [item.series || item.author, item.folder].filter(Boolean).join(" · ");

const podcastProgress = (item) => {
	if (!item) {
		return 0;
	}

	const stored = Number(item.completedRatio);
	if (Number.isFinite(stored) && stored > 0) {
		return Math.max(0, Math.min(1, stored));
	}

	const position = Number(item.lastPosition) || 0;
	const duration = Number(item.duration) || 0;
	if (duration <= 0) {
		return 0;
	}

	return Math.max(0, Math.min(1, position / duration));
};

const podcastProgressPercent = (item) => Math.round(podcastProgress(item) * 100);

const externalState = (item) => mergedDownloadState($externalDownloads, item);

const externalPlayable = (item) => {
	if (item?.sourceType !== "yt_dlp") {
		return true;
	}
	return externalState(item).status === "ready";
};

const externalStatusLabel = (item) => {
	const state = externalState(item);
	switch (state.status) {
		case "fetching_metadata":
			return "Проверяем ссылку";
		case "queued":
			return "В очереди";
		case "downloading":
			return `Скачивается ${Math.round(state.progress * 100)}%`;
		case "converting":
			return "Конвертируется в MP3";
		case "error":
			return state.error || "Ошибка загрузки";
		case "canceled":
			return "Загрузка отменена";
		case "ready":
			return item?.analysisStatus === "pending" ? "Скачано · анализируется" : "";
		default:
			return "";
	}
};

const podcastContentLabels = {
	recommended: "Рекомендуемое",
	explore: "Исследование",
	current_folder: "Текущая папка",
};

const podcastHistorySourceLabel = (source) => {
	switch (source) {
		case "library":
			return "Из библиотеки";
		case "ray":
			return "Из луча";
		case "ray_auto":
			return "Автопереход луча";
		case "ray_previous":
			return "Назад по лучу";
		case "resume":
			return "Продолжение";
		default:
			return "Ручной запуск";
	}
};

const podcastRayContentLabel = (mode) => podcastContentLabels[mode] || "Рекомендуемое";

const podcastRaySortLabel = (mode) => podcastSortLabels[mode] || "Рекомендуемое";

const openPodcastRayHistory = async (rayId) => {
	await syncPayload(api.openPodcastRayHistory(rayId));
	setScreen("rays");
};

const playPodcastHistoryItem = async (entry) => {
	await playPodcast(entry.item.id, false);
};

const podcastSortLabels = {
	recommended: "Рекомендуемое",
	name_asc: "Название A → Z",
	name_desc: "Название Z → A",
	date_desc: "Сначала новые",
	date_asc: "Сначала старые",
	manual: "Ручной порядок",
};

const musicContentLabels = {
	stable: "Ровный поток",
	warm_up: "Разогрев",
	cool_down: "Снижение",
	intensify: "Интенсивнее",
	deepen: "Глубже",
	explore: "Исследование",
};

const musicSortLabels = {
	recommended: "Рекомендуемое",
	name_asc: "Название A → Z",
	name_desc: "Название Z → A",
	date_desc: "Сначала новые",
	date_asc: "Сначала старые",
	manual: "Ручной порядок",
};

const setMusicContentMode = async (mode) => {
	if (musicRayUpdating || mode === appState.musicRay?.contentMode) {
		return;
	}

	musicRayUpdating = true;
	try {
		await syncPayload(api.setMusicRayContentMode(mode));
	} finally {
		musicRayUpdating = false;
	}
};

const setMusicSortMode = async (mode) => {
	if (musicRayUpdating || mode === appState.musicRay?.sortMode) {
		return;
	}

	musicRayUpdating = true;
	try {
		await syncPayload(api.setMusicRaySortMode(mode));
	} finally {
		musicRayUpdating = false;
	}
};

const setPodcastContentMode = async (mode) => {
	if (mode === appState.podcastRay?.contentMode || podcastRayUpdating) {
		return;
	}

	if (
		appState.podcastRay?.isManualOrder &&
		!window.confirm("Смена наполнения пересоберёт луч и сбросит ручной порядок. Продолжить?")
	) {
		return;
	}

	podcastRayUpdating = true;
	try {
		await syncPayload(api.setPodcastRayContentMode(mode));
	} finally {
		podcastRayUpdating = false;
	}
};

const setPodcastSortMode = async (mode) => {
	if (mode === appState.podcastRay?.sortMode || podcastRayUpdating) {
		return;
	}
	podcastRayUpdating = true;
	try {
		await syncPayload(api.setPodcastRaySortMode(mode));
	} finally {
		podcastRayUpdating = false;
	}
};

const playPodcast = async (itemId, fromRay = false) => {
	const item = (appState.podcasts || []).find((row) => row.id === itemId);
	if (item && !externalPlayable(item)) {
		return;
	}

	if (itemId === playback.currentTrackId) {
		await togglePause();
		return;
	}

	const payload = fromRay ? await api.playPodcastRayItem(itemId) : await api.playPodcast(itemId);
	await syncPayload(Promise.resolve(payload));
	podcastResults = [...(appState.podcasts || [])];
	setScreen("ray");
};

const togglePodcastRow = async (itemId, fromRay = false) => {
	if (itemId === playback.currentTrackId) {
		await togglePause();
		return;
	}
	await playPodcast(itemId, fromRay);
};

const beginPodcastRayDrag = (event, index) => {
	event.stopPropagation();
	internalRayDrag = true;
	draggedPodcastRayIndex = index;
	podcastRayDropIndex = index;
	event.dataTransfer.effectAllowed = "move";
	event.dataTransfer.setData("application/x-podcast-ray-index", String(index));
};

const overPodcastRayItem = (event, index) => {
	if (draggedPodcastRayIndex < 0) {
		return;
	}
	event.preventDefault();
	event.stopPropagation();
	event.dataTransfer.dropEffect = "move";
	podcastRayDropIndex = index;
};

const dropPodcastRayItem = async (event, index) => {
	event.preventDefault();
	event.stopPropagation();
	const raw = event.dataTransfer.getData("application/x-podcast-ray-index");
	const from = Number.parseInt(raw, 10);
	const to = index;
	draggedPodcastRayIndex = -1;
	podcastRayDropIndex = -1;

	if (!Number.isInteger(from) || from === to || podcastRayUpdating) {
		return;
	}

	podcastRayUpdating = true;
	try {
		await syncPayload(api.movePodcastRayItem(from, to));
	} finally {
		podcastRayUpdating = false;
	}
};

const finishPodcastRayDrag = () => {
	internalRayDrag = false;
	draggedPodcastRayIndex = -1;
	podcastRayDropIndex = -1;
};

const removePodcastRayItem = async (event, itemId) => {
	event.stopPropagation();
	if (podcastRayUpdating) {
		return;
	}
	podcastRayUpdating = true;
	try {
		await syncPayload(api.removePodcastRayItem(itemId));
	} finally {
		podcastRayUpdating = false;
	}
};

const beginMusicRayDrag = (event, item, index) => {
	event.stopPropagation();
	internalRayDrag = true;
	draggedMusicTrackId = item.trackId;
	draggedMusicRayIndex = index;
	musicRayDropIndex = index;

	event.dataTransfer.effectAllowed = "move";
	event.dataTransfer.setData("application/x-music-ray-track", item.trackId);
	event.dataTransfer.setData("application/x-music-ray-index", String(index));
};

const overMusicRayItem = (event, index) => {
	if (!draggedMusicTrackId) {
		return;
	}
	event.preventDefault();
	event.stopPropagation();
	event.dataTransfer.dropEffect = "move";
	musicRayDropIndex = index;
};

const dropMusicRayItem = async (event, index) => {
	if (!draggedMusicTrackId) {
		return;
	}

	event.preventDefault();
	event.stopPropagation();

	const trackId = event.dataTransfer.getData("application/x-music-ray-track") || draggedMusicTrackId;

	const from = draggedMusicRayIndex;
	const to = index;

	finishMusicRayDrag();

	if (!trackId || from === to || musicRayUpdating) {
		return;
	}

	musicRayUpdating = true;
	try {
		await syncPayload(api.moveQueueItem(trackId, to));
	} finally {
		musicRayUpdating = false;
	}
};

const finishMusicRayDrag = () => {
	internalRayDrag = false;
	draggedMusicTrackId = "";
	draggedMusicRayIndex = -1;
	musicRayDropIndex = -1;
};

const playNext = async () => {
	if (libraryMode === "podcast") {
		await syncPayload(api.nextPodcast());
		return;
	}
	await nextTrack();
};

const playPrevious = async () => {
	if (libraryMode === "podcast") {
		await syncPayload(api.previousPodcast());
		return;
	}
	await previousTrack();
};

onMount(async () => {
	await bootstrap();

	appState = $state;
	podcastResults = appState.podcasts || [];

	await runSearch("");
	results = $searchResults || [];
	return () => {
		unsubscribeState();
		unsubscribePlayback();
		unsubscribeRayBuild();
		unsubscribeScreen();
		unsubscribeQuery();
		unsubscribeResults();
		unbindSnapshotEvents();
	};
});

const setScreen = (value) => screen.set(value);

const focusSearch = async () => {
	setScreen("search");
	await tick();
	searchInputEl?.focus();
	searchInputEl?.select?.();
};

const handleKeydown = async (event) => {
	if (event.key === "Escape") {
		contextMenu = null;
		if (showDoctor) {
			showDoctor = false;
			return;
		}
		if (showSettings) {
			showSettings = false;
		}
		return;
	}
	const tag = event.target?.tagName?.toLowerCase?.() || "";
	const inEditable = tag === "input" || tag === "textarea" || event.target?.isContentEditable;
	if ((event.metaKey || event.ctrlKey) && event.key?.toLowerCase() === "k") {
		event.preventDefault();
		await focusSearch();
		return;
	}
	if ((event.metaKey || event.ctrlKey) && event.key?.toLowerCase() === "l") {
		event.preventDefault();
		setScreen("ray");
		return;
	}
	if (event.altKey && event.key?.toLowerCase() === "d") {
		event.preventDefault();
		await openSettings();
		showAdvanced = true;
		return;
	}
	if (inEditable) {
		return;
	}
	if (event.key === " ") {
		event.preventDefault();
		await togglePause();
		return;
	}
	if (event.key === "ArrowRight") {
		event.preventDefault();
		const duration = appState.current?.durationMs || 0;
		const next = Math.min((appState.current?.positionMs || 0) + 5000, duration);
		const statePatch = await api.seek(next);
		state.update((prev) => ({
			...prev,
			current: { ...prev.current, ...statePatch },
		}));
		return;
	}
	if (event.key === "ArrowLeft") {
		event.preventDefault();
		const next = Math.max((appState.current?.positionMs || 0) - 5000, 0);
		const statePatch = await api.seek(next);
		state.update((prev) => ({
			...prev,
			current: { ...prev.current, ...statePatch },
		}));
	}
};

const closeTrackMenu = () => {
	contextMenu = null;
};

const openTrackMenu = (event, trackId, source = "search") => {
	event.preventDefault();
	event.stopPropagation();
	contextMenu = {
		trackId,
		source,
		x: event.clientX,
		y: event.clientY,
	};
};

const startNewRayFromMenu = async () => {
	const trackId = contextMenu?.trackId;
	closeTrackMenu();
	if (!trackId) return;
	await syncPayload(api.playTrackWithMode(trackId, selectedRayMode));
	await refreshAudit(trackId);
	setScreen("ray");
};

const playTrackFromMenu = async () => {
	const trackId = contextMenu?.trackId;
	const source = contextMenu?.source;
	closeTrackMenu();
	if (!trackId) return;
	if (source === "ray") {
		await playTrackFromQueue(trackId);
		return;
	}
	await playOrToggle(trackId, "ray");
};

const togglePause = async () => {
	if (playback.status === "loading") {
		return;
	}

	const next = await api.togglePlay();
	playbackState.set(next);
	state.update((prev) => ({
		...prev,
		current: { ...prev.current, ...next },
	}));
};

const playOrToggle = async (trackId, targetScreen = null) => {
	const track = findTrackById(trackId);
	if (track && !externalPlayable(track)) {
		return;
	}

	if (trackId === playback.currentTrackId) {
		await togglePause();
		return;
	}

	const requestSeq = ++libraryPlayRequestSeq;

	if (targetScreen) {
		setScreen(targetScreen);
	}

	try {
		const payload = await api.playTrackWithMode(trackId, selectedRayMode);

		if (requestSeq !== libraryPlayRequestSeq) {
			return;
		}

		if (payload?.library) {
			state.set(payload);
			if (payload.current?.status) {
				playbackState.set(payload.current);
			}
			syncEmoFlowFromPayload(payload);
		}

		await searchCurrentLibrary(query);
	} catch (error) {
		if (requestSeq !== libraryPlayRequestSeq) {
			return;
		}
		console.error("play ray failed", error);
		toast.set({
			kind: "error",
			message: error?.message || "Не удалось построить луч",
		});
	}
};

const refreshAudit = async (trackId = appState.current?.currentTrackId) => {
	if (!trackId || !showInsight) {
		auditRows = [];
		return;
	}
	const audit = await api.rayAudit(trackId, selectedRayMode, 12);
	auditRows = audit?.rows || [];
};

const playTrackFromQueue = async (trackId) => {
	if (trackId === playback.currentTrackId) {
		await togglePause();
		return;
	}
	await syncPayload(api.skipToTrackInQueue(trackId));
};

const resumeRay = async (rayId) => {
	await syncPayload(api.resumeRay(rayId));
	setScreen("ray");
};
const nextTrack = async () => syncPayload(api.nextTrack());
const previousTrack = async () => syncPayload(api.previousTrack());
const addFolder = async () => {
	await syncPayload(libraryMode === "podcast" ? api.addPodcastFolder() : api.addFolder());
	if (libraryMode === "podcast") {
		podcastResults = appState.podcasts || [];
	}
	setScreen("search");
};
const addFiles = async () => {
	await syncPayload(libraryMode === "podcast" ? api.addPodcastFiles() : api.addFiles());
	if (libraryMode === "podcast") {
		podcastResults = appState.podcasts || [];
	}
	setScreen("search");
};
const removeFromQueue = async (trackId) => syncPayload(api.removeFromQueue(trackId));
const changeSeek = async (event) => {
	const pct = Number(event.currentTarget.value);
	const duration = appState.current?.durationMs || 0;
	const target = Math.round((duration * pct) / 100);
	const next = await api.seek(target);
	state.update((prev) => ({
		...prev,
		current: { ...prev.current, ...next },
	}));
};
const changeVolume = async (event) => {
	const value = Number(event.currentTarget.value) / 100;
	const next = await api.setVolume(value);
	state.update((prev) => ({
		...prev,
		current: { ...prev.current, ...next },
	}));
};

const openSettings = async () => {
	showDoctor = false;
	settingsPayload = await api.getSettings();
	settingsPayload.emoFlowUi = {
		enabled: true,
		intensity: 1,
		animateDuringTrack: true,
		respectReducedMotion: true,
		...(settingsPayload.emoFlowUi || {}),
	};
	externalSettings = await api.getExternalMediaSettings();
	ytDlpCheck = null;
	runtimeTestResult = null;
	miniLMTestResult = null;
	essentiaTestResult = null;
	ffmpegTestResult = null;
	showSettings = true;
};
const pickOnnxRuntime = async () => {
	const file = await api.chooseFile("Выберите библиотеку ONNX Runtime", "*.dylib;*.so;*.dll");
	if (file) {
		settingsPayload.onnxRuntimePath = file;
		runtimeTestResult = null;
		miniLMTestResult = null;
		essentiaTestResult = null;
	}
};
const pickMiniLMDir = async () => {
	const dir = await api.chooseDirectory("Выберите папку MiniLM модели");
	if (dir) {
		settingsPayload.miniLMModelDir = dir;
		miniLMTestResult = null;
	}
};
const pickEssentiaDir = async () => {
	const dir = await api.chooseDirectory("Выберите папку моделей Essentia");
	if (dir) {
		settingsPayload.essentiaModelDir = dir;
		essentiaTestResult = null;
	}
};
const saveSettings = async () => {
	await syncPayload(api.saveSettings(settingsPayload));
	showDoctor = false;
	showSettings = false;
};

const applyDoctorPatch = (patch) => {
	settingsPayload = {
		...settingsPayload,
		...(patch?.onnxRuntimePath ? { onnxRuntimePath: patch.onnxRuntimePath } : {}),
		...(patch?.miniLMModelDir ? { miniLMModelDir: patch.miniLMModelDir } : {}),
		...(patch?.ffmpegPath ? { ffmpegPath: patch.ffmpegPath } : {}),
		...(patch?.ffprobePath ? { ffprobePath: patch.ffprobePath } : {}),
	};
	runtimeTestResult = null;
	miniLMTestResult = null;
	essentiaTestResult = null;
	ffmpegTestResult = null;
};

const checkYtDlp = async () => {
	ytDlpChecking = true;
	try {
		ytDlpCheck = await api.testYtDlp(externalSettings.ytDlpPath);
	} finally {
		ytDlpChecking = false;
	}
};

const saveExternalSettings = async () => {
	externalSettingsSaving = true;
	try {
		await api.saveExternalMediaSettings(externalSettings);
	} finally {
		externalSettingsSaving = false;
	}
};

const openAddLinkModal = () => {
	addLinkError = "";
	addLinkOpen = true;
};

const closeAddLinkModal = () => {
	if (addLinkSubmitting) {
		return;
	}
	addLinkOpen = false;
	addLinkError = "";
};

const submitExternalLink = async (event) => {
	const url = event.detail?.url?.trim();
	if (!url || addLinkSubmitting) {
		return;
	}

	addLinkSubmitting = true;
	addLinkError = "";
	try {
		const job = await api.addExternalLink(url, libraryMode === "podcast" ? "podcast" : "music");
		putExternalDownload(job);
		addLinkOpen = false;
		await bootstrap();
		await searchCurrentLibrary("");
	} catch (error) {
		addLinkError = error?.message || String(error) || "Не удалось добавить ссылку";
	} finally {
		addLinkSubmitting = false;
	}
};

const previewSeek = (nextRatio) => {
	seekValue = Math.round((Number(nextRatio) || 0) * 100);
};

const commitSeek = async (nextRatio) => {
	const duration = Number(appState.current?.durationMs) || stableDurationMs;
	if (duration <= 0 || seekInFlight) {
		return;
	}

	const ratio = Math.max(0, Math.min(1, Number(nextRatio) || 0));
	const target = Math.round(duration * ratio);

	seekInFlight = true;
	seekValue = Math.round(ratio * 100);
	try {
		const next = await api.seek(target);
		state.update((prev) => ({
			...prev,
			current: {
				...prev.current,
				...next,
				durationMs: next?.durationMs || prev.current?.durationMs || stableDurationMs,
			},
		}));
	} finally {
		seekInFlight = false;
	}
};

// --- Volume with mute support ---
let volumeFrame = 0;
let pendingVolume = null;

const setPlayerVolume = (value) => {
	value = Math.max(0, Math.min(1, Number(value) || 0));
	volumePreview = value;
	pendingVolume = value;

	if (volumeFrame) return;

	volumeFrame = requestAnimationFrame(async () => {
		volumeFrame = 0;
		const next = pendingVolume;
		pendingVolume = null;

		try {
			const newState = await api.setVolume(next);
			if (newState) {
				state.update((prev) => ({
					...prev,
					current: { ...prev.current, ...newState },
				}));
			}
		} finally {
			volumePreview = null;
		}
	});
};

const togglePlayerMute = async () => {
	if (volumeMuteBusy) return;
	volumeMuteBusy = true;
	try {
		const newState = await api.toggleMute();
		if (newState) {
			state.update((prev) => ({
				...prev,
				current: { ...prev.current, ...newState },
			}));
		}
	} finally {
		volumePreview = null;
		volumeMuteBusy = false;
	}
};

// Устаревшие обработчики (оставляем для обратной совместимости).
let lastVolumeSend = 0;
const previewVolume = async (nextValue) => {
	setPlayerVolume(nextValue);
};

const commitVolume = async (nextValue) => {
	const value = Math.max(0, Math.min(1, Number(nextValue) || 0));
	volumePreview = value;
	const newState = await api.setVolume(value);
	if (newState) {
		state.update((prev) => ({
			...prev,
			current: { ...prev.current, ...newState },
		}));
	}
	volumePreview = null;
};

const toggleRepeatRay = async () => {
	settingsPayload.repeatRay = !settingsPayload.repeatRay;
	await syncPayload(api.saveSettings(settingsPayload));
};

const dataTransferHasFiles = (dataTransfer) => {
	if (!dataTransfer) {
		return false;
	}

	const types = Array.from(dataTransfer.types || []);
	return types.includes("Files");
};

const dataTransferMayContainURL = (dataTransfer) => {
	const types = Array.from(dataTransfer?.types || []);
	return types.includes("text/uri-list") || types.includes("text/plain");
};

const extractDroppedURL = (dataTransfer) => {
	if (!dataTransfer) {
		return "";
	}
	const uriList = (dataTransfer.getData("text/uri-list") || "")
		.split(/\r?\n/)
		.map((value) => value.trim())
		.find((value) => value && !value.startsWith("#") && /^https?:\/\//i.test(value));
	if (uriList) {
		return uriList;
	}
	const plain = (dataTransfer.getData("text/plain") || "").trim();
	const match = plain.match(/https?:\/\/[^\s<>"']+/i);
	return match?.[0] || "";
};

const isExternalImportDrag = (event) =>
	!internalRayDrag && (dataTransferHasFiles(event.dataTransfer) || dataTransferMayContainURL(event.dataTransfer));

function onDragEnter(event) {
	if (!isExternalImportDrag(event)) {
		return;
	}

	event.preventDefault();
	dragDepth += 1;
	isDragging = true;
}
function onDragOver(event) {
	if (!isExternalImportDrag(event)) {
		return;
	}

	event.preventDefault();
	event.dataTransfer.dropEffect = "copy";
}
function onDragLeave(event) {
	if (!isExternalImportDrag(event)) {
		return;
	}

	event.preventDefault();
	dragDepth -= 1;
	if (dragDepth <= 0) {
		dragDepth = 0;
		isDragging = false;
	}
}
async function onDrop(event) {
	if (!isExternalImportDrag(event)) {
		return;
	}

	event.preventDefault();
	dragDepth = 0;
	isDragging = false;
	const droppedURL = extractDroppedURL(event.dataTransfer);
	if (droppedURL) {
		const job = await api.addExternalLink(droppedURL, libraryMode === "podcast" ? "podcast" : "music");
		putExternalDownload(job);
		await bootstrap();
		await searchCurrentLibrary("");
		return;
	}
	const paths = Array.from(event.dataTransfer?.files ?? [])
		.map((f) => f.path || f.webkitRelativePath)
		.filter(Boolean);
	if (paths.length > 0) {
		if (libraryMode === "podcast") {
			await api.importPodcastPaths(paths);
		} else {
			await api.importPaths(paths);
		}
		await bootstrap();
		appState = $state;
		await searchCurrentLibrary("");
	}
}

const testRuntime = async () => {
	isTestingRuntime = true;
	try {
		runtimeTestResult = await api.testONNXRuntime(settingsPayload);
	} finally {
		isTestingRuntime = false;
	}
};

const testMiniLM = async () => {
	isTestingMiniLM = true;
	try {
		miniLMTestResult = await api.testMiniLM(settingsPayload);
	} finally {
		isTestingMiniLM = false;
	}
};

const testFFmpeg = async () => {
	isTestingFFmpeg = true;
	try {
		const result = await api.testFFmpeg(settingsPayload.ffmpegPath);
		ffmpegTestResult = {
			ok: true,
			runtimePath: settingsPayload.ffmpegPath || "ffmpeg",
			message: result,
		};
	} catch (error) {
		ffmpegTestResult = {
			ok: false,
			runtimePath: settingsPayload.ffmpegPath || "ffmpeg",
			message: error?.message || String(error),
		};
	} finally {
		isTestingFFmpeg = false;
	}
};

const testEssentia = async () => {
	isTestingEssentia = true;
	try {
		essentiaTestResult = await api.testEssentia(settingsPayload);
	} finally {
		isTestingEssentia = false;
	}
};

const debugReindex = async () => {
	const result = await api.debugReindexLibrary();
	if (result?.message) {
		console.log("[ui] debug reindex", result);
	}
	if (result && result.started === false) {
		reindexStatus.set({
			active: false,
			index: result.total || 0,
			total: result.total || 0,
			stage: "busy",
			state: "error",
			message: result.message || "reindex busy",
			trackId: "",
			path: "",
		});
	}
};

const genreBadge = (track) => track?.genreLabel || track?.genrePrimary || "";

const trackById = (id) => (appState.library || []).find((t) => t.id === id);
const findTrackById = (trackId) => {
	if (!trackId) return null;
	return trackById(trackId) || null;
};
const getTrackUIState = (trackId) => ({
	isPlayingTrack: trackId === playback.currentTrackId,
	isRaySeed: trackId === playback.raySeedTrackId && Boolean(playback.rayId),
	isActuallyPlaying: trackId === playback.currentTrackId && playback.status === "playing",
	isPausedCurrent: trackId === playback.currentTrackId && playback.status === "paused",
	isLoadingCurrent: trackId === playback.currentTrackId && playback.status === "loading",
});
const rowIcon = (trackId) => (getTrackUIState(trackId).isActuallyPlaying ? "Ⅱ" : "▶");
const rowCurrent = (trackId) => getTrackUIState(trackId).isPlayingTrack;
const rowRaySeed = (trackId) => getTrackUIState(trackId).isRaySeed;

const rowIsBuildingRay = (trackId) => rayBuild.status === "building" && rayBuild.seedTrackId === trackId;

$: isRayBuilding = rayBuild.status === "building";

const toggleInsight = async () => {
	showInsight = !showInsight;

	if (showInsight && playback.currentTrackId && rayBuild.status !== "building") {
		await refreshAudit(playback.currentTrackId);
	} else if (!showInsight) {
		auditRows = [];
	}
};

const isCurrentTrackPlaying = () => playback.status === "playing" && Boolean(playback.currentTrackId);
const rayPlaying = () => Boolean(playback.status === "playing" && playback.currentTrackId);

const emotionLabels = {
	happy: "happy",
	joy: "happy",
	party: "party",
	uplift: "uplift",
	aggressive: "aggressive",
	combat: "aggressive",
	tense: "tense",
	pressure: "tense",
	sad: "sad",
	melancholy: "melancholic",
	melancholic: "melancholic",
	relaxed: "relaxed",
	serene: "serene",
	serenity: "serene",
	dreamy: "dreamy",
	dream: "dreamy",
	dark: "dark",
	soft: "soft",
	warm: "warm",
	swagger: "groovy",
	electronic: "electronic",
	neutral: "neutral",
};

const normalizeEmotionKey = (value) => {
	const raw = String(value || "")
		.trim()
		.toLowerCase();

	if (!raw) return "";

	const exact = emotionLabels[raw];
	if (exact) return exact;

	const parts = raw.split(/[_\s-]+/).filter(Boolean);
	for (const part of parts) {
		if (emotionLabels[part]) {
			return emotionLabels[part];
		}
	}

	return raw.replaceAll("_", " ");
};

const currentEmotionLabel = (emoFlow) => {
	const current = emoFlow?.current;
	return normalizeEmotionKey(current?.dominant || current?.basis?.label || current?.label || emoFlow?.basis?.label);
};

function num(value, digits = 2) {
	const n = Number(value);
	if (!Number.isFinite(n)) return "—";
	return n.toFixed(digits);
}

function pct01(value) {
	const n = Number(value);
	if (!Number.isFinite(n)) return "—";
	return Math.round(Math.max(0, Math.min(1, n)) * 100).toString();
}

function shortText(value, fallback = "—") {
	const s = String(value ?? "").trim();
	return s || fallback;
}

function compactGenreTags(track) {
	const tags = track?.genreTags;
	if (!Array.isArray(tags) || tags.length === 0) return "";
	return tags
		.slice(0, 4)
		.map((tag) => {
			const name = tag?.label || tag?.Label || tag?.name || tag?.Name || tag?.genre || tag?.Genre || "";
			const detail = tag?.detail || tag?.Detail || "";
			const score = Number(
				tag?.score ?? tag?.Score ?? tag?.probability ?? tag?.Probability ?? tag?.value ?? tag?.Value,
			);
			const title = [name, detail].filter(Boolean).join(":");
			if (!title) return "";
			if (Number.isFinite(score)) return `${title}:${score.toFixed(2)}`;
			return title;
		})
		.filter(Boolean)
		.join(",");
}

function insightWarningClass(value, metric) {
	const n = Number(value);
	if (!Number.isFinite(n)) return "";
	if (metric === "jump" && n > 0.25) return "debug-warn";
	if (metric === "low" && n < 0.5) return "debug-warn";
	if (metric === "tempo" && n < 0.45) return "debug-warn";
	return "";
}

function playlistInsightLine(item, index) {
	const ins = item?.insight || {};
	const emo = ins.emotion || {};
	const parts = [];
	parts.push(`flow: ${shortText(ins.bucket || item?.bucket, "—")}/${shortText(ins.strategy || item?.strategy, "—")}`);
	parts.push(`pos ${index + 1}`);
	parts.push(`score ${num(ins.score ?? item?.score, 2)}`);
	parts.push(`mode ${shortText(ins.mode)}`);
	parts.push(`tr ${shortText(ins.transition)}`);
	parts.push(`sim ${num(ins.similarity, 2)}`);
	parts.push(`mood ${num(ins.moodSmoothness, 2)}`);
	parts.push(`dist ${num(ins.moodDistance, 2)}`);
	parts.push(`jump ${num(ins.jumpPenalty, 2)}`);
	parts.push(`tempo ${num(ins.tempoCompatibility, 2)}`);
	if (emo.label) parts.push(`emo ${shortText(emo.prevLabel)}→${shortText(emo.label)}`);
	if (emo.distance !== undefined) parts.push(`edist ${num(emo.distance, 2)}`);
	if (emo.hardJump !== undefined) parts.push(`ehard ${num(emo.hardJump, 2)}`);
	if (emo.bridgeScore !== undefined) parts.push(`ebr ${num(emo.bridgeScore, 2)}`);
	if (emo.rawDistance !== undefined) parts.push(`raw ${num(emo.rawDistance, 2)}`);
	if (emo.edgeDrive !== undefined) parts.push(`edge ${num(emo.edgeDrive, 2)}`);
	if (emo.dirtyElectro !== undefined) parts.push(`dirty ${num(emo.dirtyElectro, 2)}`);
	if (emo.textureConfidence !== undefined) parts.push(`tconf ${num(emo.textureConfidence, 2)}`);
	if (ins.tempoUnknown) parts.push(`tempoUnknown`);
	parts.push(`tex ${num(ins.textureContinuity, 2)}`);
	parts.push(`voc ${num(ins.vocalContinuity, 2)}`);
	parts.push(`sess ${num(ins.sessionFit, 2)}`);
	parts.push(`target ${num(ins.targetMoodFit, 2)}`);
	parts.push(`nov ${num(ins.novelty, 2)}`);
	parts.push(`Δe ${num(ins.energyDelta, 2)}`);
	parts.push(`conf ${num(ins.confidence, 2)}`);
	if (ins.confidence >= 0.9 && (ins.tempoUnknown || ins.fallback || ins.warning)) parts.push(`conf?`);
	if (ins.fallback) parts.push(`fallback ${ins.fallback}`);
	if (ins.warning) parts.push(`warn ${ins.warning}`);
	return parts.join(" · ");
}

function trackDebugLine(item) {
	const track = item?.track || findTrackById(item?.trackId);
	if (!track) return `track: no track payload id=${item?.trackId || "—"}`;
	const genre = [track.genreLabel, track.genrePrimary, track.genreDetail, track.genre, compactGenreTags(track)]
		.map((v) => String(v || "").trim())
		.filter(Boolean)
		.join(" / ");

	const parts = [];
	parts.push(`track: ${shortText(track.title)}`);
	parts.push(`artist ${shortText(track.artist)}`);
	parts.push(`album ${shortText(track.album)}`);
	parts.push(`dur ${shortText(track.durationLabel)}`);
	parts.push(`bpm ${num(track.bpmPerceived || track.tempo, 1)}`);
	parts.push(`rawTempo ${num(track.tempo, 1)}`);
	parts.push(`half ${num(track.bpmHalf, 1)}`);
	parts.push(`double ${num(track.bpmDouble, 1)}`);
	parts.push(`tConf ${num(track.tempoConfidence, 2)}`);
	parts.push(`tStab ${num(track.tempoStability, 2)}`);
	parts.push(`tSrc ${shortText(track.tempoSource)}`);
	parts.push(`genre ${genre || shortText(track.genre)}`);
	parts.push(`E ${num(track.energy, 2)}`);
	parts.push(`dance ${num(track.danceability, 2)}`);
	parts.push(`val ${num(track.valence, 2)}`);
	parts.push(`happy ${num(track.happy, 2)}`);
	parts.push(`sad ${num(track.sad, 2)}`);
	parts.push(`relax ${num(track.relaxed, 2)}`);
	parts.push(`party ${num(track.party, 2)}`);
	parts.push(`aggr ${num(track.aggressive, 2)}`);
	parts.push(`ac ${num(track.acousticness, 2)}`);
	parts.push(`el ${num(track.electronicness, 2)}`);
	parts.push(`instr ${num(track.instrumentalness, 2)}`);
	parts.push(`vocal ${num(track.vocalness, 2)}`);
	parts.push(`mel ${num(track.melodicness, 2)}`);
	parts.push(`soft ${num(track.softness, 2)}`);
	parts.push(`heavy ${num(track.heaviness, 2)}`);
	parts.push(`dream ${num(track.dreaminess, 2)}`);
	parts.push(`emo ${num(track.emotionality, 2)}`);
	parts.push(`bright ${num(track.timbreBrightness, 2)}`);
	parts.push(`tonal ${num(track.tonality, 2)}`);
	parts.push(`approach ${num(track.approachability, 2)}`);
	parts.push(`engage ${num(track.engagement, 2)}`);
	parts.push(`loud ${num(track.loudness, 2)}`);
	parts.push(`centroid ${num(track.spectralCentroid, 0)}`);
	parts.push(`zcr ${num(track.zeroCrossingRate, 3)}`);
	parts.push(`rms ${num(track.rms, 3)}`);
	parts.push(`cluster ${track.clusterId ?? "—"}`);
	parts.push(`plays ${track.playCount ?? 0}`);
	parts.push(`skips ${track.skipCount ?? 0}`);
	parts.push(`complete ${track.completeCount ?? 0}`);
	parts.push(`lvl ${track.analyzedLevel ?? "—"}`);
	parts.push(`aStatus ${shortText(track.analysisStatus)}`);
	parts.push(`import ${shortText(track.importStatus)}`);
	parts.push(`model ${shortText(track.essentiaModelVersion)}`);
	if (track.analysisError) parts.push(`aErr ${track.analysisError}`);
	if (track.tempoError) parts.push(`tErr ${track.tempoError}`);
	if (track.lastError) parts.push(`lastErr ${track.lastError}`);
	parts.push(`missing ${track.fileMissing ? "yes" : "no"}`);
	parts.push(`pbErr ${track.playbackErrorCount ?? 0}`);
	if (track.lastPlaybackError) parts.push(`pbLast ${track.lastPlaybackError}`);
	return parts.join(" · ");
}

$: currentTrack = appState.current || {};
$: emoFlow = $emoFlowState || {};
$: emoFlowCurrent = emoFlow.current || {};
$: emoFlowSummary = emoFlow.reason || emoFlowCurrent.reason || "";
$: emoFlowDirectionLabel = emoFlow.direction || emoFlowCurrent.direction || "stable";

$: emoFlowEmotionLabel = currentEmotionLabel($emoFlowState) || "neutral";

$: playerEmoFlowReason = String(
	$emoFlowState?.transition?.reason || $emoFlowState?.reason || $emoFlowState?.current?.reason || "",
).trim();
$: hasActiveMusic = Boolean(playback.currentTrackId) && !playingPodcast;
$: appShellStyle =
	visualMode === "podcast"
		? podcastAccentStyle
		: hasActiveMusic
			? buildCssVars($cssVariables)
			: defaultMusicAccentStyle;
$: currentTrackMeta = trackById(playback.currentTrackId);
$: currentQueueItem = (appState.queue || []).find((item) => item.trackId === playback.currentTrackId);
$: currentQueueIndex = (appState.queue || []).findIndex((item) => item.trackId === playback.currentTrackId);
$: libraryEmpty =
	libraryMode === "podcast" ? (appState.podcasts || []).length === 0 : (appState.libraryStat?.tracks || 0) === 0;
$: visibleResults = results;
$: appState = $state;

$: if (libraryMode === "podcast" && query.trim() === "") {
	podcastResults = [...(appState.podcasts || [])];
}

$: playingPodcast = isPodcastItemId(playback.currentTrackId);

$: visualMode = resolveVisualMode(libraryMode);

$: currentPodcast = playingPodcast
	? (appState.podcasts || []).find((item) => item.id === playback.currentTrackId) || null
	: null;

$: playbackSelection = hasPlaybackSelection(playback, currentPodcast);

$: playerTitle = resolvePlayerTitle({
	libraryMode,
	playback,
	currentPodcast,
});

$: playerArtist = currentPodcast?.author || currentPodcast?.series || playback.currentArtist || "";

$: playerSubline = playingPodcast
	? [currentPodcast?.series, "Подкаст"].filter(Boolean).join(" · ")
	: playback.currentSub || "";
</script>

<svelte:window on:keydown={handleKeydown} on:click={closeTrackMenu} on:dragenter={onDragEnter} on:dragover={onDragOver} on:dragleave={onDragLeave} on:drop={onDrop} />

<div
    class:indexing={$indexingState.isIndexing}
    class:mode-music={visualMode === "music"}
    class:mode-podcast={visualMode === "podcast"}
    class="app app-shell"
    style={appShellStyle}
>
    <aside class="sidebar">
        <div class="brand">
            <strong>Local Ray Player</strong>
            <span>локальный умный аудиоплеер</span>
        </div>

        <nav class="nav">
            <div class="nav-section nav-top">
                <button
                    class:active={currentScreen === "ray"}
                    class="nav-btn nav-ray-btn"
                    data-playing={rayPlaying() ? "1" : "0"}
                    on:click={() => setScreen("ray")}
                >
                    <span class="nav-icon nav-eq" aria-hidden="true">
                        <i></i><i></i><i></i>
                    </span>
                    <span>Луч</span>
                </button>

                <button
                    type="button"
                    class="sidebar-mode-watermark"
                    class:busy={modeSwitchBusy}
                    aria-label={libraryMode === "podcast"
                        ? "Переключиться в режим музыки"
                        : "Переключиться в режим подкастов"}
                    title={libraryMode === "podcast"
                        ? "Режим подкастов. Нажмите для музыки"
                        : "Режим музыки. Нажмите для подкастов"}
                    on:click={toggleLibraryMode}
                >
                    {#if visualMode === "podcast"}
                        <Mic strokeWidth={1} aria-hidden="true" />
                    {:else}
                        <Music2 strokeWidth={1} aria-hidden="true" />
                    {/if}
                    <span>
                        {libraryMode === "podcast"
                            ? "Подкасты"
                            : "Музыка"}
                    </span>
                </button>

            </div>

            <div class="nav-divider"></div>
            <div class="nav-section nav-bottom">
                <button
                    class:active={currentScreen === "search"}
                    class="nav-btn"
                    on:click={() => setScreen("search")}
                    ><span class="nav-icon"><Search size={16} strokeWidth={1.8} /></span><span>Поиск</span></button
                >
                <button
                    class:active={currentScreen === "history"}
                    class="nav-btn"
                    on:click={() => setScreen("history")}
                    ><span class="nav-icon"><History size={16} strokeWidth={1.8} /></span><span>История</span><small class="nav-count">{libraryMode === "podcast" ? (appState.podcastHistory || []).length : (appState.history || []).length}</small></button
                >
                <button
                    class:active={currentScreen === "rays"}
                    class="nav-btn"
                    on:click={() => setScreen("rays")}
                    ><span class="nav-icon"><ListMusic size={16} strokeWidth={1.8} /></span><span>История лучей</span><small class="nav-count">{libraryMode === "podcast" ? (appState.podcastRays || []).length : (appState.rays || []).length}</small></button
                >
            </div>
        </nav>

        <div class="sidebar-footer">
            <button class="side-action" type="button" on:click={addFolder}
                ><FolderPlus size={16} strokeWidth={1.8} /> Добавить папку {libraryMode === "podcast" ? "подкастов" : "музыки"}</button
            >
            <div class="add-actions">
                <button
                    class="icon-add-button"
                    type="button"
                    title="Добавить ссылку"
                    aria-label="Добавить ссылку"
                    on:click={openAddLinkModal}
                >
                    <Link size={17} strokeWidth={1.8} />
                </button>
                <button class="side-action add-file-button" type="button" on:click={addFiles}
                    ><FilePlus2 size={16} strokeWidth={1.8} /> Добавить {libraryMode === "podcast" ? "выпуск" : "файл"}</button
                >
            </div>
        </div>
    </aside>

    <main class="content">
        {#if currentScreen === "search"}
            <section class="screen active">
                <div class="screen-head">
                    <div>
                        <h1>{libraryMode === "podcast" ? "Подкасты" : "Поиск"}</h1>
                        {#if libraryMode === "podcast"}
                            <p>
                                Отдельная библиотека выпусков с папками,
                                сериями и памятью прогресса.
                            </p>
                        {:else}
                            <p>
                                Поиск по локальной библиотеке. Имя файла, теги,
                                артист, альбом. Клик запускает трек и базовый луч.
                            </p>
                        {/if}
                    </div>
                    <div class="top-right-status">
                        <div class:accent-reactive={$indexingState.isIndexing} class="library-status">
                            {libraryMode === "podcast"
                                ? `${(appState.podcasts || []).length} episodes`
                                : $indexingState.isIndexing && $indexingState.total > 0
                                  ? `${$indexingState.processed}/${$indexingState.total}`
                                  : `${$indexingState.libraryCount || appState.libraryStat?.tracks || 0} tracks`}
                        </div>
                        <IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton>
                    </div>
                </div>
                <div class="screen-body search-layout">
                    <div class="hero">
                        <div class="hero-top">
                            <span class="pulse"></span>
                            <span>{libraryMode === "podcast" ? "PodcastFlow · смысловой маршрут" : `EmoFlow UI · ${emoFlowDirectionLabel}`}</span>
                        </div>
                        <h2>
                            {libraryMode === "podcast"
                                ? "Продолжи тему. Не потеряй место."
                                : "Найди трек. Запусти луч."}
                        </h2>
                        <p>
                            {libraryMode === "podcast"
                                ? "Ищи по названию, автору, серии и папке. Недослушанные выпуски поднимаются выше."
                                : "Минимум визуального шума. Поиск, запуск и умные локальные рекомендации — всё остальное уходит под капот."}
                            {#if libraryMode === "music" && emoFlowSummary}
                                <span class="emoflow-copy">Сейчас: {emoFlowSummary}.</span>
                            {/if}
                        </p>
                        <div class="hero-ambient" aria-hidden="true"></div>
                    </div>
                    <div class="search-input-wrap">
                        <span class="search-mark"><Search size={18} strokeWidth={1.8} /></span>
                        <input
                            bind:this={searchInputEl}
                            class="search-input"
                            bind:value={query}
                            placeholder={libraryMode === "podcast" ? "Искать выпуск, автора, серию или папку" : "Искать в локальной библиотеке"}
                            on:input={(e) => searchCurrentLibrary(e.currentTarget.value)}
                        />
                        <span class="kbd">⌘ / Ctrl + K</span>
                    </div>

                    {#if libraryEmpty}
                        <div class="empty-state">
                            <strong>Библиотека пока пустая</strong>
                            <span
                                >Добавь {libraryMode === "podcast" ? "папку с подкастами или отдельные выпуски" : "папку или отдельные аудиофайлы"} слева.</span
                            >
                        </div>
                    {:else if libraryMode === "podcast"}
                        <div class="list podcast-list">
                            {#each podcastResults as item}
                                <button
                                    type="button"
                                    class:completed={item.isCompleted}
                                    class:current={item.id === playback.currentTrackId}
                                    class:external-pending={!externalPlayable(item)}
                                    disabled={!externalPlayable(item)}
                                    class="row action-row podcast-row"
                                    on:click={() =>
                                        togglePodcastRow(item.id, false)}
                                >
                                    <div class="podcast-icon">
                                        {#if item.id === playback.currentTrackId && playback.status === "playing"}
                                            <Pause size={20} strokeWidth={1.8} />
                                        {:else}
                                            <Play size={20} strokeWidth={1.8} />
                                        {/if}
                                    </div>
                                    <div class="meta">
                                        <strong>{item.title}</strong>
                                        <span class="podcast-meta">
                                            {podcastMeta(item) || "Локальный выпуск"}
                                        </span>
                                        <span class="podcast-progress-label">
                                            {#if item.isCompleted}
                                                Прослушано
                                            {:else if podcastProgressPercent(item) > 0}
                                                Прослушано {podcastProgressPercent(item)}% · продолжить с {Math.floor(item.resumePosition / 60)}:{String(Math.floor(item.resumePosition % 60)).padStart(2, "0")}
                                            {:else}
                                                Новый выпуск
                                            {/if}
                                        </span>
                                        {#if item.sourceType === "yt_dlp" && externalStatusLabel(item)}
                                            <span class="external-download-label">
                                                {externalStatusLabel(item)}
                                            </span>
                                        {/if}
                                    </div>
                                    <div class="tail tail-stack">
                                        <span>
                                            {item.durationLabel || "—"}
                                        </span>
                                        <span
                                            class="semantic-status"
                                            title="Полная semantic-индексация по MiniLM будет добавлена отдельным worker"
                                        >
                                            {#if item.semanticStatus === "done"}
                                                <CheckCircle2 size={12} />
                                                Индекс готов
                                            {:else if item.semanticStatus === "failed"}
                                                Ошибка индекса
                                            {:else}
                                                <FileText size={12} />
                                                Метаданные готовы
                                            {/if}
                                        </span>
                                    </div>

                                    <PodcastProgressBar
                                        {item}
                                        className="podcast-row-progress"
                                    />
                                    {#if item.sourceType === "yt_dlp" && !externalPlayable(item)}
                                        <span class="external-download-progress">
                                            <span style={`width:${Math.round(externalState(item).progress * 100)}%`}></span>
                                        </span>
                                    {/if}
                                </button>
                            {/each}
                        </div>
                    {:else}
                        <div class="list">
                            {#each visibleResults as row}
                                <button
                                    class:itemCurrent={rowCurrent(row.track.id)}
                                    class:external-pending={!externalPlayable(row.track)}
                                    disabled={!externalPlayable(row.track)}
                                    class="row action-row track-row"
                                    on:click={() =>
                                        playOrToggle(row.track.id, "ray")}
                                    on:contextmenu={(event) =>
                                        openTrackMenu(
                                            event,
                                            row.track.id,
                                            "search",
                                        )}
                                >
                                    <div class="cover-wrapper">
                                        <div class="cover"></div>
                                        <div class="cover-play-indicator">
                                            <span
                                                class:icon-playing={rowCurrent(
                                                    row.track.id,
                                                )}
                                            >
                                                {#if rowIsBuildingRay(row.track.id)}
                                                    <LoaderCircle size={15} />
                                                {:else}
                                                    {rowIcon(row.track.id)}
                                                {/if}
                                            </span>
                                            {#if rowRaySeed(row.track.id)}
                                                <Sparkles
                                                    class="ray-seed-icon"
                                                    size={14}
                                                    title="Seed track for current ray"
                                                />
                                            {/if}
                                        </div>
                                    </div>
                                    <div class="meta">
                                        <strong
                                            >{row.track.title ||
                                                row.track.fileName}</strong
                                        >
                                        <TrackMetaLine
                                            track={row.track}
                                            maxGenres={2}
                                            showBpm={true}
                                        />
                                        {#if row.track.sourceType === "yt_dlp" && externalStatusLabel(row.track)}
                                            <span class="external-download-label">
                                                {externalStatusLabel(row.track)}
                                            </span>
                                        {/if}
                                        {#if genreBadge(row.track)}<div
                                                class="badges"
                                            >
                                                <i class="badge genre"
                                                    >{genreBadge(row.track)}</i
                                                >
                                            </div>{/if}
                                    </div>
                                    <div class="tail tail-stack">
                                        <span>{row.track.durationLabel}</span>
                                    </div>
                                    {#if row.track.sourceType === "yt_dlp" && !externalPlayable(row.track)}
                                        <span class="external-download-progress">
                                            <span style={`width:${Math.round(externalState(row.track).progress * 100)}%`}></span>
                                        </span>
                                    {/if}
                                    <span class="track-menu-wrap">
                                        <span
                                            class="track-menu-btn"
                                            role="button"
                                            tabindex="0"
                                            aria-label="Меню трека"
                                            on:click={(event) =>
                                                openTrackMenu(
                                                    event,
                                                    row.track.id,
                                                    "search",
                                                )}
                                            on:keydown={(event) =>
                                                (event.key === "Enter" ||
                                                    event.key === " ") &&
                                                openTrackMenu(
                                                    event,
                                                    row.track.id,
                                                    "search",
                                                )}>⋯</span
                                        >
                                    </span>
                                </button>
                            {/each}
                        </div>
                    {/if}
                </div>
            </section>
        {/if}

        {#if currentScreen === "history"}
            <section class="screen active">
                <div class="screen-head">
                    <div>
                        <h1>{libraryMode === "podcast" ? "История подкастов" : "История"}</h1>
                        <p>{libraryMode === "podcast" ? "Независимая история прослушивания выпусков, прогресса и источников запуска." : "Что слушали, где остановились, и сколько уже прослушано."}</p>
                    </div>
                    <div class="top-right-status"><div class:accent-reactive={$indexingState.isIndexing} class="library-status">{$indexingState.isIndexing && $indexingState.total > 0 ? `${$indexingState.processed}/${$indexingState.total}` : `${$indexingState.libraryCount || appState.libraryStat?.tracks || 0} tracks`}</div><IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton></div>
                </div>
                <div class="screen-body">
                    {#if libraryMode === "podcast"}
                        <div class="list podcast-history-list">
                            {#if (appState.podcastHistory || []).length}
                                {#each appState.podcastHistory as entry}
                                    <button type="button" class="row action-row podcast-history-row" class:current={entry.item.id === playback.currentTrackId} on:click={() => playPodcastHistoryItem(entry)}>
                                        <div class="podcast-history-icon">{#if entry.item.id === playback.currentTrackId && playback.status === "playing"}<Pause size={19} strokeWidth={1.8} />{:else}<Play size={19} strokeWidth={1.8} />{/if}</div>
                                        <div class="meta">
                                            <strong>{entry.item.title}</strong>
                                            <span>{podcastMeta(entry.item) || "Локальный выпуск"}</span>
                                            <small class="podcast-history-state">{entry.playedAtLabel} · {podcastHistorySourceLabel(entry.source)}{#if entry.listenedLabel} · прослушано {entry.listenedLabel}{/if}{#if entry.positionLabel} · остановка {entry.positionLabel}{/if}</small>
                                        </div>
                                        <div class="tail tail-stack">
                                            <span>{entry.progressPercent}%</span>
                                            {#if entry.rayId}
                                                <span class="history-ray-link">Луч</span>
                                            {/if}
                                        </div>
                                        <span class="podcast-history-progress"><span style={`width:${entry.progressPercent}%`}></span></span>
                                    </button>
                                {/each}
                            {:else}
                                <div class="empty-state">
                                    <strong>История подкастов пуста</strong>
                                    <span>Запустите выпуск из библиотеки или подкастового луча.</span>
                                </div>
                            {/if}
                        </div>
                    {:else}
                    <div class="list">
                        {#if appState.history?.length}
                            {#each appState.history as item}
                                <button
                                    class:itemCurrent={rowCurrent(
                                        item.track.id,
                                    )}
                                    class="row action-row track-row"
                                    on:click={() =>
                                        playOrToggle(item.track.id, "ray")}
                                    on:contextmenu={(event) =>
                                        openTrackMenu(
                                            event,
                                            item.track.id,
                                            "history",
                                        )}
                                >
                                    <div class="cover-wrapper">
                                        <div class="cover"></div>
                                        <div class="cover-play-indicator">
                                            <span
                                                class:icon-playing={rowCurrent(
                                                    item.track.id,
                                                )}
                                            >
                                                {rowIcon(item.track.id)}
                                            </span>
                                            {#if rowRaySeed(item.track.id)}
                                                <Sparkles
                                                    class="ray-seed-icon"
                                                    size={14}
                                                    title="Seed track for current ray"
                                                />
                                            {/if}
                                        </div>
                                    </div>
                                    <div class="meta">
                                        <strong>{item.track.title}</strong>
                                        <TrackMetaLine
                                            track={item.track}
                                            maxGenres={2}
                                            showBpm={true}
                                        />
                                        <small class="history-track-state">
                                            {item.playedAtLabel} ·
                                            {item.progressLabel} из
                                            {item.track.durationLabel}
                                        </small>
                                        <div
                                            class="progress"
                                            style="margin-top: 6px;"
                                        >
                                            <i
                                                style={`--w:${Math.round((item.progress ?? 0.4) * 100)}%`}
                                            ></i>
                                        </div>
                                    </div>
                                    <div class="tail">
                                        {item.track.durationLabel}
                                    </div>
                                    <span class="track-menu-wrap">
                                        <span
                                            class="track-menu-btn"
                                            role="button"
                                            tabindex="0"
                                            aria-label="Меню трека"
                                            on:click={(event) =>
                                                openTrackMenu(
                                                    event,
                                                    item.track.id,
                                                    "history",
                                                )}
                                            on:keydown={(event) =>
                                                (event.key === "Enter" ||
                                                    event.key === " ") &&
                                                openTrackMenu(
                                                    event,
                                                    item.track.id,
                                                    "history",
                                                )}>⋯</span
                                        >
                                    </span>
                                </button>
                            {/each}
                        {:else}
                            <div class="empty-state">
                                <strong>История пуста</strong><span
                                    >Запусти трек из поиска — он появится здесь.</span
                                >
                            </div>
                        {/if}
                    </div>
                    {/if}
                </div>
            </section>
        {/if}

        {#if currentScreen === "ray" && libraryMode === "podcast"}
            <section class="screen active podcast-ray-screen">
                <div class="screen-head">
                    <div>
                        <h1>Луч подкастов</h1>
                        <p>
                            {#if appState.podcastRay?.folderScope}
                                Приоритет текущей папки:
                                {appState.podcastRay.folderScope}
                            {:else}
                                Запустите выпуск, чтобы построить смысловой маршрут.
                            {/if}
                        </p>
                    </div>
                    <div class="top-right-status"><div class="library-status">{(appState.podcastRay?.items || []).length} episodes</div><IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton></div>
                </div>

                <div class="screen-body">
                    {#if !(appState.podcastRay?.items || []).length}
                        <div class="empty-state">
                            <strong>Луч ещё не построен</strong>
                            <span>
                                Выберите подкаст в библиотеке. Сначала будут
                                добавлены выпуски из той же папки, затем из
                                соседних папок и серий.
                            </span>
                        </div>
                    {:else}
                        <div class="podcast-ray-toolbar">
                            <label>
                                <span>Наполнение</span>
                                <select
                                    value={appState.podcastRay?.contentMode || "recommended"}
                                    disabled={podcastRayUpdating}
                                    on:change={(event) =>
                                        setPodcastContentMode(
                                            event.currentTarget.value,
                                        )}
                                >
                                    <option value="recommended">
                                        Рекомендуемое
                                    </option>
                                    <option value="explore">
                                        Исследование
                                    </option>
                                    <option value="current_folder">
                                        Текущая папка
                                    </option>
                                </select>
                            </label>

                            <label>
                                <span>Сортировка</span>
                                <select
                                    value={appState.podcastRay?.sortMode || "recommended"}
                                    disabled={podcastRayUpdating}
                                    on:change={(event) =>
                                        setPodcastSortMode(
                                            event.currentTarget.value,
                                        )}
                                >
                                    <option value="recommended">Рекомендуемое</option>
                                    <option value="name_asc">Название A → Z</option>
                                    <option value="name_desc">Название Z → A</option>
                                    <option value="date_desc">Сначала новые</option>
                                    <option value="date_asc">Сначала старые</option>
                                    <option value="manual">Ручной порядок</option>
                                </select>
                            </label>

                            {#if appState.podcastRay?.isManualOrder}
                                <span class="podcast-manual-badge">
                                    Ручной порядок
                                </span>
                            {/if}
                        </div>

                        <div class="podcast-ray-list">
                            {#each appState.podcastRay.items as rayItem}
                                <button
                                    type="button"
                                    class="podcast-ray-row"
                                    class:current={rayItem.item.id === playback.currentTrackId}
                                    class:drop-target={podcastRayDropIndex === rayItem.position && draggedPodcastRayIndex !== rayItem.position}
                                    on:dragover={(event) =>
                                        overPodcastRayItem(
                                            event,
                                            rayItem.position,
                                        )}
                                    on:drop={(event) =>
                                        dropPodcastRayItem(
                                            event,
                                            rayItem.position,
                                        )}
                                    on:click={() =>
                                        togglePodcastRow(rayItem.item.id, true)}
                                >
                                    <span
                                        class="podcast-ray-drag"
                                        draggable="true"
                                        role="button"
                                        tabindex="0"
                                        aria-label="Перетащить"
                                        title="Перетащить"
                                        on:click|stopPropagation
                                        on:keydown={(event) => {
                                            if (
                                                event.key === "Enter" ||
                                                event.key === " "
                                            ) {
                                                event.preventDefault();
                                                event.stopPropagation();
                                            }
                                        }}
                                        on:dragstart={(event) =>
                                            beginPodcastRayDrag(
                                                event,
                                                rayItem.position,
                                            )}
                                        on:dragend={finishPodcastRayDrag}
                                    >
                                        <GripVertical size={15} />
                                    </span>

                                    <span class="podcast-ray-position">
                                        {rayItem.position + 1}
                                    </span>

                                    <span class="podcast-ray-play">
                                        {#if rayItem.item.id === playback.currentTrackId && playback.status === "playing"}
                                            <Pause size={16} strokeWidth={2} />
                                        {:else}
                                            <Play size={16} strokeWidth={2} />
                                        {/if}
                                    </span>

                                    <span class="podcast-ray-copy">
                                        <strong>{rayItem.item.title}</strong>
                                        <small>
                                            {podcastMeta(rayItem.item) ||
                                                "Локальный выпуск"}
                                        </small>
                                        <small class="podcast-ray-reason">
                                            {rayItem.reason}
                                        </small>
                                    </span>

                                    <span class="podcast-ray-tail">
                                        {#if podcastProgressPercent(rayItem.item) > 0}
                                            {podcastProgressPercent(rayItem.item)}%
                                        {:else}
                                            {rayItem.item.durationLabel}
                                        {/if}
                                    </span>

                                    {#if rayItem.item.id !== appState.podcastRay.seedItemId}
                                        <span
                                            class="podcast-ray-remove"
                                            role="button"
                                            tabindex="0"
                                            aria-label="Удалить из луча"
                                            title="Удалить из луча"
                                            on:click={(event) =>
                                                removePodcastRayItem(
                                                    event,
                                                    rayItem.item.id,
                                                )}
                                            on:keydown={(event) => {
                                                if (
                                                    event.key === "Enter" ||
                                                    event.key === " "
                                                ) {
                                                    event.preventDefault();
                                                    removePodcastRayItem(
                                                        event,
                                                        rayItem.item.id,
                                                    );
                                                }
                                            }}
                                        >
                                            <Trash2 size={14} />
                                        </span>
                                    {/if}

                                    <PodcastProgressBar
                                        item={rayItem.item}
                                        className="podcast-ray-progress"
                                    />
                                </button>
                            {/each}
                        </div>
                    {/if}
                </div>
            </section>
        {/if}

        {#if currentScreen === "ray"}
            <section class="screen active">
                <div class="screen-head">
                    <div>
                        <h1>
                            Луч{currentTrack.currentTitle
                                ? ` · ${currentTrack.currentTitle}`
                                : ""}
                        </h1>
                        <p>
                            Текущий луч. Можно ткнуть в любой трек очереди и
                            сразу перескочить на него.
                        </p>
                        <div class="badges" style="margin-top:8px; gap:8px;">
                            <i class="badge emoflow">{emoFlowDirectionLabel}</i>
                            {#if emoFlowEmotionLabel}
                                <span
                                    class={`badge emotion emotion-${emoFlowEmotionLabel.replaceAll(" ", "-")}`}
                                >
                                    {emoFlowEmotionLabel}
                                </span>
                            {/if}
                            <label class="badge genre">mode
                                <select bind:value={selectedRayMode} style="margin-left:6px; background:transparent; color:inherit; border:none;">
                                    <option value="">auto</option>
                                    <option value="continue_mood">continue</option>
                                    <option value="warm_up">warm_up</option>
                                    <option value="cool_down">cool_down</option>
                                    <option value="explore">explore</option>
                                    <option value="deepen">deepen</option>
                                </select>
                            </label>
                            <UIButton compact className="badge-height" on:click={toggleInsight}>{showInsight ? 'hide insight' : 'show insight'}</UIButton>
                        </div>
                    </div>
                    <div class="top-right-status"><div class:accent-reactive={$indexingState.isIndexing} class="library-status">{$indexingState.isIndexing && $indexingState.total > 0 ? `${$indexingState.processed}/${$indexingState.total}` : `${$indexingState.libraryCount || appState.libraryStat?.tracks || 0} tracks`}</div><IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton></div>
                </div>
                <div class="screen-body">
                    {#if (appState.musicRay?.id || appState.queue?.length)}
                        <div class="ray-toolbar">
                            <label>
                                <span>Траектория</span>
                                <select
                                    value={appState.musicRay?.contentMode || "stable"}
                                    disabled={musicRayUpdating}
                                    on:change={(event) =>
                                        setMusicContentMode(
                                            event.currentTarget.value,
                                        )}
                                >
                                    <option value="stable">Ровный поток</option>
                                    <option value="warm_up">Разогрев</option>
                                    <option value="cool_down">Снижение</option>
                                    <option value="intensify">Интенсивнее</option>
                                    <option value="deepen">Глубже</option>
                                    <option value="explore">Исследование</option>
                                </select>
                            </label>

                            <label>
                                <span>Сортировка</span>
                                <select
                                    value={appState.musicRay?.sortMode || "recommended"}
                                    disabled={musicRayUpdating}
                                    on:change={(event) =>
                                        setMusicSortMode(
                                            event.currentTarget.value,
                                        )}
                                >
                                    <option value="recommended">Рекомендуемое</option>
                                    <option value="name_asc">Название A → Z</option>
                                    <option value="name_desc">Название Z → A</option>
                                    <option value="date_desc">Сначала новые</option>
                                    <option value="date_asc">Сначала старые</option>
                                    <option value="manual">Ручной порядок</option>
                                </select>
                            </label>

                            {#if appState.musicRay?.isManualOrder}
                                <span class="ray-manual-badge">
                                    Ручной порядок
                                </span>
                            {/if}
                        </div>
                    {/if}

                    <div class="playlist">
                        {#if isRayBuilding}
                            <RayBuildSkeleton
                                seedTitle={trackById(
                                    rayBuild.seedTrackId,
                                )?.title || playback.currentTitle}
                            />
                        {:else if appState.queue?.length}
                            {#each appState.queue as item, index}
                                <div
                                    class:rowPast={currentQueueIndex >= 0 &&
                                        index < currentQueueIndex}
                                    class:rowFuture={currentQueueIndex >= 0 &&
                                        index > currentQueueIndex}
                                >
                                    <RayTrackRow
                                        {item}
                                        {index}
                                        {playback}
                                        {showInsight}
                                        dropTarget={musicRayDropIndex === index &&
                                            draggedMusicRayIndex !== index}
                                        dragging={draggedMusicRayIndex === index}
                                        insightLine={playlistInsightLine(
                                            item,
                                            index,
                                        )}
                                        debugLine={trackDebugLine(item)}
                                        on:dragstart={(event) =>
                                            beginMusicRayDrag(
                                                event.detail.event,
                                                event.detail.item,
                                                event.detail.index,
                                            )}
                                        on:dragover={(event) =>
                                            overMusicRayItem(
                                                event.detail.event,
                                                event.detail.index,
                                            )}
                                        on:drop={(event) =>
                                            dropMusicRayItem(
                                                event.detail.event,
                                                event.detail.index,
                                            )}
                                        on:dragend={finishMusicRayDrag}
                                        on:play={(event) =>
                                            playTrackFromQueue(
                                                event.detail.trackId,
                                            )}
                                        on:menu={(event) =>
                                            openTrackMenu(
                                                event.detail.event,
                                                event.detail.trackId,
                                                "ray",
                                            )}
                                    />
                                </div>
                            {/each}
                        {:else}
                            <div class="empty-state">
                                <strong>Луч ещё не запущен</strong><span
                                    >Выбери трек в поиске — он станет seed для
                                    базового луча.</span
                                >
                            </div>
                        {/if}
                        {#if showInsight && auditRows.length}
                            <div class="settings-block" style="margin-top:16px;">
                                <div class="small-head">Ray audit</div>
                                {#each auditRows as row}
                                    <div class="probe-row probe-row-tight">
                                        <strong>{row.position}. {row.title}</strong>
                                        <span>{row.reason}</span>
                                        <small>sim {row.insight?.similarity?.toFixed?.(2) || '0.00'} · dist {row.insight?.moodDistance?.toFixed?.(2) || '0.00'} · jump {row.insight?.jumpPenalty?.toFixed?.(2) || '0.00'} · Δe {row.insight?.energyDelta?.toFixed?.(2) || '0.00'} · tempo {row.insight?.tempoCompatibility?.toFixed?.(2) || '0.00'}</small>
                                    </div>
                                {/each}
                            </div>
                        {/if}
                    </div>
                </div>
            </section>
        {/if}

        {#if currentScreen === "rays"}
            <section class="screen active">
                <div class="screen-head">
                    <div>
                        <h1>{libraryMode === "podcast" ? "История подкастовых лучей" : "История лучей"}</h1>
                        <p>{libraryMode === "podcast" ? "Сохранённые смысловые маршруты, их режимы наполнения и ручной порядок." : "Ранее собранные лучи. Нажатие активирует луч и возвращает к нему с сохранением позиции."}</p>
                    </div>
                    <div class="top-right-status"><div class:accent-reactive={$indexingState.isIndexing} class="library-status">{$indexingState.isIndexing && $indexingState.total > 0 ? `${$indexingState.processed}/${$indexingState.total}` : `${$indexingState.libraryCount || appState.libraryStat?.tracks || 0} tracks`}</div><IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton></div>
                </div>
                <div class="screen-body">
                    {#if libraryMode === "podcast"}
                        <div class="podcast-ray-history-list">
                            {#if (appState.podcastRays || []).length}
                                {#each appState.podcastRays as ray}
                                    <button type="button" class="podcast-ray-history-row" class:current={ray.id === appState.podcastRay?.id} on:click={() => openPodcastRayHistory(ray.id)}>
                                        <span class="podcast-ray-history-icon"><Mic size={19} strokeWidth={1.6} /></span>
                                        <span class="podcast-ray-history-copy">
                                            <strong>{ray.title || ray.seed.title}</strong>
                                            <small>{ray.seed.series || ray.seed.author || ray.folderScope || "Подкастовый луч"}</small>
                                            <span class="podcast-ray-history-badges">
                                                <i>{podcastRayContentLabel(ray.contentMode)}</i>
                                                <i>{podcastRaySortLabel(ray.sortMode)}</i>
                                                {#if ray.isManualOrder}
                                                    <i>Ручной порядок</i>
                                                {/if}
                                                {#if ray.parentRayId}
                                                    <i>Версия {ray.revision}</i>
                                                {/if}
                                            </span>
                                        </span>
                                        <span class="podcast-ray-history-tail">
                                            <strong>{ray.itemCount}</strong>
                                            <small>выпусков</small>
                                            <small>{ray.createdAtLabel}</small>
                                        </span>
                                    </button>
                                {/each}
                            {:else}
                                <div class="empty-state">
                                    <strong>История подкастовых лучей пуста</strong>
                                    <span>Запустите подкаст, чтобы построить первый смысловой маршрут.</span>
                                </div>
                            {/if}
                        </div>
                    {:else}
                    <div class="ray-archive">
                        {#if appState.rays?.length}
                            {#each appState.rays as ray}
                                <button
                                    class:active={ray.active}
                                    class="ray-card"
                                    on:click={() => resumeRay(ray.id)}
                                >
                                    <div>
                                        <strong>{ray.name}</strong>
                                        <span
                                            >{ray.trackCount} треков · остановлено
                                            на {ray.currentTrackName} · {ray.resumeLabel}</span
                                        >
                                    </div>
                                    <div class="ray-status">
                                        {ray.active ? "текущий" : "архив"}
                                    </div>
                                </button>
                            {/each}
                        {:else}
                            <div class="empty-state">
                                <strong>История лучей пуста</strong><span
                                    >После первого запуска трека здесь появится
                                    сохранённый луч.</span
                                >
                            </div>
                        {/if}
                    </div>
                    {/if}
                </div>
            </section>
        {/if}
    </main>

    <footer class="player">
        <div class="player-inner">
            <div class="player-now">
                <div class="cover"></div>
                <div class="meta">
                    <strong>{playerTitle}</strong>
                    {#if !playingPodcast && currentTrackMeta}
                        <TrackMetaLine
                            track={currentTrackMeta}
                            maxGenres={2}
                            showBpm={true}
                        />
                    {:else}
                        <span>{playerArtist || playerSubline}</span>
                    {/if}
                    {#if !playingPodcast && playback.currentGenre}
                        <small>{playback.currentGenre}</small>
                    {/if}
                    {#if playback.status === "error" && playback.lastError}
                        <small class="player-error">
                            {playback.lastError}
                        </small>
                    {/if}
                    {#if playerEmoFlowReason &&
                    playback.status !== "error"}
                        <small
                            class="player-emoflow"
                            title={playerEmoFlowReason}
                        >
                            {playerEmoFlowReason}
                        </small>
                    {/if}
                </div>
            </div>

            <div class="transport">
                <div class="controls"><IconButton className={`control-btn ${settingsPayload.repeatRay ? "active accent-reactive" : ""}`} title="Repeat ray" on:click={toggleRepeatRay}><Repeat size={18} strokeWidth={1.8} /></IconButton>
                    <IconButton className="control-btn" title="Previous" disabled={!playbackSelection} on:click={playPrevious}><SkipBack size={18} strokeWidth={1.8} /></IconButton>
                    <UIButton
                        primary
                        className={`play-btn ${
                            playback.status === "loading"
                                ? "loading"
                                : ""
                        }`}
                         disabled={!playbackSelection}
                         on:click={togglePause}
                    >
                        {#if playback.status === "loading"}
                            <LoaderCircle
                                size={18}
                                strokeWidth={2}
                            />
                        {:else if playback.status === "playing"}
                            <Pause size={18} strokeWidth={2} />
                        {:else}
                            <Play size={18} strokeWidth={2} />
                        {/if}
                    </UIButton>
                    <IconButton className="control-btn" title="Next" disabled={!playbackSelection} on:click={playNext}><SkipForward size={18} strokeWidth={1.8} /></IconButton>
                </div>
                <div class="seek">
                    <span>{currentTrack.positionLabel || "0:00"}</span>
                    <UISlider value={seekValue / 100} min={0} max={1} step={0.01} showValue={false} disabled={!playbackSelection} accentReactive={$indexingState.isIndexing} on:preview={(e) => previewSeek(e.detail)} on:commit={(e) => commitSeek(e.detail)} />
                    <span>{currentTrack.durationLabel || "0:00"}</span>
                </div>
            </div>

            <div class="player-side">
                <button
                    type="button"
                    class="volume-icon-button"
                    class:muted={volumeIconLevel === "muted"}
                    aria-label={volumeIconLevel === "muted" ? "Включить звук" : "Выключить звук"}
                    title={volumeIconLevel === "muted" ? "Включить звук" : "Выключить звук"}
                    disabled={volumeMuteBusy}
                    on:click={togglePlayerMute}
                >
                    {#if volumeIconLevel === "muted"}
                        <VolumeX size={16} strokeWidth={1.8} />
                    {:else if volumeIconLevel === "low"}
                        <Volume size={16} strokeWidth={1.8} />
                    {:else if volumeIconLevel === "medium"}
                        <Volume1 size={16} strokeWidth={1.8} />
                    {:else}
                        <Volume2 size={16} strokeWidth={1.8} />
                    {/if}
                </button>
                <div class="volume-range">
                    <UISlider value={displayedVolume} min={0} max={1} step={0.01} showValue={false} accentReactive={$indexingState.isIndexing} on:preview={(e) => setPlayerVolume(e.detail)} on:commit={(e) => commitVolume(e.detail)} />
                </div>
            </div>
        </div>
    </footer>
</div>

{#if contextMenu}
    <div
        class="track-context-menu"
        role="menu"
        tabindex="-1"
        style={`left:${contextMenu.x}px; top:${contextMenu.y}px;`}
        on:mousedown|stopPropagation
    >
        <button
            class="context-menu-item primary"
            on:click={startNewRayFromMenu}
        >
            <span>✦</span>
            <span>Начать новый луч</span>
        </button>
        <button class="context-menu-item" on:click={playTrackFromMenu}>
            <span>▶</span>
            <span
                >{contextMenu.source === "ray"
                    ? "Перейти к треку"
                    : "Воспроизвести"}</span
            >
        </button>
    </div>
{/if}

{#if showSettings}
    <div
        class="settings-overlay"
	role="presentation"
	class:doctor-open={showDoctor}
	on:pointerdown={(e) => e.target === e.currentTarget && (showSettings = false)}
    >
        <section
            class="settings-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="settings-title"
            on:click|stopPropagation
        >
            <header class="settings-header">
                <div class="settings-header-copy">
                    <h2 id="settings-title">Настройки</h2>
                    <p>Настройки приложения и медиатеки.</p>
                </div>

			<div class="settings-header-actions">
				<button
					type="button"
					class="ghost-btn compact doctor-launch"
					on:click={() => (showDoctor = true)}
				>
					<Stethoscope size={15} />
					<span>Доктор</span>
				</button>
				<button
					type="button"
					class="icon-button"
					aria-label="Закрыть настройки"
					on:click={() => { showDoctor = false; showSettings = false; }}
				>
					<X size={18} />
				</button>
			</div>
            </header>

            <div class="settings-scroll">
                <div class="settings-content">
                    <div class="settings-block">
                        <div class="field-hint">
                            Основной интерфейс скрывает технические детали. Расширенные
                            параметры нужны только для диагностики и перенастройки моделей.
                        </div>
                        <button
                            class="ghost-btn compact"
                            type="button"
                            on:click={() => (showAdvanced = !showAdvanced)}
                            >{showAdvanced
                                ? "Скрыть advanced"
                                : "Показать advanced"}</button
                        >
                    </div>

        {#if showAdvanced}
            <div class="settings-block">
                <div class="small-head">Системный путь хранения</div>
                <div class="readonly-path">
                    {settingsPayload.storagePath || "не определён"}
                </div>
                <div class="field-hint">
                    Тут лежат `ray-player.db`, `ray-player.db-wal` и
                    `ray-player.db-shm`.
                </div>
            </div>

            <div class="settings-block">
                <div class="small-head">ONNX Runtime</div>
                <div class="path-picker-row">
                    <input
                        type="text"
                        bind:value={settingsPayload.onnxRuntimePath}
                        placeholder="/path/to/libonnxruntime.dylib"
                    />
                    <button
                        class="ghost-btn compact picker-btn"
                        type="button"
                        on:click={pickOnnxRuntime}>Выбрать</button
                    >
                </div>
                <div class="settings-actions-row split">
                    <button
                        class="ghost-btn compact"
                        type="button"
                        on:click={testRuntime}
                        disabled={isTestingRuntime}
                        >{isTestingRuntime
                            ? "Проверка…"
                            : "Проверить runtime"}</button
                    >
                </div>
                {#if runtimeTestResult}
                    <div
                        class:success-panel={runtimeTestResult.ok}
                        class:error-panel={!runtimeTestResult.ok}
                        class="panel-message"
                    >
                        <strong
                            >{runtimeTestResult.ok
                                ? "Runtime OK"
                                : "Runtime error"}</strong
                        >
                        <span>{runtimeTestResult.message}</span>
                        <small
                            >{runtimeTestResult.runtimePath ||
                                "путь не определён"} · {runtimeTestResult.latencyMS}
                            ms</small
                        >
                    </div>
                {/if}
            </div>

            <div class="settings-block">
                <div class="small-head">MiniLM</div>
                <div class="path-picker-row">
                    <input
                        type="text"
                        bind:value={settingsPayload.miniLMModelDir}
                        placeholder="/path/to/minilm dir"
                    />
                    <button
                        class="ghost-btn compact picker-btn"
                        type="button"
                        on:click={pickMiniLMDir}>Выбрать</button
                    >
                </div>
                <div class="settings-actions-row split">
                    <button
                        class="ghost-btn compact"
                        type="button"
                        on:click={testMiniLM}
                        disabled={isTestingMiniLM}
                        >{isTestingMiniLM
                            ? "Проверка…"
                            : "Проверить MiniLM"}</button
                    >
                </div>
                {#if miniLMTestResult}
                    <div
                        class:success-panel={miniLMTestResult.ok}
                        class:error-panel={!miniLMTestResult.ok}
                        class="panel-message"
                    >
                        <strong
                            >{miniLMTestResult.ok
                                ? "MiniLM OK"
                                : "MiniLM error"}</strong
                        >
                        <span>{miniLMTestResult.message}</span>
                        <small
                            >{miniLMTestResult.modelPath ||
                                miniLMTestResult.modelDir} · {miniLMTestResult.tokenizerPath ||
                                "tokenizer not found"} · {miniLMTestResult.embeddingDim ||
                                0} dim</small
                        >
                    </div>
                {/if}
            </div>

            <div class="settings-block">
                <div class="small-head">Essentia</div>
                <div class="path-picker-row">
                    <input
                        type="text"
                        bind:value={settingsPayload.essentiaModelDir}
                        placeholder="/path/to/essentia models"
                    />
                    <button
                        class="ghost-btn compact picker-btn"
                        type="button"
                        on:click={pickEssentiaDir}>Выбрать</button
                    >
                </div>
                <div class="settings-actions-row split">
                    <button
                        class="ghost-btn compact"
                        type="button"
                        on:click={testEssentia}
                        disabled={isTestingEssentia}
                        >{isTestingEssentia
                            ? "Проверка…"
                            : "Проверить Essentia"}</button
                    >
                </div>
                {#if essentiaTestResult}
                    <div
                        class:success-panel={essentiaTestResult.ok}
                        class:error-panel={!essentiaTestResult.ok}
                        class="panel-message"
                    >
                        <strong
                            >{essentiaTestResult.ok
                                ? "Essentia OK"
                                : "Essentia error"}</strong
                        >
                        <span>{essentiaTestResult.message}</span>
                        <small
                            >{essentiaTestResult.runtimePath ||
                                "runtime not found"} · {essentiaTestResult.modelDir ||
                                "dir not found"} · {essentiaTestResult.latencyMS}
                            ms</small
                        >
                    </div>
                {/if}

                <div class="settings-block nested">
                    <div class="small-head">FFmpeg</div>
                    <div class="path-picker-row">
                        <input
                            type="text"
                            bind:value={settingsPayload.ffmpegPath}
                            placeholder="ffmpeg"
                        />
                    </div>
                    <div class="settings-actions-row split">
                        <button
                            class="ghost-btn compact"
                            type="button"
                            on:click={testFFmpeg}
                            disabled={isTestingFFmpeg}
                            >{isTestingFFmpeg
                                ? "Проверка…"
                                : "Проверить ffmpeg"}</button
                        >
                    </div>
                    {#if ffmpegTestResult}
                        <div
                            class:success-panel={ffmpegTestResult.ok}
                            class:error-panel={!ffmpegTestResult.ok}
                            class="panel-message"
                        >
                            <strong
                                >{ffmpegTestResult.ok
                                    ? "ffmpeg OK"
                                    : "ffmpeg error"}</strong
                            >
                            <span>{ffmpegTestResult.message}</span>
                            <small
                                >{ffmpegTestResult.runtimePath ||
                                    settingsPayload.ffmpegPath ||
                                    "ffmpeg"}</small
                            >
                        </div>
                    {/if}
                </div>

                <div class="field-hint">
                    Alt+D открывает этот diagnostics-блок сразу. Здесь видно
                    runtime, schema и shape моделей.
                </div>
                <div class="probe-stack">
                    <div
                        class:probe-ok={essentiaTestResult?.base?.loaded}
                        class:probe-bad={!essentiaTestResult?.base?.loaded}
                        class="probe-row"
                    >
                        <strong>Base</strong>
                        <span
                            >{essentiaTestResult?.base?.message ||
                                (essentiaTestResult?.base?.loaded
                                    ? "loaded"
                                    : "missing")}</span
                        >
                        {#if probeLabel(essentiaTestResult?.base)}<small
                                >{probeLabel(essentiaTestResult?.base)}</small
                            >{/if}
                    </div>
                    <div
                        class:probe-ok={essentiaTestResult?.genre?.loaded}
                        class:probe-bad={!essentiaTestResult?.genre?.loaded}
                        class="probe-row"
                    >
                        <strong>Genre</strong>
                        <span
                            >{essentiaTestResult?.genre?.message ||
                                (essentiaTestResult?.genre?.loaded
                                    ? "loaded"
                                    : "missing")}</span
                        >
                        {#if probeLabel(essentiaTestResult?.genre)}<small
                                >{probeLabel(essentiaTestResult?.genre)}</small
                            >{/if}
                    </div>
                    <div class="probe-list">
                        {#each essentiaTestResult?.heads || [] as head}
                            <div
                                class:probe-ok={head.loaded}
                                class:probe-bad={!head.loaded}
                                class="probe-row probe-row-tight"
                            >
                                <strong>{head.name}</strong>
                                <span
                                    >{head.message ||
                                        (head.loaded
                                            ? "loaded"
                                            : "missing")}</span
                                >
                                {#if probeLabel(head)}<small
                                        >{probeLabel(head)}</small
                                    >{/if}
                            </div>
                        {/each}
                    </div>
                </div>
            </div>

            <div class="settings-block debug-block">
                <div class="small-head">Debug rebuild</div>
                <div class="field-hint">
                    Фоновый пересчёт только stale/missing анализа: Essentia,
                    embedding и связанные поля.
                </div>
                <button
                    class="ghost-btn compact debug-btn"
                    type="button"
                    on:click={debugReindex}
                    disabled={$reindexStatus.active}
                    >{$reindexStatus.active
                        ? `Идёт пересчёт ${$reindexStatus.index}/${$reindexStatus.total}`
                        : "Пересчитать stale анализ"}</button
                >
                {#if $reindexStatus.active || $reindexStatus.message}
                    <div
                        class:success-panel={$reindexStatus.state === "ok"}
                        class:error-panel={$reindexStatus.state === "error"}
                        class="panel-message"
                    >
                        <strong
                            >{$reindexStatus.state === "error"
                                ? "Reindex error"
                                : $reindexStatus.active
                                  ? "Reindex running"
                                  : "Reindex done"}</strong
                        >
                        <span>{$reindexStatus.message}</span>
                        <small
                            >{$reindexStatus.stage || "stage"} · {$reindexStatus.index ||
                                0}/{$reindexStatus.total || 0}</small
                        >
                    </div>
                {/if}
            </div>
        {/if}

        <div class="settings-block">
            <div class="small-head">Внешние инструменты</div>
            <div class="field-hint">
                yt-dlp используется только для импорта ссылок.
            </div>
            <label class="settings-field">
                <span>Путь к yt-dlp</span>
                <input bind:value={externalSettings.ytDlpPath} placeholder="yt-dlp" />
            </label>
            <label class="settings-field">
                <span>Путь к ffmpeg</span>
                <input bind:value={externalSettings.ffmpegPath} placeholder="ffmpeg или каталог с ffmpeg" />
            </label>
            <label class="settings-field">
                <span>Папка загрузок</span>
                <input bind:value={externalSettings.ytDlpDownloadDir} placeholder="По умолчанию: ~/Downloads" />
            </label>
            <div class="settings-tool-actions">
                <button type="button" class="ghost-btn compact" on:click={checkYtDlp} disabled={ytDlpChecking}>
                    {ytDlpChecking ? "Проверка…" : "Проверить yt-dlp"}
                </button>
                <button type="button" class="ghost-btn compact" on:click={saveExternalSettings} disabled={externalSettingsSaving}>
                    {externalSettingsSaving ? "Сохранение…" : "Сохранить"}
                </button>
            </div>
            {#if ytDlpCheck}
                <div class:success-panel={ytDlpCheck.ok} class:error-panel={!ytDlpCheck.ok} class="panel-message">
                    <strong>{ytDlpCheck.ok ? `yt-dlp OK · ${ytDlpCheck.version}` : "yt-dlp error"}</strong>
                    <span>{ytDlpCheck.ok ? (ytDlpCheck.output || ytDlpCheck.version || "") : ytDlpCheck.error}</span>
                </div>
            {/if}
        </div>

        <div class="settings-block">
            <div class="small-head">Подкасты</div>
            <SettingsSwitch
                title="Нормализация громкости"
                description="Выравнивает тихую и громкую речь. Целевой уровень −16 LUFS, максимальный пик −1.5 dB."
                checked={settingsPayload.normalizePodcastVolume || false}
                on:change={async (event) => {
                    const enabled = event.detail.checked;
                    settingsPayload = { ...settingsPayload, normalizePodcastVolume: enabled };
                    try {
                        const next = await api.setNormalizePodcastVolume(enabled);
                        if (next) settingsPayload = { ...settingsPayload, ...next };
                    } catch (err) {
                        console.error('normalize podcast', err);
                    }
                }}
            />
            <p class="settings-note">При изменении во время воспроизведения поток переоткрывается с сохранением позиции.</p>
        </div>

        <div class="settings-block">
            <div class="small-head">Интерфейс · EmoFlow UI</div>
            <SettingsSwitch
                title="Эмоциональные цвета"
                checked={settingsPayload.emoFlowUi.enabled}
                on:change={(event) => { settingsPayload.emoFlowUi.enabled = event.detail.checked; }}
            />
            <SettingsSwitch
                title="Плавная анимация внутри трека"
                checked={settingsPayload.emoFlowUi.animateDuringTrack}
                on:change={(event) => { settingsPayload.emoFlowUi.animateDuringTrack = event.detail.checked; }}
            />
            <SettingsSwitch
                title="Уважать reduced motion"
                checked={settingsPayload.emoFlowUi.respectReducedMotion}
                on:change={(event) => { settingsPayload.emoFlowUi.respectReducedMotion = event.detail.checked; }}
            />
            <UISlider id="emoflow-intensity" label="EmoFlow influence" value={emoFlowIntensityPercent} min={0} max={100} step={1} on:input={(e) => { setEmoFlowIntensityPercent(Number(e.detail) || 0); }} />
        </div>

                </div>
            </div>

            <footer class="settings-footer">
                <button
                    type="button"
                    class="secondary-button ghost-btn compact"
				on:click={() => { showDoctor = false; showSettings = false; }}
				>Закрыть</button
                >

                <button
                    type="button"
                    class="primary-button save-btn"
                    on:click={saveSettings}
                    >Сохранить и применить</button
                >
            </footer>
        </section>
    </div>
{/if}

{#if showDoctor}
	<DoctorModal
		settings={settingsPayload}
		on:close={() => (showDoctor = false)}
		on:patch={(event) => applyDoctorPatch(event.detail)}
	/>
{/if}

<AddLinkModal
    open={addLinkOpen}
    libraryType={libraryMode}
    submitting={addLinkSubmitting}
    error={addLinkError}
    on:close={closeAddLinkModal}
    on:submit={submitExternalLink}
/>

{#if isDragging}<div class="drop-overlay"><div class="drop-card"><div class="drop-title">Перетащите аудиофайлы или ссылку</div><div class="drop-subtitle">Ссылка будет загружена через yt-dlp в текущую библиотеку</div></div></div>{/if}
{#if $toast}<div class={`toast ${$toast.type || "info"}`}><strong>{$toast.title}</strong><span>{$toast.message}</span></div>{/if}
