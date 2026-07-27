import { writable, get } from "svelte/store";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { api } from "../lib/api";
import { emoFlowState, syncEmoFlowFromPayload } from "./emoflow";
import { isPodcastItemId } from "../lib/mediaIdentity";
import { bindExternalDownloadEvents, unbindExternalDownloadEvents } from "./externalDownloads";
import { playbackState, syncPlayback } from "../entities/playback";

export const screen = writable("search");
export const state = writable({
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
});
export { playbackState };
export const rayBuildState = writable({
	status: "idle",
	seedTrackId: "",
	requestId: 0,
	startedAt: 0,
	finishedAt: 0,
	lastError: "",
});
export const selectedTrackId = writable("");
export const searchQuery = writable("");
export const searchResults = writable([]);
export const reindexStatus = writable({
	active: false,
	index: 0,
	total: 0,
	stage: "",
	state: "",
	message: "",
	trackId: "",
	path: "",
});
export const indexingState = writable({
	isIndexing: false,
	libraryCount: 0,
	processed: 0,
	total: 0,
	queued: 0,
	phase: "idle",
	currentPath: "",
});
export const toast = writable(null);
export { emoFlowState } from "./emoflow";

let snapshotBound = false;
let reindexBound = false;
let timer = null;

function showToast(payload) {
	toast.set(payload);
	if (timer) clearTimeout(timer);
	timer = setTimeout(() => toast.set(null), payload?.duration || 3200);
}

function syncRayBuild(payload) {
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

export async function syncPayload(promise) {
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

export async function runSearch(query) {
	searchQuery.set(query);
	const result = await api.searchTracks(query);
	searchResults.set(Array.isArray(result) ? result : []);
}

export function bindSnapshotEvents() {
	if (snapshotBound || !globalThis?.window?.runtime?.EventsOn) {
		return;
	}
	snapshotBound = true;
	EventsOn("app:snapshot", (payload) => {
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
	EventsOn("playback:update", (payload) => {
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
	EventsOn("ray:build-state", (payload) => {
		if (payload?.status) {
			rayBuildState.set(payload);
		}
	});
	EventsOn("emoflow:update", (payload) => {
		if (payload) {
			emoFlowState.set(payload);
		}
	});
	EventsOn("library:analyzed", async () => {
		await runSearch(get(searchQuery));
	});
	EventsOn("indexing:update", (payload) => {
		if (payload) indexingState.set(payload);
	});
	EventsOn("import:result", (r) => {
		showToast({
			type: "success",
			title: `${r.added || 0} tracks added`,
			message: `${r.audioFound || 0} audio found · ${r.alreadyPresent || 0} already in library`,
			duration: 3200,
		});
	});
	EventsOn("playback:failed", (failure) => {
		showToast({
			type: "error",
			title: `Skipped unavailable track`,
			message: failure?.title || failure?.path || "Playback failed",
			duration: 3200,
		});
	});
	EventsOn("emoflow:technical_skip", (payload) => {
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
	if (reindexBound || !globalThis?.window?.runtime?.EventsOn) {
		return;
	}
	reindexBound = true;
	EventsOn("app:reindex:progress", (payload) => {
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
	EventsOn("app:reindex:done", async (payload) => {
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
	if (snapshotBound && globalThis?.window?.runtime?.EventsOff) {
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
	if (reindexBound && globalThis?.window?.runtime?.EventsOff) {
		EventsOff("app:reindex:progress");
		EventsOff("app:reindex:done");
		reindexBound = false;
	}
	unbindExternalDownloadEvents();
}
