import { writable } from "svelte/store";

export type RayBuildState = {
	status: string;
	seedTrackId: string;
	requestId: number;
	startedAt: number;
	finishedAt: number;
	lastError: string;
};

export const rayBuildState = writable<RayBuildState>({
	status: "idle",
	seedTrackId: "",
	requestId: 0,
	startedAt: 0,
	finishedAt: 0,
	lastError: "",
});

export function syncRayBuild(payload: { rayBuild?: RayBuildState } | RayBuildState | null | undefined) {
	const build = (payload as { rayBuild?: RayBuildState })?.rayBuild || payload;
	if (build && typeof build === "object" && "status" in build && build.status) {
		rayBuildState.set(build as RayBuildState);
	}
}
