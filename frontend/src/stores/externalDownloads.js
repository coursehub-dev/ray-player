import { writable } from "svelte/store";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

export const externalDownloads = writable(new Map());

let bound = false;
const eventNames = [
	"external-download:update",
	"external-download:done",
	"external-download:error",
	"external-download:canceled",
];

function updateJob(job) {
	if (!job?.itemId) return;

	externalDownloads.update((current) => {
		const next = new Map(current);
		next.set(job.itemId, job);
		return next;
	});
}

export function bindExternalDownloadEvents() {
	if (bound) return;
	if (!globalThis?.window?.runtime?.EventsOn) return;
	bound = true;
	for (const eventName of eventNames) {
		EventsOn(eventName, updateJob);
	}
}

export function unbindExternalDownloadEvents() {
	if (!bound) return;
	bound = false;
	for (const eventName of eventNames) {
		EventsOff(eventName);
	}
}

export function putExternalDownload(job) {
	updateJob(job);
}

export function externalJobFor(map, item) {
	return map?.get?.(item?.id) || null;
}

export function mergedDownloadState(map, item) {
	const job = externalJobFor(map, item);
	return {
		status: job?.status || item?.downloadStatus || "ready",
		progress: Number(job?.progress) || Number(item?.downloadProgress) || 0,
		error: job?.error || item?.downloadError || "",
	};
}
