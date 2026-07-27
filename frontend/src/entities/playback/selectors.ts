import type { PlaybackState } from "./model";

export function isCurrentTrack(playback: PlaybackState | null | undefined, trackId: string): boolean {
	return Boolean(trackId) && trackId === playback?.currentTrackId;
}

export function isCurrentPlaying(playback: PlaybackState | null | undefined, trackId: string): boolean {
	return isCurrentTrack(playback, trackId) && playback?.status === "playing";
}

export function getTrackPlaybackUI(playback: PlaybackState | null | undefined, trackId: string) {
	const current = isCurrentTrack(playback, trackId);
	return {
		isPlayingTrack: current,
		isRaySeed: trackId === playback?.raySeedTrackId && Boolean(playback?.rayId),
		isActuallyPlaying: current && playback?.status === "playing",
		isPausedCurrent: current && playback?.status === "paused",
		isLoadingCurrent: current && playback?.status === "loading",
	};
}
