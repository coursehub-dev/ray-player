<script lang="ts">
export let track: Record<string, any> | null = null;
export let artist = "";
export let genreLabel = "";
export let genrePrimary = "";
export let genreTags: any[] = [];
export let maxGenres = 2;
export let showBpm = false;
export let bpm = 0;
export let tempo = 0;
export let tempoConfidence = 0;
export let className = "";

const tagLabel = (tag: any) =>
	String(tag?.label ?? tag?.Label ?? tag?.name ?? tag?.Name ?? tag?.genre ?? tag?.Genre ?? "").trim();

const compactGenreText = (label: unknown, primary: unknown, tags: unknown, limit: number) => {
	const preparedLabel = String(label || "").trim();
	if (preparedLabel) return preparedLabel;

	if (Array.isArray(tags) && tags.length) {
		const values = tags
			.map(tagLabel)
			.filter(Boolean)
			.slice(0, Math.max(1, limit || 2));

		if (values.length) return values.join(", ");
	}

	return String(primary || "").trim();
};

const formatBpm = (value: unknown, fallback: unknown, confidence: unknown) => {
	const resolved = Math.round(Number(value || fallback || 0));
	if (!showBpm || !resolved) return "";
	return Number(confidence || 0) > 0 && Number(confidence || 0) < 0.35 ? `~${resolved} BPM` : `${resolved} BPM`;
};

$: resolvedArtist = String(artist || track?.artist || "").trim();
$: resolvedGenres = compactGenreText(
	genreLabel || track?.genreLabel,
	genrePrimary || track?.genrePrimary,
	genreTags?.length ? genreTags : track?.genreTags,
	maxGenres,
);
$: resolvedBpm = formatBpm(
	bpm || track?.bpmPerceived,
	tempo || track?.tempo,
	tempoConfidence || track?.tempoConfidence,
);
$: hasContent = Boolean(resolvedArtist || resolvedGenres || resolvedBpm);
</script>

{#if hasContent}
    <div class={`track-meta-line ${className}`.trim()}>
        {#if resolvedArtist}
            <span class="track-meta-artist">{resolvedArtist}</span>
        {/if}

        {#if resolvedArtist && resolvedGenres}
            <span class="track-meta-dot">·</span>
        {/if}

        {#if resolvedGenres}
            <span class="track-meta-genres">{resolvedGenres}</span>
        {/if}

        {#if (resolvedArtist || resolvedGenres) && resolvedBpm}
            <span class="track-meta-dot">·</span>
        {/if}

        {#if resolvedBpm}
            <span class="track-meta-bpm">{resolvedBpm}</span>
        {/if}
    </div>
{/if}
