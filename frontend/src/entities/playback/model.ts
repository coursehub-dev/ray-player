export type PlaybackStatus = "stopped" | "playing" | "paused" | "loading" | "error" | string;

export type PlaybackState = {
	status: PlaybackStatus;
	currentTrackId: string;
	currentPath?: string;
	positionMs: number;
	durationMs: number;
	queueId: string;
	queueIndex: number;
	queueLength: number;
	rayId: string;
	raySeedTrackId: string;
	updatedAt?: number;
	lastError: string;
	currentTitle?: string;
	currentArtist?: string;
	currentSub?: string;
	currentGenre?: string;
};

export const initialPlaybackState: PlaybackState = {
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
