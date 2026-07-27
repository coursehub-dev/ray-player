export type PodcastItem = {
	id: string;
	title?: string;
	author?: string;
	series?: string;
	folder?: string;
	duration?: number;
	durationLabel?: string;
	lastPosition?: number;
	resumePosition?: number;
	completedRatio?: number;
	isCompleted?: boolean;
	sourceType?: string;
	semanticStatus?: string;
	analysisStatus?: string;
};

export type PodcastHistoryEntry = {
	item: PodcastItem;
	source?: string;
	rayId?: string;
	playedAtLabel?: string;
	listenedLabel?: string;
	positionLabel?: string;
	progressPercent?: number;
};
