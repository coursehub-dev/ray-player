<script lang="ts">
import { createEventDispatcher } from "svelte";
import { GripVertical, LoaderCircle, Pause, Play, Sparkles } from "@lucide/svelte";
import TrackMetaLine from "./TrackMetaLine.svelte";

export let item: any;
export let index = 0;
export let playback: Record<string, any> = {};
export let showInsight = false;
export let insightLine = "";
export let debugLine = "";
export let dropTarget = false;
export let dragging = false;

const dispatch = createEventDispatcher<{
	play: { trackId: string };
	menu: { event: MouseEvent; trackId: string };
	dragstart: { event: DragEvent; item: any; index: number };
	dragover: { event: DragEvent; index: number };
	drop: { event: DragEvent; index: number };
	dragend: void;
}>();

const roleLabels: Record<string, string> = {
	next: "далее",
	nearby: "рядом",
	discovery: "открытие",
	bridge: "переход",
	comfort: "своё",
	manual: "вручную",
	seed: "seed",
};

const roleName = (value: unknown) => {
	const normalized = String(value || "")
		.trim()
		.toLowerCase();
	return roleLabels[normalized] || normalized;
};

const open = () => {
	if (item?.trackId) {
		dispatch("play", { trackId: item.trackId });
	}
};

const openMenu = (event: MouseEvent | KeyboardEvent) => {
	event.preventDefault();
	event.stopPropagation();
	dispatch("menu", {
		event: event as MouseEvent,
		trackId: item?.trackId,
	});
};

$: trackId = item?.trackId || item?.id || item?.track?.id || "";
$: isCurrent = Boolean(trackId) && trackId === playback?.currentTrackId;
$: isPlaying = isCurrent && playback?.status === "playing";
$: isLoading = Boolean(trackId) && trackId === playback?.currentTrackId && playback?.status === "loading";
$: isSeed = Boolean(playback?.rayId) && trackId === playback?.raySeedTrackId;
$: role = String(item?.rayRole || item?.role || (isSeed ? "seed" : "next"))
	.trim()
	.toLowerCase();
$: roleLabel = roleName(role);
$: reason = item?.rayReason || item?.reason || "";
$: metadataTrack = {
	...(item?.track || {}),
	artist: item?.artist || item?.track?.artist || "",
	genreLabel: item?.genreLabel || item?.track?.genreLabel || "",
	genrePrimary: item?.genrePrimary || item?.track?.genrePrimary || "",
	genreTags: item?.genreTags || item?.track?.genreTags || [],
};
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<button
    type="button"
    class="ray-track-row"
    class:current={isCurrent}
    class:playing={isPlaying}
    class:loading={isLoading}
    class:seed={isSeed}
    class:drop-target={dropTarget}
    class:dragging={dragging}
    data-position={item?.position ?? index}
    on:click={open}
    on:contextmenu={openMenu}
    on:dragover={(event) =>
        dispatch("dragover", { event, index })}
    on:drop={(event) =>
        dispatch("drop", { event, index })}
>
    <span
        class="ray-drag-handle"
        role="button"
        tabindex="0"
        aria-label="Переместить трек"
        draggable="true"
        title="Изменить порядок"
        on:click|stopPropagation
        on:keydown={(event) => {
            if (
                event.key === "Enter" ||
                event.key === " "
            ) {
                event.preventDefault();
                event.stopPropagation();
            }
        }}
        on:dragstart={(event) =>
            dispatch("dragstart", {
                event,
                item,
                index,
            })}
        on:dragend={() => dispatch("dragend")}
    >
        <GripVertical size={15} strokeWidth={1.8} />
    </span>

    <span
        role="button"
        tabindex="0"
        class="ray-track-play"
        aria-label={isPlaying
            ? "Поставить на паузу"
            : "Воспроизвести трек"}
        on:click|stopPropagation={open}
        on:keydown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                event.stopPropagation();
                open();
            }
        }}
    >
        {#if isLoading}
            <LoaderCircle
                class="ray-track-loader"
                size={16}
                strokeWidth={1.8}
            />
        {:else if isPlaying}
            <Pause size={16} strokeWidth={2} />
        {:else}
            <Play size={16} strokeWidth={2} />
        {/if}
    </span>

    <div class="ray-track-text">
        <div class="ray-track-title-line">
            <span class="ray-track-title">
                {item?.title || item?.track?.title || "Без названия"}
            </span>

            {#if isSeed}
                <Sparkles
                    class="ray-seed-icon"
                    size={13}
                    strokeWidth={1.8}
                    aria-label="Seed текущего луча"
                />
            {/if}
        </div>

        <TrackMetaLine
            track={metadataTrack}
            maxGenres={2}
        />

        {#if reason}
            <small class="ray-track-reason">{reason}</small>
        {/if}

        {#if showInsight && insightLine}
            <small class="debug-line debug-line-flow">
                {insightLine}
            </small>
        {/if}

        {#if showInsight && debugLine}
            <small class="debug-line debug-line-track">
                {debugLine}
            </small>
        {/if}
    </div>

    {#if roleLabel}
        <span class={`ray-role ray-role-${role}`}>
            {roleLabel}
        </span>
    {/if}

    <span
        role="button"
        tabindex="0"
        class="ray-track-menu"
        aria-label="Меню трека"
        on:click={openMenu}
        on:keydown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                openMenu(event);
            }
        }}
    >
        ⋯
    </span>
</button>
