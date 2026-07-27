<script lang="ts">
import { Settings, Play, Pause, Sparkles } from "@lucide/svelte";
import { IconButton } from "../../shared/ui";
import TrackMetaLine from "../../components/TrackMetaLine.svelte";

export let libraryMode: string = "music";
export let appState: any = {};
export let playback: any = {};
export let indexing: any = {};

export let openSettings: () => void = () => {};
export let playPodcastHistoryItem: (entry: any) => void = () => {};
export let playOrToggle: (trackId: string, screen?: string | null) => void = () => {};
export let openTrackMenu: (event: MouseEvent | KeyboardEvent, trackId: string, source: string) => void = () => {};
export let podcastMeta: (item: any) => string = () => "";
export let podcastHistorySourceLabel: (source: string) => string = () => "";
export let rowCurrent: (trackId: string) => boolean = () => false;
export let rowIcon: (trackId: string) => string = () => "▶";
export let rowRaySeed: (trackId: string) => boolean = () => false;
</script>

<section class="screen active">
	<div class="screen-head">
		<div>
			<h1>{libraryMode === "podcast" ? "История подкастов" : "История"}</h1>
			<p>{libraryMode === "podcast" ? "Независимая история прослушивания выпусков, прогресса и источников запуска." : "Что слушали, где остановились, и сколько уже прослушано."}</p>
		</div>
		<div class="top-right-status">
			<div class:accent-reactive={indexing.isIndexing} class="library-status">
				{indexing.isIndexing && indexing.total > 0
					? `${indexing.processed}/${indexing.total}`
					: `${indexing.libraryCount || appState.libraryStat?.tracks || 0} tracks`}
			</div>
			<IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton>
		</div>
	</div>
	<div class="screen-body">
		{#if libraryMode === "podcast"}
			<div class="list podcast-history-list">
				{#if (appState.podcastHistory || []).length}
					{#each appState.podcastHistory as entry}
						<button
							type="button"
							class="row action-row podcast-history-row"
							class:current={entry.item.id === playback.currentTrackId}
							on:click={() => playPodcastHistoryItem(entry)}
						>
							<div class="podcast-history-icon">
								{#if entry.item.id === playback.currentTrackId && playback.status === "playing"}
									<Pause size={19} strokeWidth={1.8} />
								{:else}
									<Play size={19} strokeWidth={1.8} />
								{/if}
							</div>
							<div class="meta">
								<strong>{entry.item.title}</strong>
								<span>{podcastMeta(entry.item) || "Локальный выпуск"}</span>
								<small class="podcast-history-state"
									>{entry.playedAtLabel} · {podcastHistorySourceLabel(entry.source)}{#if entry.listenedLabel}
										· прослушано {entry.listenedLabel}{/if}{#if entry.positionLabel}
										· остановка {entry.positionLabel}{/if}</small
								>
							</div>
							<div class="tail tail-stack">
								<span>{entry.progressPercent}%</span>
								{#if entry.rayId}
									<span class="history-ray-link">Луч</span>
								{/if}
							</div>
							<span class="podcast-history-progress"><span style={`width:${entry.progressPercent}%`}></span></span>
						</button>
					{/each}
				{:else}
					<div class="empty-state">
						<strong>История подкастов пуста</strong>
						<span>Запустите выпуск из библиотеки или подкастового луча.</span>
					</div>
				{/if}
			</div>
		{:else}
			<div class="list">
				{#if appState.history?.length}
					{#each appState.history as item}
						<button
							class:itemCurrent={rowCurrent(item.track.id)}
							class="row action-row track-row"
							on:click={() => playOrToggle(item.track.id, "ray")}
							on:contextmenu={(event) => openTrackMenu(event, item.track.id, "history")}
						>
							<div class="cover-wrapper">
								<div class="cover"></div>
								<div class="cover-play-indicator">
									<span class:icon-playing={rowCurrent(item.track.id)}>
										{rowIcon(item.track.id)}
									</span>
									{#if rowRaySeed(item.track.id)}
										<Sparkles
											class="ray-seed-icon"
											size={14}
											title="Seed track for current ray"
										/>
									{/if}
								</div>
							</div>
							<div class="meta">
								<strong>{item.track.title}</strong>
								<TrackMetaLine track={item.track} maxGenres={2} showBpm={true} />
								<small class="history-track-state">
									{item.playedAtLabel} ·
									{item.progressLabel} из
									{item.track.durationLabel}
								</small>
								<div class="progress" style="margin-top: 6px;">
									<i style={`--w:${Math.round((item.progress ?? 0.4) * 100)}%`}></i>
								</div>
							</div>
							<div class="tail">
								{item.track.durationLabel}
							</div>
							<span class="track-menu-wrap">
								<span
									class="track-menu-btn"
									role="button"
									tabindex="0"
									aria-label="Меню трека"
									on:click={(event) => openTrackMenu(event, item.track.id, "history")}
									on:keydown={(event) =>
										(event.key === "Enter" || event.key === " ") &&
										openTrackMenu(event, item.track.id, "history")}>⋯</span
								>
							</span>
						</button>
					{/each}
				{:else}
					<div class="empty-state">
						<strong>История пуста</strong><span
							>Запусти трек из поиска — он появится здесь.</span
						>
					</div>
				{/if}
			</div>
		{/if}
	</div>
</section>
