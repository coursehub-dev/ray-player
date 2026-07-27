import { writable } from "svelte/store";
import { initialPlaybackState, type PlaybackState } from "./model";

export const playbackState = writable<PlaybackState>({ ...initialPlaybackState });

/** Sync playback store from snapshot payload or a bare playback object. */
export function syncPlayback(payload: unknown): void {
	const source = payload as { current?: PlaybackState; status?: string } | null | undefined;
	const playback = source?.current || (source as PlaybackState | null | undefined);
	if (playback?.status) {
		playbackState.set(playback as PlaybackState);
	}
}
