export function podcastMeta(item: { series?: string; author?: string; folder?: string } | null | undefined): string {
	if (!item) return "";
	return [item.series || item.author, item.folder].filter(Boolean).join(" · ");
}

export function podcastProgress(item: {
	completedRatio?: number;
	lastPosition?: number;
	duration?: number;
} | null | undefined): number {
	if (!item) {
		return 0;
	}

	const stored = Number(item.completedRatio);
	if (Number.isFinite(stored) && stored > 0) {
		return Math.max(0, Math.min(1, stored));
	}

	const position = Number(item.lastPosition) || 0;
	const duration = Number(item.duration) || 0;
	if (duration <= 0) {
		return 0;
	}

	return Math.max(0, Math.min(1, position / duration));
}

export function podcastProgressPercent(item: Parameters<typeof podcastProgress>[0]): number {
	return Math.round(podcastProgress(item) * 100);
}
