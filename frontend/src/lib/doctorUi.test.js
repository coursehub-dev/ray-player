import assert from "node:assert/strict";
import test from "node:test";
import {
	createDoctorState,
	doctorRows,
	doctorStatusLabel,
	mergeDoctorPatch,
} from "./doctorUi.js";

test("doctor initializes every row as pending", () => {
	const state = createDoctorState();
	assert.equal(Object.keys(state).length, doctorRows.length);
	for (const row of doctorRows) assert.equal(state[row.id].status, "pending");
});

test("doctor patch only replaces values returned by backend", () => {
	const settings = {
		onnxRuntimePath: "old-runtime",
		miniLMModelDir: "old-model",
		ffmpegPath: "ffmpeg",
		ffprobePath: "ffprobe",
		essentiaModelDir: "essentia",
	};
	const next = mergeDoctorPatch(settings, {
		ffmpegPath: "/managed/ffmpeg",
		ffprobePath: "/managed/ffprobe",
		essentiaModelDir: "/managed/essentia",
	});
	assert.equal(next.ffmpegPath, "/managed/ffmpeg");
	assert.equal(next.ffprobePath, "/managed/ffprobe");
	assert.equal(next.onnxRuntimePath, "old-runtime");
	assert.equal(next.essentiaModelDir, "/managed/essentia");
});

test("doctor status labels map to the intended actions", () => {
	assert.equal(doctorStatusLabel({ status: "pending" }), "Проверка");
	assert.equal(doctorStatusLabel({ status: "ready" }), "Готово");
	assert.equal(doctorStatusLabel({ status: "repairable" }), "Исправить");
	assert.equal(doctorStatusLabel({ status: "blocked" }), "Не готово");
});
