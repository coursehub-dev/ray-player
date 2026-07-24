export const normalizeLibraryMode = (mode) =>
	mode === "podcast" ? "podcast" : "music";

// Visual chrome follows the library the user is browsing. Playback kind is
// intentionally separate so a still-playing podcast cannot pin the UI in
// podcast mode after the user switches back to music.
export const resolveVisualMode = (libraryMode) =>
	normalizeLibraryMode(libraryMode);

export const hasPlaybackSelection = (playback, currentPodcast = null) =>
	Boolean(
		currentPodcast?.id || String(playback?.currentTrackId || "").trim(),
	);

export const resolvePlayerTitle = ({
	libraryMode,
	playback,
	currentPodcast,
}) => {
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
