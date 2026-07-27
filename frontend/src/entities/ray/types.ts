export type MusicRaySummary = {
	id: string;
	name?: string;
	seedTrackId?: string;
	contentMode?: string;
	sortMode?: string;
	isManualOrder?: boolean;
	active?: boolean;
	trackCount?: number;
	currentTrackName?: string;
	resumeLabel?: string;
};

export type QueueItem = {
	trackId: string;
	track?: unknown;
	position?: number;
	rayRole?: string;
	role?: string;
	rayReason?: string;
	reason?: string;
	artist?: string;
	genreLabel?: string;
	genrePrimary?: string;
	genreTags?: unknown[];
};

export type PodcastRaySummary = {
	id: string;
	title?: string;
	seedItemId?: string;
	folderScope?: string;
	contentMode?: string;
	sortMode?: string;
	isManualOrder?: boolean;
	itemCount?: number;
	revision?: number;
	parentRayId?: string;
	createdAtLabel?: string;
	seed?: {
		title?: string;
		series?: string;
		author?: string;
	};
};
