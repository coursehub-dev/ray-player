const warningsAsErrors = process.env.SVELTE_WARNINGS_AS_ERRORS === "1";

export default {
	compilerOptions: {
		compatibility: {
			componentApi: 4,
		},
	},
	onwarn(warning, defaultHandler) {
		if (!warningsAsErrors) {
			defaultHandler(warning);
			return;
		}

		const location = warning.filename
			? ` (${warning.filename}${warning.start?.line ? `:${warning.start.line}` : ""})`
			: "";
		throw new Error(`Svelte compiler warning ${warning.code}${location}: ${warning.message}`);
	},
};
