export type LibraryMode = "music" | "podcast";

export const normalizeLibraryMode = (mode: unknown): LibraryMode =>
	mode === "podcast" ? "podcast" : "music";

// Visual chrome follows the library the user is browsing. Playback kind is
// intentionally separate so a still-playing podcast cannot pin the UI in
// podcast mode after the user switches back to music.
export const resolveVisualMode = (libraryMode: unknown): LibraryMode =>
	normalizeLibraryMode(libraryMode);

export const hasPlaybackSelection = (
	playback: { currentTrackId?: string } | null | undefined,
	currentPodcast: { id?: string } | null = null,
): boolean => Boolean(currentPodcast?.id || String(playback?.currentTrackId || "").trim());

export const resolvePlayerTitle = ({
	libraryMode,
	playback,
	currentPodcast,
}: {
	libraryMode: unknown;
	playback?: { currentTrackId?: string; currentTitle?: string } | null;
	currentPodcast?: { id?: string; title?: string } | null;
}): string => {
	const title = hasPlaybackSelection(playback, currentPodcast)
		? String(currentPodcast?.title || playback?.currentTitle || "").trim()
		: "";
	if (title) {
		return title;
	}

	return normalizeLibraryMode(libraryMode) === "podcast"
		? "Выберите выпуск в библиотеке, чтобы начать воспроизведение"
		: "Выберите аудио в библиотеке, чтобы начать воспроизведение";
};
