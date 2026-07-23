export async function bootstrapFallback() {
	return {
		library: [],
		current: { volume: 0.58, queue: [] },
		podcastHistory: [],
		podcastRays: [],
		history: [],
		rays: [],
		queue: [],
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
