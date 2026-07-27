import assert from "node:assert/strict";
import test from "node:test";
import { clampSliderValue, sliderValueFromKey, snapSliderValue } from "./sliderUi.ts";

test("slider snapping is relative to min and clamps to bounds", () => {
	assert.equal(snapSliderValue(11.4, 10, 20, 3), 10);
	assert.equal(snapSliderValue(12.1, 10, 20, 3), 13);
	assert.equal(snapSliderValue(99, 10, 20, 3), 20);
	assert.equal(clampSliderValue(Number.NaN, 4, 8), 4);
});

test("slider keyboard controls use step, page, home and end", () => {
	const base = { value: 50, min: 0, max: 100, step: 5 };

	assert.equal(sliderValueFromKey({ ...base, key: "ArrowRight" }), 55);
	assert.equal(sliderValueFromKey({ ...base, key: "ArrowDown" }), 45);
	assert.equal(sliderValueFromKey({ ...base, key: "PageUp" }), 100);
	assert.equal(sliderValueFromKey({ ...base, key: "Home" }), 0);
	assert.equal(sliderValueFromKey({ ...base, key: "End" }), 100);
	assert.equal(sliderValueFromKey({ ...base, key: "Enter" }), null);
});
