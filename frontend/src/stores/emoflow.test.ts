import assert from "node:assert/strict";
import test from "node:test";

import { interpolateTrackPalette, mergeEmoFlowState } from "./emoflow.ts";

type Palette = {
	accent: string;
	accentSoft: string;
	accentHot: string;
	background: string;
	surface: string;
	glow: string;
	glowSoft: string;
	ring: string;
	progress: string;
	icon: string;
	accentOn: string;
};

function palette(hue: number): Palette {
	const opaque = `oklch(62% 0.180 ${hue})`;
	const translucent = `oklch(62% 0.180 ${hue} / 0.200)`;
	return {
		accent: opaque,
		accentSoft: opaque,
		accentHot: opaque,
		background: opaque,
		surface: opaque,
		glow: translucent,
		glowSoft: translucent,
		ring: translucent,
		progress: opaque,
		icon: opaque,
		accentOn: "white",
	};
}

test("new track starts in its own palette", () => {
	const current = palette(225);
	const next = palette(25);
	assert.equal(interpolateTrackPalette(current, next, 0).accent, current.accent);
	assert.equal(interpolateTrackPalette(current, next, 0.5).accent, current.accent);
});

test("outgoing track reaches incoming palette at natural end", () => {
	const current = palette(225);
	const next = palette(25);
	assert.equal(interpolateTrackPalette(current, next, 1).accent, next.accent);
	assert.notEqual(interpolateTrackPalette(current, next, 0.91).accent, current.accent);
});

test("authoritative null neighbours clear stale transition state", () => {
	const state = {
		current: { trackId: "old", palette: palette(25) },
		previous: { trackId: "older", palette: palette(40) },
		next: { trackId: "old-next", palette: palette(225) },
		palette: palette(25),
	};
	const merged = mergeEmoFlowState(state, {
		trackId: "new",
		current: { trackId: "new", palette: palette(245) },
		previous: null,
		next: null,
		palette: palette(245),
	});
	assert.equal(merged.previous, null);
	assert.equal(merged.next, null);
	assert.equal(merged.current.trackId, "new");
});
