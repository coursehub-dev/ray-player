import assert from "node:assert/strict";
import test from "node:test";

import { podcastMeta, podcastProgress, podcastProgressPercent } from "./helpers.ts";
import { podcastHistorySourceLabel, podcastRayContentLabel } from "./labels.ts";

test("podcastMeta joins series/author and folder", () => {
	assert.equal(podcastMeta({ series: "Show", folder: "A" }), "Show · A");
	assert.equal(podcastMeta({ author: "Host" }), "Host");
	assert.equal(podcastMeta(null), "");
});

test("podcastProgress prefers completedRatio then position/duration", () => {
	assert.equal(podcastProgress({ completedRatio: 0.4 }), 0.4);
	assert.equal(podcastProgress({ lastPosition: 30, duration: 100 }), 0.3);
	assert.equal(podcastProgressPercent({ lastPosition: 30, duration: 100 }), 30);
	assert.equal(podcastProgress({}), 0);
});

test("podcast labels map known modes and sources", () => {
	assert.equal(podcastRayContentLabel("explore"), "Исследование");
	assert.equal(podcastRayContentLabel("unknown"), "Рекомендуемое");
	assert.equal(podcastHistorySourceLabel("ray"), "Из луча");
	assert.equal(podcastHistorySourceLabel("other"), "Ручной запуск");
});
