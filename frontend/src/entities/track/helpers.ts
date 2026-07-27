import type { Track } from "./types";

export function genreBadge(track: Pick<Track, "genreLabel" | "genrePrimary"> | null | undefined): string {
	return track?.genreLabel || track?.genrePrimary || "";
}

export function findTrackById(library: Track[] | null | undefined, trackId: string | null | undefined): Track | null {
	if (!trackId) return null;
	return (library || []).find((track) => track.id === trackId) || null;
}
