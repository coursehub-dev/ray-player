export const musicContentLabels: Record<string, string> = {
	stable: "Ровный поток",
	warm_up: "Разогрев",
	cool_down: "Снижение",
	intensify: "Интенсивнее",
	deepen: "Глубже",
	explore: "Исследование",
};

export const musicSortLabels: Record<string, string> = {
	recommended: "Рекомендуемое",
	name_asc: "Название A → Z",
	name_desc: "Название Z → A",
	date_desc: "Сначала новые",
	date_asc: "Сначала старые",
	manual: "Ручной порядок",
};

export function musicRayContentLabel(mode: string): string {
	return musicContentLabels[mode] || "Ровный поток";
}

export function musicRaySortLabel(mode: string): string {
	return musicSortLabels[mode] || "Рекомендуемое";
}
