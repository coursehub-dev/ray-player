import { get } from "svelte/store";
import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";
import { api } from "../../shared/api";
import { emoFlowState } from "../../entities/emoflow";
import { isPodcastItemId } from "../../entities/podcast";
import { rayBuildState, type RayBuildState } from "../../entities/ray";
import {
	bindExternalDownloadEvents,
	unbindExternalDownloadEvents,
} from "../../features/external-link";
import { playbackState, syncPlayback, type PlaybackState } from "../../entities/playback";
import type { IndexingState } from "./types";
import {
	applySnapshot,
	indexingState,
	reindexStatus,
	runSearch,
	searchQuery,
	showToast,
	syncPayload,
} from "./store";

let snapshotBound = false;
let reindexBound = false;

export async function bootstrap() {
	const payload = await api.bootstrap();
	if (payload?.library) {
		applySnapshot(payload);
	}

	bindSnapshotEvents();
	bindReindexEvents();
	bindExternalDownloadEvents();

	const playback = await api.getPlaybackState();
	syncPlayback(playback);
	await runSearch(get(searchQuery));
}

export function bindSnapshotEvents() {
	if (snapshotBound || !(globalThis as any)?.window?.runtime?.EventsOn) {
		return;
	}

	snapshotBound = true;

	EventsOn("app:snapshot", (payload: any) => {
		if (payload?.library) {
			applySnapshot(payload);
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
