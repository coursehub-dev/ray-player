export type AppSnapshot = Record<string, any>;

export type ToastPayload = {
	type?: string;
	kind?: string;
	title?: string;
	message?: string;
	duration?: number;
} | null;

export type IndexingState = {
	isIndexing: boolean;
	libraryCount: number;
	processed: number;
	total: number;
	queued: number;
	phase: string;
	currentPath: string;
};

export type ReindexStatus = {
	active: boolean;
	index: number;
	total: number;
	stage: string;
	state: string;
	message: string;
	trackId: string;
	path: string;
};
