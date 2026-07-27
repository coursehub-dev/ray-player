import { get } from "svelte/store";
import { api } from "../../lib/api";
import { playbackState, type PlaybackState } from "../../entities/playback";
import { state, syncPayload } from "../../stores/app";

type PlaybackPatch = Partial<PlaybackState> & Record<string, unknown>;

function patchAppCurrent(patch: Record<string, unknown>): void {
	state.update((prev) => ({
		...prev,
		current: { ...prev.current, ...patch },
	}));
}

/** Merge a playback/current patch into playback store + app snapshot current. */
export function applyPlaybackPatch(patch: PlaybackPatch | null | undefined): void {
	if (!patch) return;

	if (patch.status) {
		playbackState.set({ ...get(playbackState), ...patch } as PlaybackState);
	}

	patchAppCurrent(patch);
}

export async function togglePause(): Promise<PlaybackPatch | null> {
	if (get(playbackState).status === "loading") {
		return null;
	}

	const next = (await api.togglePlay()) as PlaybackPatch;
	applyPlaybackPatch(next);
	return next;
}

export async function seekTo(positionMs: number, options?: { stableDurationMs?: number }): Promise<PlaybackPatch> {
	const next = (await api.seek(positionMs)) as PlaybackPatch;
	const stableDurationMs = options?.stableDurationMs ?? 0;
	const prevCurrent = get(state).current as { durationMs?: number } | undefined;
	const durationMs = Number(next?.durationMs || prevCurrent?.durationMs || stableDurationMs) || 0;

	patchAppCurrent({ ...next, durationMs });
	return next;
}

export async function setVolumeLevel(volume: number): Promise<PlaybackPatch | null> {
	const value = Math.max(0, Math.min(1, Number(volume) || 0));
	const next = (await api.setVolume(value)) as PlaybackPatch | null;
	if (next) {
		patchAppCurrent(next);
	}
	return next;
}

export async function toggleMute(): Promise<PlaybackPatch | null> {
	const next = (await api.toggleMute()) as PlaybackPatch | null;
	if (next) {
		patchAppCurrent(next);
	}
	return next;
}

export async function nextTrack() {
	return syncPayload(api.nextTrack());
}

export async function previousTrack() {
	return syncPayload(api.previousTrack());
}
