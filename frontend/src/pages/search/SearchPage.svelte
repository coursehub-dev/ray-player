<script lang="ts">
import { Play, Pause, LoaderCircle, Sparkles, CheckCircle2, FileText } from "@lucide/svelte";
import { TrackMetaLine } from "../../entities/track";
import { PodcastProgressBar } from "../../entities/podcast";

export let libraryMode: string = "music";
export let playback: any = {};
export let libraryEmpty = false;
export let podcastResults: any[] = [];
export let visibleResults: any[] = [];

export let togglePodcastRow: (id: string, fromRay: boolean) => void = () => {};
export let playOrToggle: (trackId: string, screen?: string | null) => void = () => {};
export let openTrackMenu: (event: MouseEvent | KeyboardEvent, trackId: string, source: string) => void = () => {};
export let externalPlayable: (item: any) => boolean = () => true;
export let externalStatusLabel: (item: any) => string = () => "";
export let externalState: (item: any) => { progress: number } = () => ({ progress: 0 });
export let podcastMeta: (item: any) => string = () => "";
export let podcastProgressPercent: (item: any) => number = () => 0;
export let rowCurrent: (trackId: string) => boolean = () => false;
export let rowIsBuildingRay: (trackId: string) => boolean = () => false;
export let rowIcon: (trackId: string) => string = () => "▶";
export let rowRaySeed: (trackId: string) => boolean = () => false;
export let genreBadge: (track: any) => string = () => "";
</script>

<section class="screen active library-screen">
	<div class="screen-body library-layout">
		{#if libraryEmpty}
			<div class="empty-state">
				<strong>Библиотека пока пустая</strong>
				<span
					>Добавь {libraryMode === "podcast" ? "папку с подкастами или отдельные выпуски" : "папку или отдельные аудиофайлы"} слева.</span
				>
			</div>
		{:else if libraryMode === "podcast"}
			<div class="list podcast-list">
				{#each podcastResults as item}
					<button
						type="button"
						class:completed={item.isCompleted}
						class:current={item.id === playback.currentTrackId}
						class:external-pending={!externalPlayable(item)}
						disabled={!externalPlayable(item)}
						class="row action-row podcast-row"
						on:click={() => togglePodcastRow(item.id, false)}
					>
						<div class="podcast-icon">
							{#if item.id === playback.currentTrackId && playback.status === "playing"}
								<Pause size={20} strokeWidth={1.8} />
							{:else}
								<Play size={20} strokeWidth={1.8} />
							{/if}
						</div>
						<div class="meta">
							<strong>{item.title}</strong>
							<span class="podcast-meta">
								{podcastMeta(item) || "Локальный выпуск"}
							</span>
							<span class="podcast-progress-label">
								{#if item.isCompleted}
									Прослушано
								{:else if podcastProgressPercent(item) > 0}
									Прослушано {podcastProgressPercent(item)}% · продолжить с {Math.floor(item.resumePosition / 60)}:{String(Math.floor(item.resumePosition % 60)).padStart(2, "0")}
								{:else}
									Новый выпуск
								{/if}
							</span>
							{#if item.sourceType === "yt_dlp" && externalStatusLabel(item)}
								<span class="external-download-label">
									{externalStatusLabel(item)}
								</span>
							{/if}
						</div>
						<div class="tail tail-stack">
							<span>
								{item.durationLabel || "—"}
							</span>
							<span
								class="semantic-status"
								title="Полная semantic-индексация по MiniLM будет добавлена отдельным worker"
							>
								{#if item.semanticStatus === "done"}
									<CheckCircle2 size={12} />
									Индекс готов
								{:else if item.semanticStatus === "failed"}
									Ошибка индекса
								{:else}
									<FileText size={12} />
									Метаданные готовы
								{/if}
							</span>
						</div>

						<PodcastProgressBar {item} className="podcast-row-progress" />
						{#if item.sourceType === "yt_dlp" && !externalPlayable(item)}
							<span class="external-download-progress">
								<span style={`width:${Math.round(externalState(item).progress * 100)}%`}></span>
							</span>
						{/if}
					</button>
				{/each}
			</div>
		{:else}
			<div class="list">
				{#each visibleResults as row}
					<button
						class:itemCurrent={rowCurrent(row.track.id)}
						class:external-pending={!externalPlayable(row.track)}
						disabled={!externalPlayable(row.track)}
						class="row action-row track-row"
						on:click={() => playOrToggle(row.track.id, "ray")}
						on:contextmenu={(event) => openTrackMenu(event, row.track.id, "search")}
					>
						<div class="cover-wrapper">
							<div class="cover"></div>
							<div class="cover-play-indicator">
								<span class:icon-playing={rowCurrent(row.track.id)}>
									{#if rowIsBuildingRay(row.track.id)}
										<LoaderCircle size={15} />
									{:else}
										{rowIcon(row.track.id)}
									{/if}
								</span>
								{#if rowRaySeed(row.track.id)}
									<Sparkles
										class="ray-seed-icon"
										size={14}
										title="Seed track for current ray"
									/>
								{/if}
							</div>
						</div>
						<div class="meta">
							<strong>{row.track.title || row.track.fileName}</strong>
							<TrackMetaLine track={row.track} maxGenres={2} showBpm={true} />
							{#if row.track.sourceType === "yt_dlp" && externalStatusLabel(row.track)}
								<span class="external-download-label">
									{externalStatusLabel(row.track)}
								</span>
							{/if}
							{#if genreBadge(row.track)}
								<div class="badges">
									<i class="badge genre">{genreBadge(row.track)}</i>
								</div>
							{/if}
						</div>
						<div class="tail tail-stack">
							<span>{row.track.durationLabel}</span>
						</div>
						{#if row.track.sourceType === "yt_dlp" && !externalPlayable(row.track)}
							<span class="external-download-progress">
								<span style={`width:${Math.round(externalState(row.track).progress * 100)}%`}></span>
							</span>
						{/if}
						<span class="track-menu-wrap">
							<span
								class="track-menu-btn"
								role="button"
								tabindex="0"
								aria-label="Меню трека"
								on:click={(event) => openTrackMenu(event, row.track.id, "search")}
								on:keydown={(event) =>
									(event.key === "Enter" || event.key === " ") &&
									openTrackMenu(event, row.track.id, "search")}>⋯</span
							>
						</span>
					</button>
				{/each}
			</div>
		{/if}
	</div>
</section>
