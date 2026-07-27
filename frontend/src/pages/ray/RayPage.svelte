<script lang="ts">
import { Settings, Play, Pause, GripVertical, Trash2 } from "@lucide/svelte";
import { IconButton, UIButton } from "../../shared/ui";
import { RayTrackRow } from "../../entities/ray";
import { PodcastProgressBar } from "../../entities/podcast";
import { RayBuildSkeleton } from "../../widgets/ray-build-panel";

export let libraryMode: string = "music";
export let appState: any = {};
export let playback: any = {};
export let indexing: any = {};
export let currentTrack: any = {};
export let rayBuild: any = {};
export let selectedRayMode = "";
export let showInsight = false;
export let auditRows: any[] = [];
export let emoFlowDirectionLabel = "";
export let emoFlowEmotionLabel = "";
export let isRayBuilding = false;
export let currentQueueIndex = -1;
export let podcastRayUpdating = false;
export let podcastRayDropIndex = -1;
export let draggedPodcastRayIndex = -1;
export let musicRayUpdating = false;
export let musicRayDropIndex = -1;
export let draggedMusicRayIndex = -1;

export let openSettings: () => void = () => {};
export let toggleInsight: () => void = () => {};
export let setPodcastContentMode: (mode: string) => void = () => {};
export let setPodcastSortMode: (mode: string) => void = () => {};
export let setMusicContentMode: (mode: string) => void = () => {};
export let setMusicSortMode: (mode: string) => void = () => {};
export let overPodcastRayItem: (event: DragEvent, position: number) => void = () => {};
export let dropPodcastRayItem: (event: DragEvent, position: number) => void = () => {};
export let beginPodcastRayDrag: (event: DragEvent, position: number) => void = () => {};
export let finishPodcastRayDrag: () => void = () => {};
export let togglePodcastRow: (id: string, fromRay: boolean) => void = () => {};
export let removePodcastRayItem: (event: MouseEvent | KeyboardEvent, id: string) => void = () => {};
export let podcastMeta: (item: any) => string = () => "";
export let podcastProgressPercent: (item: any) => number = () => 0;
export let trackById: (id: string) => any = () => null;
export let playlistInsightLine: (item: any, index: number) => string = () => "";
export let trackDebugLine: (item: any) => string = () => "";
export let beginMusicRayDrag: (event: DragEvent, item: any, index: number) => void = () => {};
export let overMusicRayItem: (event: DragEvent, index: number) => void = () => {};
export let dropMusicRayItem: (event: DragEvent, index: number) => void = () => {};
export let finishMusicRayDrag: () => void = () => {};
export let playTrackFromQueue: (trackId: string) => void = () => {};
export let openTrackMenu: (event: MouseEvent | KeyboardEvent, trackId: string, source: string) => void = () => {};
</script>

{#if libraryMode === "podcast"}
	<section class="screen active podcast-ray-screen">
		<div class="screen-head">
			<div>
				<h1>Луч подкастов</h1>
				<p>
					{#if appState.podcastRay?.folderScope}
						Приоритет текущей папки:
						{appState.podcastRay.folderScope}
					{:else}
						Запустите выпуск, чтобы построить смысловой маршрут.
					{/if}
				</p>
			</div>
			<div class="top-right-status">
				<div class="library-status">{(appState.podcastRay?.items || []).length} episodes</div>
				<IconButton className="gear-btn" on:click={openSettings} title="Системные настройки"><Settings size={18} strokeWidth={1.8} /></IconButton>
			</div>
		</div>

		<div class="screen-body">
			{#if !(appState.podcastRay?.items || []).length}
				<div class="empty-state">
					<strong>Луч ещё не построен</strong>
					<span>
						Выберите подкаст в библиотеке. Сначала будут
						добавлены выпуски из той же папки, затем из
						соседних папок и серий.
					</span>
				</div>
			{:else}
				<div class="podcast-ray-toolbar">
					<label>
						<span>Наполнение</span>
						<select
							value={appState.podcastRay?.contentMode || "recommended"}
							disabled={podcastRayUpdating}
							on:change={(event) => setPodcastContentMode(event.currentTarget.value)}
						>
							<option value="recommended">Рекомендуемое</option>
							<option value="explore">Исследование</option>
							<option value="current_folder">Текущая папка</option>
						</select>
					</label>

					<label>
						<span>Сортировка</span>
						<select
							value={appState.podcastRay?.sortMode || "recommended"}
							disabled={podcastRayUpdating}
							on:change={(event) => setPodcastSortMode(event.currentTarget.value)}
						>
							<option value="recommended">Рекомендуемое</option>
							<option value="name_asc">Название A → Z</option>
							<option value="name_desc">Название Z → A</option>
							<option value="date_desc">Сначала новые</option>
							<option value="date_asc">Сначала старые</option>
							<option value="manual">Ручной порядок</option>
						</select>
					</label>

					{#if appState.podcastRay?.isManualOrder}
						<span class="podcast-manual-badge">Ручной порядок</span>
					{/if}
				</div>

				<div class="podcast-ray-list">
					{#each appState.podcastRay.items as rayItem}
						<button
							type="button"
							class="podcast-ray-row"
							class:current={rayItem.item.id === playback.currentTrackId}
							class:drop-target={podcastRayDropIndex === rayItem.position &&
								draggedPodcastRayIndex !== rayItem.position}
							on:dragover={(event) => overPodcastRayItem(event, rayItem.position)}
							on:drop={(event) => dropPodcastRayItem(event, rayItem.position)}
							on:click={() => togglePodcastRow(rayItem.item.id, true)}
						>
							<span
								class="podcast-ray-drag"
								draggable="true"
								role="button"
								tabindex="0"
								aria-label="Перетащить"
								title="Перетащить"
								on:click|stopPropagation
								on:keydown={(event) => {
									if (event.key === "Enter" || event.key === " ") {
										event.preventDefault();
										event.stopPropagation();
									}
								}}
								on:dragstart={(event) => beginPodcastRayDrag(event, rayItem.position)}
								on:dragend={finishPodcastRayDrag}
							>
								<GripVertical size={15} />
							</span>

							<span class="podcast-ray-position">{rayItem.position + 1}</span>

							<span class="podcast-ray-play">
								{#if rayItem.item.id === playback.currentTrackId && playback.status === "playing"}
									<Pause size={16} strokeWidth={2} />
								{:else}
									<Play size={16} strokeWidth={2} />
								{/if}
							</span>

							<span class="podcast-ray-copy">
								<strong>{rayItem.item.title}</strong>
								<small>{podcastMeta(rayItem.item) || "Локальный выпуск"}</small>
								<small class="podcast-ray-reason">{rayItem.reason}</small>
							</span>

							<span class="podcast-ray-tail">
								{#if podcastProgressPercent(rayItem.item) > 0}
									{podcastProgressPercent(rayItem.item)}%
								{:else}
									{rayItem.item.durationLabel}
								{/if}
							</span>

							{#if rayItem.item.id !== appState.podcastRay.seedItemId}
								<span
									class="podcast-ray-remove"
									role="button"
									tabindex="0"
									aria-label="Удалить из луча"
									title="Удалить из луча"
									on:click={(event) => removePodcastRayItem(event, rayItem.item.id)}
									on:keydown={(event) => {
										if (event.key === "Enter" || event.key === " ") {
											event.preventDefault();
											removePodcastRayItem(event, rayItem.item.id);
										}
									}}
								>
									<Trash2 size={14} />
								</span>
							{/if}

							<PodcastProgressBar item={rayItem.item} className="podcast-ray-progress" />
						</button>
					{/each}
				</div>
			{/if}
		</div>
	</section>
{:else}
	<section class="screen active">
		<div class="screen-head">
			<div>
				<h1>
					Луч{currentTrack.currentTitle ? ` · ${currentTrack.currentTitle}` : ""}
				</h1>
				<p>
					Текущий луч. Можно ткнуть в любой трек очереди и
					сразу перескочить на него.
				</p>
				<div class="badges" style="margin-top:8px; gap:8px;">
					<i class="badge emoflow">{emoFlowDirectionLabel}</i>
					{#if emoFlowEmotionLabel}
						<span class={`badge emotion emotion-${emoFlowEmotionLabel.replace(/ /g, "-")}`}>
							{emoFlowEmotionLabel}
						</span>
					{/if}
					<label class="badge genre">mode
						<select bind:value={selectedRayMode} style="margin-left:6px; background:transparent; color:inherit; border:none;">
							<option value="">auto</option>
							<option value="continue_mood">continue</option>
							<option value="warm_up">warm_up</option>
							<option value="cool_down">cool_down</option>
							<option value="explore">explore</option>
							<option value="deepen">deepen</option>
						</select>
					</label>
					<UIButton compact className="badge-height" on:click={toggleInsight}
						>{showInsight ? "hide insight" : "show insight"}</UIButton
					>
				</div>
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
			{#if appState.musicRay?.id || appState.queue?.length}
				<div class="ray-toolbar">
					<label>
						<span>Траектория</span>
						<select
							value={appState.musicRay?.contentMode || "stable"}
							disabled={musicRayUpdating}
							on:change={(event) => setMusicContentMode(event.currentTarget.value)}
						>
							<option value="stable">Ровный поток</option>
							<option value="warm_up">Разогрев</option>
							<option value="cool_down">Снижение</option>
							<option value="intensify">Интенсивнее</option>
							<option value="deepen">Глубже</option>
							<option value="explore">Исследование</option>
						</select>
					</label>

					<label>
						<span>Сортировка</span>
						<select
							value={appState.musicRay?.sortMode || "recommended"}
							disabled={musicRayUpdating}
							on:change={(event) => setMusicSortMode(event.currentTarget.value)}
						>
							<option value="recommended">Рекомендуемое</option>
							<option value="name_asc">Название A → Z</option>
							<option value="name_desc">Название Z → A</option>
							<option value="date_desc">Сначала новые</option>
							<option value="date_asc">Сначала старые</option>
							<option value="manual">Ручной порядок</option>
						</select>
					</label>

					{#if appState.musicRay?.isManualOrder}
						<span class="ray-manual-badge">Ручной порядок</span>
					{/if}
				</div>
			{/if}

			<div class="playlist">
				{#if isRayBuilding}
					<RayBuildSkeleton
						seedTitle={trackById(rayBuild.seedTrackId)?.title || playback.currentTitle}
					/>
				{:else if appState.queue?.length}
					{#each appState.queue as item, index}
						<div
							class:rowPast={currentQueueIndex >= 0 && index < currentQueueIndex}
							class:rowFuture={currentQueueIndex >= 0 && index > currentQueueIndex}
						>
							<RayTrackRow
								{item}
								{index}
								{playback}
								{showInsight}
								dropTarget={musicRayDropIndex === index && draggedMusicRayIndex !== index}
								dragging={draggedMusicRayIndex === index}
								insightLine={playlistInsightLine(item, index)}
								debugLine={trackDebugLine(item)}
								on:dragstart={(event) =>
									beginMusicRayDrag(event.detail.event, event.detail.item, event.detail.index)}
								on:dragover={(event) => overMusicRayItem(event.detail.event, event.detail.index)}
								on:drop={(event) => dropMusicRayItem(event.detail.event, event.detail.index)}
								on:dragend={finishMusicRayDrag}
								on:play={(event) => playTrackFromQueue(event.detail.trackId)}
								on:menu={(event) => openTrackMenu(event.detail.event, event.detail.trackId, "ray")}
							/>
						</div>
					{/each}
				{:else}
					<div class="empty-state">
						<strong>Луч ещё не запущен</strong><span
							>Выбери трек в поиске — он станет seed для
							базового луча.</span
						>
					</div>
				{/if}
				{#if showInsight && auditRows.length}
					<div class="settings-block" style="margin-top:16px;">
						<div class="small-head">Ray audit</div>
						{#each auditRows as row}
							<div class="probe-row probe-row-tight">
								<strong>{row.position}. {row.title}</strong>
								<span>{row.reason}</span>
								<small
									>sim {row.insight?.similarity?.toFixed?.(2) || "0.00"} · dist {row.insight
										?.moodDistance?.toFixed?.(2) || "0.00"} · jump {row.insight?.jumpPenalty?.toFixed?.(
										2,
									) || "0.00"} · Δe {row.insight?.energyDelta?.toFixed?.(2) || "0.00"} · tempo {row
										.insight?.tempoCompatibility?.toFixed?.(2) || "0.00"}</small
								>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	</section>
{/if}
