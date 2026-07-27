import { api } from "../../shared/api";
import { syncPayload } from "../../app";

export async function playPodcastItem(itemId: string, fromRay = false) {
	const payload = fromRay ? await api.playPodcastRayItem(itemId) : await api.playPodcast(itemId);
	return syncPayload(Promise.resolve(payload));
}

export async function nextPodcast() {
	return syncPayload(api.nextPodcast());
}

export async function previousPodcast() {
	return syncPayload(api.previousPodcast());
}

export async function openPodcastRayHistory(rayId: string) {
	return syncPayload(api.openPodcastRayHistory(rayId));
}

export async function setPodcastRayContentMode(mode: string) {
	return syncPayload(api.setPodcastRayContentMode(mode));
}

export async function setPodcastRaySortMode(mode: string) {
	return syncPayload(api.setPodcastRaySortMode(mode));
}

export async function movePodcastRayItem(from: number, to: number) {
	return syncPayload(api.movePodcastRayItem(from, to));
}

export async function removePodcastRayItem(itemId: string) {
	return syncPayload(api.removePodcastRayItem(itemId));
}
