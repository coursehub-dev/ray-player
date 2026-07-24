export const doctorRows = Object.freeze([
	{ id: "storage", title: "Папка данных и ассетов" },
	{ id: "ffmpeg", title: "FFmpeg / ffprobe" },
	{ id: "onnxruntime", title: "ONNX Runtime" },
	{ id: "minilm", title: "MiniLM" },
	{ id: "essentia", title: "Essentia models" },
]);

export const createDoctorState = () =>
	Object.fromEntries(
		doctorRows.map((row) => [
			row.id,
			{
				...row,
				status: "pending",
				message: "Ожидает проверки",
				path: "",
				repairable: false,
			},
		]),
	);

export const mergeDoctorPatch = (settings = {}, patch = {}) => ({
	...settings,
	...(patch.onnxRuntimePath ? { onnxRuntimePath: patch.onnxRuntimePath } : {}),
	...(patch.miniLMModelDir ? { miniLMModelDir: patch.miniLMModelDir } : {}),
	...(patch.ffmpegPath ? { ffmpegPath: patch.ffmpegPath } : {}),
	...(patch.ffprobePath ? { ffprobePath: patch.ffprobePath } : {}),
});

export const doctorStatusLabel = (row) => {
	if (!row || row.status === "pending" || row.status === "checking") {
		return "Проверка";
	}
	if (row.status === "ready") return "Готово";
	if (row.status === "repairable") return "Исправить";
	return "Не готово";
};
