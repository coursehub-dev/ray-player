import { api } from "../../shared/api";
import { applyPlaybackPatch } from "../playback";
import { state, syncPayload, toast } from "../../app";
import { syncEmoFlowFromPayload } from "../../entities/emoflow";

/** Request play+ray build from backend (does not touch stores). */
export async function playTrackBuildingRay(trackId: string, mode: string) {
	return api.playTrackWithMode(trackId, mode);
}

/** Apply a successful playTrackWithMode snapshot into stores. */
export function applyPlayTrackPayload(payload: unknown) {
	const snapshot = payload as { library?: unknown; current?: { status?: string } } | null | undefined;
	if (!snapshot?.library) return;

	state.set(snapshot);
	if (snapshot.current?.status) {
		applyPlaybackPatch(snapshot.current);
	}
	syncEmoFlowFromPayload(snapshot);
}

export async function skipToQueueTrack(trackId: string) {
	return syncPayload(api.skipToTrackInQueue(trackId));
}

export async function resumeMusicRay(rayId: string) {
	return syncPayload(api.resumeRay(rayId));
}

export async function setMusicRayContentMode(mode: string) {
	return syncPayload(api.setMusicRayContentMode(mode));
}

export async function setMusicRaySortMode(mode: string) {
	return syncPayload(api.setMusicRaySortMode(mode));
}

export async function moveMusicQueueItem(trackId: string, newIndex: number) {
	return syncPayload(api.moveQueueItem(trackId, newIndex));
}

export async function removeFromMusicQueue(trackId: string) {
	return syncPayload(api.removeFromQueue(trackId));
}

export async function playTrackStartingRay(trackId: string, mode: string) {
	return syncPayload(api.playTrackWithMode(trackId, mode));
}

export function reportPlayRayError(error: unknown) {
	const message =
		error && typeof error === "object" && "message" in error
			? String((error as { message?: unknown }).message || "")
			: "";
	toast.set({
		kind: "error",
		message: message || "Не удалось построить луч",
	});
}
