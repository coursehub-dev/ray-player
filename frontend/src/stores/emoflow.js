import { derived, writable } from "svelte/store";

const DEFAULT_SETTINGS = {
	enabled: true,
	intensity: 1,
	animateDuringTrack: true,
	respectReducedMotion: true,
};

const DEFAULT_PALETTE = {
	accent: "oklch(62% 0.080 230)",
	accentSoft: "oklch(70% 0.060 230)",
	accentHot: "oklch(66% 0.110 230)",
	background: "oklch(10% 0.018 230)",
	surface: "oklch(14% 0.022 230)",
	glow: "oklch(66% 0.110 230 / 0.26)",
	glowSoft: "oklch(62% 0.080 230 / 0.18)",
	ring: "oklch(62% 0.080 230 / 0.14)",
	progress: "oklch(66% 0.090 230)",
	icon: "oklch(74% 0.090 230)",
	accentOn: "rgba(255,255,255,.92)",
};

const DEFAULT_MOTION = {
	pulseSpeed: 8.8,
	glowStrength: 0.16,
	breathingSpeed: 10.5,
	transitionMs: 700,
};

export const emoFlowState = writable({
	trackId: "",
	current: null,
	previous: null,
	next: null,
	vector: null,
	direction: "stable",
	intensity: 0,
	heat: 0,
	cool: 0,
	tension: 0,
	palette: DEFAULT_PALETTE,
	transition: null,
});

export const emoFlowSettings = writable({ ...DEFAULT_SETTINGS });
export const playback = writable({
	trackId: "",
	position: 0,
	duration: 0,
	isPlaying: false,
});

export function updatePlaybackFromCurrent(current) {
	playback.set({
		trackId: current?.currentTrackId || "",
		position: current?.positionMs || 0,
		duration: current?.durationMs || 0,
		isPlaying: Boolean(current?.playing),
	});
}

export function syncEmoFlowFromPayload(payload) {
	if (payload?.emoFlowUiSettings) {
		emoFlowSettings.set({ ...DEFAULT_SETTINGS, ...payload.emoFlowUiSettings });
	}
	if (payload?.emoFlow) {
		emoFlowState.update((state) => mergeEmoFlowState(state, payload.emoFlow));
	}
	if (payload?.current) {
		updatePlaybackFromCurrent(payload.current);
	}
}

export const trackProgress = derived(playback, ($p) => {
	if (!$p.duration) return 0;
	return clamp01($p.position / $p.duration);
});

export const activePalette = derived(
	[emoFlowState, emoFlowSettings, trackProgress],
	([$emo, $settings, $progress]) => {
		if (!$settings.enabled) return DEFAULT_PALETTE;
		const current = $emo.current?.palette || $emo.palette || DEFAULT_PALETTE;
		const next = $emo.next?.palette || current;
		if (!$settings.animateDuringTrack) {
			return mixPalette(DEFAULT_PALETTE, current, $settings.intensity);
		}
		const interpolated = interpolateTrackPalette(current, next, $progress);
		return mixPalette(
			DEFAULT_PALETTE,
			interpolated,
			clamp01($settings.intensity),
		);
	},
);

export const activeMotion = derived(
	[emoFlowState, emoFlowSettings],
	([$emo, $settings]) => {
		if (!$settings.enabled) return DEFAULT_MOTION;
		const vector = $emo.current?.vector || $emo.vector;
		if (!vector) return DEFAULT_MOTION;
		return computeMotion(vector, $emo.direction, $settings);
	},
);

export const cssVariables = derived(
	[activePalette, activeMotion, emoFlowSettings],
	([$palette, $motion, $settings]) => ({
		"--accent": $palette.accent,
		"--accent-soft": $palette.accentSoft,
		"--accent-hot": $palette.accentHot,
		"--accent-glow": $palette.glow,
		"--accent-glow-soft": $palette.glowSoft || $palette.glow,
		"--accent-ring": $palette.ring || "oklch(68% 0.220 220 / 0.18)",
		"--progress-color": $palette.progress,
		"--icon-active": $palette.icon,
		"--accent-on": $palette.accentOn || "rgba(0,0,0,.92)",
		"--app-bg": $palette.background,
		"--panel-tint": $palette.surface,
		"--emo-glow-opacity": String($motion.glowStrength),
		"--emo-pulse-speed": `${$motion.pulseSpeed}s`,
		"--emo-breathing-speed": `${$motion.breathingSpeed}s`,
		"--emo-transition-ms": `${Math.round($motion.transitionMs)}ms`,
		"--emo-enabled": $settings.enabled ? "1" : "0",
	}),
);

export function mergeEmoFlowState(state, nextState) {
	const normalized = nextState ? normalizeMoodForColor(nextState) : nextState;
	const palette = normalized?.palette?.accent
		? normalized.palette
		: state?.palette || DEFAULT_PALETTE;
	const merged = {
		...(state || {}),
		...normalized,
		direction: normalized?.direction || state?.direction || "stable",
		palette,
	};
	const previousCurrent = state?.current || null;
	const nextCurrent = normalized?.current || null;
	const sameCurrentTrack =
		Boolean(previousCurrent?.trackId) &&
		previousCurrent?.trackId === nextCurrent?.trackId;

	merged.current = normalized?.current
		? {
				...(sameCurrentTrack ? previousCurrent : {}),
				...normalized.current,
				palette: normalized.current.palette || palette,
			}
		: state?.current || null;
	const hasPrevious = Boolean(normalized) && Object.prototype.hasOwnProperty.call(normalized, "previous");
	const hasNext = Boolean(normalized) && Object.prototype.hasOwnProperty.call(normalized, "next");
	merged.previous = normalized?.previous
		? {
				...(sameCurrentTrack ? state?.previous || {} : {}),
				...normalized.previous,
				palette: normalized.previous.palette || palette,
			}
		: hasPrevious
			? null
			: state?.previous || null;
	merged.next = normalized?.next
		? {
				...(sameCurrentTrack ? state?.next || {} : {}),
				...normalized.next,
				palette: normalized.next.palette || palette,
			}
		: hasNext
			? null
			: state?.next || null;
	return merged;
}

function getNormalizedState(state) {
	return mergeEmoFlowState(null, state);
}

function normalizeMoodForColor(mood) {
	const energy = clamp01(mood?.energy ?? 0);
	const valence = clamp01(mood?.valence ?? 0);
	const sadRaw = clamp01(mood?.sad ?? 0);
	const party = clamp01(mood?.party ?? 0);
	const relaxed = clamp01(mood?.relaxed ?? 0);
	const aggressive = clamp01(mood?.aggressive ?? 0);
	const electronic = clamp01(mood?.electronicness ?? mood?.electronic ?? 0);
	const dreamy = clamp01(mood?.dreaminess ?? mood?.dreamy ?? 0);
	const emotional = clamp01(mood?.emotionality ?? mood?.emotional ?? 0);
	const drive = clamp01(mood?.drive ?? 0);
	const intensity = clamp01(mood?.intensity ?? 0);
	const melancholy = clamp01(mood?.melancholy ?? 0);
	const darkByValence = clamp01((0.45 - valence) / 0.45);
	const sad = clamp01(
		((sadRaw - 0.65) / 0.35) * 0.35 +
			melancholy * 0.3 +
			darkByValence * 0.25 -
			drive * 0.25,
	);
	return {
		...mood,
		energy,
		valence,
		sad,
		party,
		relaxed,
		aggressive,
		electronic,
		dreamy,
		emotional,
		drive,
		intensity,
		melancholy,
	};
}

function computeMotion(vector, direction, settings) {
	const pulse = clamp01(
		0.25 + 0.45 * (vector.energy || 0) + 0.35 * (vector.aggression || 0),
	);
	const breathing = clamp01(
		0.65 * (vector.calmness || 0) + 0.35 * (vector.melodicness || 0),
	);
	const sharpness = clamp01(
		0.6 * (vector.aggression || 0) + 0.4 * (vector.engagement || 0),
	);
	const reduced = settings.respectReducedMotion && prefersReducedMotion();
	let motion = {
		pulseSpeed: lerp(10, 3, pulse),
		glowStrength: lerp(0.12, 0.34, pulse) * clamp01(settings.intensity || 1),
		breathingSpeed: lerp(14, 7, breathing),
		transitionMs: lerp(900, 600, sharpness),
	};
	if (direction === "cool_down") {
		motion = {
			...motion,
			pulseSpeed: motion.pulseSpeed + 2.4,
			breathingSpeed: motion.breathingSpeed + 2,
			glowStrength: motion.glowStrength * 0.76,
			transitionMs: motion.transitionMs + 120,
		};
	} else if (direction === "warm_up" || direction === "intensify") {
		motion = {
			...motion,
			pulseSpeed: Math.max(2.8, motion.pulseSpeed - 1),
			glowStrength: Math.min(0.34, motion.glowStrength * 1.15),
			transitionMs: Math.max(520, motion.transitionMs - 100),
		};
	} else if (direction === "deepen") {
		motion = {
			...motion,
			pulseSpeed: motion.pulseSpeed + 1.2,
			glowStrength: motion.glowStrength * 0.88,
		};
	}
	if (reduced) {
		motion = {
			pulseSpeed: 0,
			glowStrength: Math.min(motion.glowStrength, 0.1),
			breathingSpeed: 0,
			transitionMs: 200,
		};
	}
	return motion;
}

export function interpolateTrackPalette(current, next, progress) {
	const p = clamp01(progress);
	const handoffStart = 0.82;
	if (!next || p <= handoffStart) return current;

	// The outgoing track owns the colour until its final section.  By the
	// natural end we have already reached the incoming track palette, so the
	// next track starts with the same colour instead of flashing the old one.
	return mixPalette(current, next, (p - handoffStart) / (1 - handoffStart));
}

function mixPalette(a, b, t) {
	t = smoothstep(t);
	return {
		accent: mixColor(a.accent, b.accent, t),
		accentSoft: mixColor(a.accentSoft, b.accentSoft, t),
		accentHot: mixColor(a.accentHot || a.accent, b.accentHot || b.accent, t),
		background: mixColor(a.background, b.background, t),
		surface: mixColor(a.surface, b.surface, t),
		glow: mixColor(a.glow, b.glow, t),
		glowSoft: mixColor(a.glowSoft || a.glow, b.glowSoft || b.glow, t),
		ring: mixColor(a.ring || a.glow, b.ring || b.glow, t),
		progress: mixColor(a.progress, b.progress, t),
		icon: mixColor(a.icon, b.icon, t),
		accentOn:
			t < 0.5
				? a.accentOn || DEFAULT_PALETTE.accentOn
				: b.accentOn || DEFAULT_PALETTE.accentOn,
	};
}

function mixColor(a, b, t) {
	const ca = parseOKLCH(a);
	const cb = parseOKLCH(b);
	return toOKLCHString(mixOKLCH(ca, cb, t));
}

function mixOKLCH(a, b, t) {
	t = clamp01(t);
	const delta = ((((b.h - a.h) % 360) + 540) % 360) - 180;
	return {
		l: a.l + (b.l - a.l) * t,
		c: a.c + (b.c - a.c) * t,
		h: (((a.h + delta * t) % 360) + 360) % 360,
		a: a.a + (b.a - a.a) * t,
	};
}

function parseOKLCH(value) {
	const source = String(value || "").trim();
	const match = source.match(/oklch\(([^)]+)\)/i);
	if (match) {
		const [base, alphaPart] = match[1].split("/").map((part) => part.trim());
		const parts = base.split(/\s+/).filter(Boolean);
		if (parts.length >= 3) {
			return {
				l: Number(parts[0].replace("%", "")) / 100,
				c: Number(parts[1]),
				h: Number(parts[2]),
				a: alphaPart ? Number(alphaPart) : 1,
			};
		}
	}
	return { l: 0.68, c: 0.22, h: 220, a: 1 };
}

function toOKLCHString(color) {
	const l = Math.round(clamp(color.l, 0, 1) * 100);
	const lightness = clamp(color.l, 0, 1);
	const minChroma = lightness < 0.24 ? 0.006 : 0.025;
	const maxChroma = lightness < 0.24 ? 0.09 : lightness > 0.7 ? 0.24 : 0.2;
	const c = clamp(color.c, minChroma, maxChroma).toFixed(3);
	let h = Math.round(((color.h % 360) + 360) % 360);
	if (lightness < 0.32 && h >= 265 && h <= 325) {
		h = 215;
	}
	const alpha = clamp(color.a, 0, 1);
	if (alpha >= 0.999) return `oklch(${l}% ${c} ${h})`;
	return `oklch(${l}% ${c} ${h} / ${alpha.toFixed(3)})`;
}

function prefersReducedMotion() {
	return Boolean(
		globalThis?.window?.matchMedia?.("(prefers-reduced-motion: reduce)")
			?.matches,
	);
}

function lerp(a, b, t) {
	return a + (b - a) * clamp01(t);
}

function clamp01(value) {
	return Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
}

function smoothstep(t) {
	t = clamp01(t);
	return t * t * (3 - 2 * t);
}

function clamp(value, min, max) {
	return Math.max(min, Math.min(max, Number.isFinite(value) ? value : min));
}
