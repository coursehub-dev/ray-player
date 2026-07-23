export function isPodcastItemId(value) {
	const id = String(value || "").trim();
	if (!id) return false;

	return (
		id.startsWith("podcast_") ||
		id.startsWith("external-podcast-") ||
		id.startsWith("podcast-external-")
	);
}
