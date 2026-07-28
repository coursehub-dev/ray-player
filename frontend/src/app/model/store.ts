import { writable, get } from "svelte/store";
import { api } from "../../shared/api";
import { syncEmoFlowFromPayload } from "../../entities/emoflow";
import { syncRayBuild } from "../../entities/ray";
import { syncPlayback } from "../../entities/playback";
import type { AppSnapshot, IndexingState, ReindexStatus, ToastPayload } from "./types";

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
	savedPodcastRayIds: [],
	current: { volume: 0.58, queue: [] },
	history: [],
	rays: [],
	savedRays: [],
	queue: [],
	libraryStat: { tracks: 0 },
	libraryMode: "music",
	resumeSession: { available: false },
};

export const screen = writable("search");

export const state = writable<AppSnapshot>({ ...initialSnapshot });

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

let timer: ReturnType<typeof setTimeout> | null = null;

export function showToast(payload: Exclude<ToastPayload, null>) {
	toast.set(payload);
	if (timer) clearTimeout(timer);
	timer = setTimeout(() => toast.set(null), payload?.duration || 3200);
}

export function applySnapshot(payload: AppSnapshot) {
	state.set(payload);
	syncPlayback(payload);
	syncRayBuild(payload);
	syncEmoFlowFromPayload(payload);
	indexingState.update((prev) => ({
		...prev,
		libraryCount: payload.libraryStat?.tracks || payload.library?.length || 0,
	}));
}

export async function syncPayload(promise: Promise<any>) {
	const payload = await promise;
	if (payload?.library) {
		applySnapshot(payload);
		await runSearch(get(searchQuery));
	}
	return payload;
}

export async function runSearch(query: string) {
	searchQuery.set(query);
	const result = await api.searchTracks(query);
	searchResults.set(Array.isArray(result) ? result : []);
}
