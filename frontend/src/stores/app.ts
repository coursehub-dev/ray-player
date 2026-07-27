import { writable, get } from "svelte/store";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { api } from "../shared/api";
import { emoFlowState, syncEmoFlowFromPayload } from "./emoflow";
import { isPodcastItemId } from "../entities/podcast";
import { bindExternalDownloadEvents, unbindExternalDownloadEvents } from "./externalDownloads";
import { playbackState, syncPlayback, type PlaybackState } from "../entities/playback";

/** App snapshot from Go — kept intentionally loose during gradual typing. */
export type AppSnapshot = Record<string, any>;

export type ToastPayload = {
	type?: string;
	kind?: string;
	title?: string;
	message?: string;
	duration?: number;
} | null;

export type RayBuildState = {
	status: string;
	seedTrackId: string;
	requestId: number;
	startedAt: number;
	finishedAt: number;
	lastError: string;
};

export type IndexingState = {
	isIndexing: boolean;
	libraryCount: number;
	processed: number;
	total: number;
	queued: number;
	phase: string;
	currentPath: string;
};

export type ReindexStatus = {
	active: boolean;
	index: number;
	total: number;
	stage: string;
	state: string;
	message: string;
	trackId: string;
	path: string;
};

const initialSnapshot: AppSnapshot = {
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
	podcastPlayback: { itemId: "", rayId: "", queueIndex: -1, queueLength: 0 },
	podcastHistory: [],
	podcastRays: [],
	current: { volume: 0.58, queue: [] },
	history: [],
	rays: [],
	queue: [],
	libraryStat: { tracks: 0 },
};

export const screen = writable("search");
export const state = writable<AppSnapshot>({ ...initialSnapshot });
export { playbackState };
export const rayBuildState = writable<RayBuildState>({
	status: "idle",
	seedTrackId: "",
	requestId: 0,
	startedAt: 0,
	finishedAt: 0,
	lastError: "",
});
export const selectedTrackId = writable("");
export const searchQuery = writable("");
export const searchResults = writable<unknown[]>([]);
export const reindexStatus = writable<ReindexStatus>({
	active: false,
	index: 0,
	total: 0,
	stage: "",
	state: "",
	message: "",
	trackId: "",
	path: "",
});
export const indexingState = writable<IndexingState>({
	isIndexing: false,
	libraryCount: 0,
	processed: 0,
	total: 0,
	queued: 0,
	phase: "idle",
	currentPath: "",
});
export const toast = writable<ToastPayload>(null);
export { emoFlowState } from "./emoflow";

let snapshotBound = false;
let reindexBound = false;
let timer: ReturnType<typeof setTimeout> | null = null;

function showToast(payload: Exclude<ToastPayload, null>) {
	toast.set(payload);
	if (timer) clearTimeout(timer);
	timer = setTimeout(() => toast.set(null), payload?.duration || 3200);
}

function syncRayBuild(payload: any) {
	const build = payload?.rayBuild || payload;
	if (build?.status) {
		rayBuildState.set(build);
	}
}

export async function bootstrap() {
	const payload = await api.bootstrap();
	if (payload?.library) {
		state.set(payload);
		syncPlayback(payload);
		syncRayBuild(payload);
		syncEmoFlowFromPayload(payload);
		indexingState.update((prev) => ({
			...prev,
			libraryCount: payload.libraryStat?.tracks || payload.library?.length || 0,
		}));
	}

	bindSnapshotEvents();
	bindReindexEvents();
	bindExternalDownloadEvents();

	const playback = await api.getPlaybackState();
	syncPlayback(playback);
	await runSearch(get(searchQuery));
}

export async function syncPayload(promise: Promise<any>) {
	const payload = await promise;
	if (payload?.library) {
		state.set(payload);
		syncPlayback(payload);
		syncRayBuild(payload);
		syncEmoFlowFromPayload(payload);
		indexingState.update((prev) => ({
			...prev,
			libraryCount: payload.libraryStat?.tracks || payload.library?.length || 0,
		}));
		await runSearch(get(searchQuery));
	}
	return payload;
}

export async function runSearch(query: string) {
	searchQuery.set(query);
	const result = await api.searchTracks(query);
	searchResults.set(Array.isArray(result) ? result : []);
}

export function bindSnapshotEvents() {
	if (snapshotBound || !(globalThis as any)?.window?.runtime?.EventsOn) {
		return;
	}
	snapshotBound = true;
	EventsOn("app:snapshot", (payload: any) => {
		if (payload?.library) {
			state.set(payload);
			syncPlayback(payload);
			syncRayBuild(payload);
			syncEmoFlowFromPayload(payload);
			indexingState.update((prev) => ({
				...prev,
				libraryCount: payload.libraryStat?.tracks || payload.library?.length || 0,
			}));
		}
	});
	EventsOn("playback:update", (payload: PlaybackState) => {
		if (!payload?.status) return;

		const current = get(playbackState);
		const currentID = String(current.currentTrackId || "");
		const incomingID = String(payload.currentTrackId || "");

		if (
			isPodcastItemId(currentID) &&
			current.status === "playing" &&
			incomingID !== "" &&
			!isPodcastItemId(incomingID)
		) {
			return;
		}

		playbackState.set(payload);
	});
	EventsOn("ray:build-state", (payload: RayBuildState) => {
		if (payload?.status) {
			rayBuildState.set(payload);
		}
	});
	EventsOn("emoflow:update", (payload: any) => {
		if (payload) {
			emoFlowState.set(payload);
		}
	});
	EventsOn("library:analyzed", async () => {
		await runSearch(get(searchQuery));
	});
	EventsOn("indexing:update", (payload: IndexingState) => {
		if (payload) indexingState.set(payload);
	});
	EventsOn("import:result", (r: any) => {
		showToast({
			type: "success",
			title: `${r.added || 0} tracks added`,
			message: `${r.audioFound || 0} audio found · ${r.alreadyPresent || 0} already in library`,
			duration: 3200,
		});
	});
	EventsOn("playback:failed", (failure: any) => {
		showToast({
			type: "error",
			title: `Skipped unavailable track`,
			message: failure?.title || failure?.path || "Playback failed",
			duration: 3200,
		});
	});
	EventsOn("emoflow:technical_skip", (payload: any) => {
		showToast({
			type: "warning",
			title: "Не удалось воспроизвести файл",
			message: payload?.title ? `${payload.title} — пробую следующий` : "Пробую следующий трек",
			duration: 2800,
		});
	});
	EventsOn("queue:updated", async () => {
		await syncPayload(api.snapshot());
	});
}

export function bindReindexEvents() {
	if (reindexBound || !(globalThis as any)?.window?.runtime?.EventsOn) {
		return;
	}
	reindexBound = true;
	EventsOn("app:reindex:progress", (payload: any) => {
		if (!payload) return;
		reindexStatus.set({
			active: true,
			index: payload.index || 0,
			total: payload.total || 0,
			stage: payload.stage || "",
			state: payload.state || "",
			message: payload.message || "",
			trackId: payload.trackId || "",
			path: payload.path || "",
		});
	});
	EventsOn("app:reindex:done", async (payload: any) => {
		reindexStatus.set({
			active: false,
			index: payload?.total || 0,
			total: payload?.total || 0,
			stage: "done",
			state: payload?.ok ? "ok" : "error",
			message: payload?.message || (payload?.ok ? "Reindex done" : "Reindex failed"),
			trackId: "",
			path: "",
		});
		await runSearch(get(searchQuery));
	});
}

export function unbindSnapshotEvents() {
	if (snapshotBound && (globalThis as any)?.window?.runtime?.EventsOff) {
		for (const name of [
			"app:snapshot",
			"playback:update",
			"ray:build-state",
			"library:analyzed",
			"emoflow:update",
			"indexing:update",
			"import:result",
			"playback:failed",
			"emoflow:technical_skip",
			"queue:updated",
		]) {
			EventsOff(name);
		}
		snapshotBound = false;
	}
	if (reindexBound && (globalThis as any)?.window?.runtime?.EventsOff) {
		EventsOff("app:reindex:progress");
		EventsOff("app:reindex:done");
		reindexBound = false;
	}
	unbindExternalDownloadEvents();
}
