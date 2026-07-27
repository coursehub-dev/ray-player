import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

import { hasPlaybackSelection, resolvePlayerTitle, resolveVisualMode } from "./playerUi.ts";

test("visual mode follows selected library, not currently playing media", () => {
	assert.equal(resolveVisualMode("podcast"), "podcast");
	assert.equal(resolveVisualMode("music"), "music");
	assert.equal(resolveVisualMode("unexpected"), "music");
});

test("empty player gives an actionable library hint", () => {
	assert.equal(
		resolvePlayerTitle({ libraryMode: "music", playback: {} }),
		"Выберите аудио в библиотеке, чтобы начать воспроизведение",
	);
	assert.equal(
		resolvePlayerTitle({ libraryMode: "podcast", playback: {} }),
		"Выберите выпуск в библиотеке, чтобы начать воспроизведение",
	);
});

test("selected media title wins over the empty-state hint", () => {
	assert.equal(
		resolvePlayerTitle({
			libraryMode: "music",
			playback: { currentTrackId: "track-1", currentTitle: "Track" },
		}),
		"Track",
	);
	assert.equal(
		resolvePlayerTitle({
			libraryMode: "podcast",
			playback: { currentTrackId: "podcast-1" },
			currentPodcast: { id: "podcast-1", title: "Episode" },
		}),
		"Episode",
	);
});

test("playback selection is false until real media is selected", () => {
	assert.equal(hasPlaybackSelection({}, null), false);
	assert.equal(hasPlaybackSelection({ currentTrackId: "track-1" }), true);
	assert.equal(hasPlaybackSelection({}, { id: "podcast-1" }), true);
});

test("podcast mode never paints generic accent-reactive containers", () => {
	const css = fs.readFileSync(new URL("../style.css", import.meta.url), "utf8");
	assert.doesNotMatch(css, /\.mode-podcast\s+\.accent-reactive\s*\{/);
});
