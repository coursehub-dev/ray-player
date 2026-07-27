import { writable } from "svelte/store";
import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";

export type ExternalDownloadJob = {
	itemId: string;
	status?: string;
	progress?: number;
	error?: string;
	[key: string]: unknown;
};

export const externalDownloads = writable(new Map<string, ExternalDownloadJob>());

let bound = false;
const eventNames = [
	"external-download:update",
	"external-download:done",
	"external-download:error",
	"external-download:canceled",
] as const;

function updateJob(job: ExternalDownloadJob | null | undefined) {
	if (!job?.itemId) return;

	externalDownloads.update((current) => {
		const next = new Map(current);
		next.set(job.itemId, job);
		return next;
	});
}

export function bindExternalDownloadEvents() {
	if (bound) return;
	if (!(globalThis as any)?.window?.runtime?.EventsOn) return;
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

export function putExternalDownload(job: ExternalDownloadJob) {
	updateJob(job);
}

export function externalJobFor(
	map: Map<string, ExternalDownloadJob> | null | undefined,
	item: { id?: string } | null | undefined,
) {
	return map?.get?.(item?.id || "") || null;
}

export function mergedDownloadState(
	map: Map<string, ExternalDownloadJob> | null | undefined,
	item: { id?: string; downloadStatus?: string; downloadProgress?: number; downloadError?: string } | null | undefined,
) {
	const job = externalJobFor(map, item);
	return {
		status: job?.status || item?.downloadStatus || "ready",
		progress: Number(job?.progress) || Number(item?.downloadProgress) || 0,
		error: job?.error || item?.downloadError || "",
	};
}
