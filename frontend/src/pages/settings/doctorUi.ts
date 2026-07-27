export type DoctorRowId = "storage" | "ffmpeg" | "onnxruntime" | "minilm" | "essentia";

export type DoctorRow = {
	id: DoctorRowId | string;
	title: string;
	status: string;
	message: string;
	path: string;
	repairable: boolean;
};

export const doctorRows = Object.freeze([
	{ id: "storage", title: "Папка данных и ассетов" },
	{ id: "ffmpeg", title: "FFmpeg / ffprobe" },
	{ id: "onnxruntime", title: "ONNX Runtime" },
	{ id: "minilm", title: "MiniLM" },
	{ id: "essentia", title: "Essentia models" },
] as const);

export const createDoctorState = (): Record<string, DoctorRow> =>
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

export const mergeDoctorPatch = (
	settings: Record<string, unknown> = {},
	patch: Record<string, unknown> = {},
): Record<string, unknown> => ({
	...settings,
	...(patch.onnxRuntimePath ? { onnxRuntimePath: patch.onnxRuntimePath } : {}),
	...(patch.miniLMModelDir ? { miniLMModelDir: patch.miniLMModelDir } : {}),
	...(patch.essentiaModelDir ? { essentiaModelDir: patch.essentiaModelDir } : {}),
	...(patch.ffmpegPath ? { ffmpegPath: patch.ffmpegPath } : {}),
	...(patch.ffprobePath ? { ffprobePath: patch.ffprobePath } : {}),
});

export const doctorStatusLabel = (row: { status?: string } | null | undefined): string => {
	if (!row || row.status === "pending" || row.status === "checking") {
		return "Проверка";
	}
	if (row.status === "ready") return "Готово";
	if (row.status === "repairable") return "Исправить";
	return "Не готово";
};
