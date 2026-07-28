export const hasSuggestionQuery = (query: unknown): boolean => String(query ?? "").trim().length >= 1;

export const shouldShowSuggestions = ({
	focused,
	query,
	count,
}: {
	focused: boolean;
	query: unknown;
	count: number;
}): boolean => focused && hasSuggestionQuery(query) && count > 0;
