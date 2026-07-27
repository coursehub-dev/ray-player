export async function bootstrapFallback() {
	return {
		library: [] as unknown[],
		current: { volume: 0.58, queue: [] as unknown[] },
		podcastHistory: [] as unknown[],
		podcastRays: [] as unknown[],
		history: [] as unknown[],
		rays: [] as unknown[],
		queue: [] as unknown[],
		libraryStat: { tracks: 0 },
		emoFlow: {},
		emoFlowUiSettings: {
			enabled: true,
			intensity: 1,
			animateDuringTrack: true,
			respectReducedMotion: true,
		},
	};
}
