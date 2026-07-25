import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig(({ command }) => {
	if (command === "build") {
		process.env.SVELTE_WARNINGS_AS_ERRORS = "1";
	}

	return {
		plugins: [svelte()],
		server: { strictPort: true },
	};
});
