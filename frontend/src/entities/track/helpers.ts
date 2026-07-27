export function genreBadge(track: { genreLabel?: string; genrePrimary?: string } | null | undefined): string {
	return track?.genreLabel || track?.genrePrimary || "";
}

export function findTrackById<T extends { id: string }>(
	library: T[] | null | undefined,
	trackId: string | null | undefined,
): T | null {
	if (!trackId) return null;
	return (library || []).find((track) => track.id === trackId) || null;
}
