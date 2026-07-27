<script lang="ts">
import { createEventDispatcher } from "svelte";
import {
	Play,
	Pause,
	LoaderCircle,
	SkipForward,
	SkipBack,
	Volume,
	Volume1,
	Volume2,
	VolumeX,
	Repeat,
} from "@lucide/svelte";
import { IconButton, UIButton, UISlider } from "../../shared/ui";
import { TrackMetaLine } from "../../entities/track";

export let playerTitle = "";
export let playingPodcast = false;
export let currentTrackMeta: any = null;
export let playerArtist = "";
export let playerSubline = "";
export let playbackStatus = "stopped";
export let playbackLastError = "";
export let playbackCurrentGenre = "";
export let playerEmoFlowReason = "";
export let repeatRay = false;
export let playbackSelection = false;
export let seekValue = 0;
export let positionLabel = "0:00";
export let durationLabel = "0:00";
export let volumeIconLevel: "muted" | "low" | "medium" | "high" = "medium";
export let volumeMuteBusy = false;
export let displayedVolume = 0.58;
export let accentReactive = false;

const dispatch = createEventDispatcher<{
	repeat: void;
	previous: void;
	togglePause: void;
	next: void;
	seekPreview: number;
	seekCommit: number;
	mute: void;
	volumePreview: number;
	volumeCommit: number;
}>();
</script>

<footer class="player">
	<div class="player-inner">
		<div class="player-now">
			<div class="cover"></div>
			<div class="meta">
				<strong>{playerTitle}</strong>
				{#if !playingPodcast && currentTrackMeta}
					<TrackMetaLine track={currentTrackMeta} maxGenres={2} showBpm={true} />
				{:else}
					<span>{playerArtist || playerSubline}</span>
				{/if}
				{#if !playingPodcast && playbackCurrentGenre}
					<small>{playbackCurrentGenre}</small>
				{/if}
				{#if playbackStatus === "error" && playbackLastError}
					<small class="player-error">
						{playbackLastError}
					</small>
				{/if}
				{#if playerEmoFlowReason && playbackStatus !== "error"}
					<small class="player-emoflow" title={playerEmoFlowReason}>
						{playerEmoFlowReason}
					</small>
				{/if}
			</div>
		</div>

		<div class="transport">
			<div class="controls">
				<IconButton
					className={`control-btn ${repeatRay ? "active accent-reactive" : ""}`}
					title="Repeat ray"
					on:click={() => dispatch("repeat")}
				>
					<Repeat size={18} strokeWidth={1.8} />
				</IconButton>
				<IconButton
					className="control-btn"
					title="Previous"
					disabled={!playbackSelection}
					on:click={() => dispatch("previous")}
				>
					<SkipBack size={18} strokeWidth={1.8} />
				</IconButton>
				<UIButton
					primary
					className={`play-btn ${playbackStatus === "loading" ? "loading" : ""}`}
					disabled={!playbackSelection}
					on:click={() => dispatch("togglePause")}
				>
					{#if playbackStatus === "loading"}
						<LoaderCircle size={18} strokeWidth={2} />
					{:else if playbackStatus === "playing"}
						<Pause size={18} strokeWidth={2} />
					{:else}
						<Play size={18} strokeWidth={2} />
					{/if}
				</UIButton>
				<IconButton
					className="control-btn"
					title="Next"
					disabled={!playbackSelection}
					on:click={() => dispatch("next")}
				>
					<SkipForward size={18} strokeWidth={1.8} />
				</IconButton>
			</div>
			<div class="seek">
				<span>{positionLabel || "0:00"}</span>
				<UISlider
					value={seekValue / 100}
					min={0}
					max={1}
					step={0.01}
					showValue={false}
					disabled={!playbackSelection}
					{accentReactive}
					on:preview={(e) => dispatch("seekPreview", e.detail)}
					on:commit={(e) => dispatch("seekCommit", e.detail)}
				/>
				<span>{durationLabel || "0:00"}</span>
			</div>
		</div>

		<div class="player-side">
			<button
				type="button"
				class="volume-icon-button"
				class:muted={volumeIconLevel === "muted"}
				aria-label={volumeIconLevel === "muted" ? "Включить звук" : "Выключить звук"}
				title={volumeIconLevel === "muted" ? "Включить звук" : "Выключить звук"}
				disabled={volumeMuteBusy}
				on:click={() => dispatch("mute")}
			>
				{#if volumeIconLevel === "muted"}
					<VolumeX size={16} strokeWidth={1.8} />
				{:else if volumeIconLevel === "low"}
					<Volume size={16} strokeWidth={1.8} />
				{:else if volumeIconLevel === "medium"}
					<Volume1 size={16} strokeWidth={1.8} />
				{:else}
					<Volume2 size={16} strokeWidth={1.8} />
				{/if}
			</button>
			<div class="volume-range">
				<UISlider
					value={displayedVolume}
					min={0}
					max={1}
					step={0.01}
					showValue={false}
					{accentReactive}
					on:preview={(e) => dispatch("volumePreview", e.detail)}
					on:commit={(e) => dispatch("volumeCommit", e.detail)}
				/>
			</div>
		</div>
	</div>
</footer>
