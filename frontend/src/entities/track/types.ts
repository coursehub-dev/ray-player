export type Track = {
	id: string;
	title?: string;
	fileName?: string;
	artist?: string;
	genreLabel?: string;
	genrePrimary?: string;
	genreTags?: unknown[];
	durationMs?: number;
	durationLabel?: string;
	folder?: string;
	bpmPerceived?: number;
	tempo?: number;
	tempoConfidence?: number;
	sourceType?: string;
	analysisStatus?: string;
};

export type TrackSearchRow = {
	track: Track;
	score?: number;
};
