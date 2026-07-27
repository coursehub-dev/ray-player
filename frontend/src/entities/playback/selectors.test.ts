import assert from "node:assert/strict";
import test from "node:test";

import type { PlaybackState } from "./model.ts";
import { getTrackPlaybackUI, isCurrentPlaying, isCurrentTrack } from "./selectors.ts";

test("selectors reflect current track playback", () => {
	const playback: PlaybackState = {
		status: "playing",
		currentTrackId: "t1",
		positionMs: 0,
		durationMs: 0,
		queueId: "",
		queueIndex: -1,
		queueLength: 0,
		rayId: "ray",
		raySeedTrackId: "t1",
		lastError: "",
	};

	assert.equal(isCurrentTrack(playback, "t1"), true);
	assert.equal(isCurrentPlaying(playback, "t1"), true);
	assert.equal(isCurrentPlaying(playback, "t2"), false);

	const ui = getTrackPlaybackUI(playback, "t1");
	assert.equal(ui.isActuallyPlaying, true);
	assert.equal(ui.isRaySeed, true);
	assert.equal(ui.isPausedCurrent, false);
});
