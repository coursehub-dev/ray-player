/**
 * App composition root: snapshot, screen, toast, bootstrap/sync, Wails events.
 * Domain stores live in entities/features — import those directly.
 */
export type {
	AppSnapshot,
	IndexingState,
	ReindexStatus,
	ToastPayload,
} from "./types";

export {
	indexingState,
	reindexStatus,
	runSearch,
	screen,
	searchQuery,
	searchResults,
	selectedTrackId,
	state,
	syncPayload,
	toast,
} from "./store";

export {
	bindReindexEvents,
	bindSnapshotEvents,
	bootstrap,
	unbindSnapshotEvents,
} from "./events";
