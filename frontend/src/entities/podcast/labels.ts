export const podcastContentLabels: Record<string, string> = {
	recommended: "Рекомендуемое",
	explore: "Исследование",
	current_folder: "Текущая папка",
};

export const podcastSortLabels: Record<string, string> = {
	recommended: "Рекомендуемое",
	name_asc: "Название A → Z",
	name_desc: "Название Z → A",
	date_desc: "Сначала новые",
	date_asc: "Сначала старые",
	manual: "Ручной порядок",
};

export function podcastHistorySourceLabel(source: string): string {
	switch (source) {
		case "library":
			return "Из библиотеки";
		case "ray":
			return "Из луча";
		case "ray_auto":
			return "Автопереход луча";
		case "ray_previous":
			return "Назад по лучу";
		case "resume":
			return "Продолжение";
		default:
			return "Ручной запуск";
	}
}

export function podcastRayContentLabel(mode: string): string {
	return podcastContentLabels[mode] || "Рекомендуемое";
}

export function podcastRaySortLabel(mode: string): string {
	return podcastSortLabels[mode] || "Рекомендуемое";
}
