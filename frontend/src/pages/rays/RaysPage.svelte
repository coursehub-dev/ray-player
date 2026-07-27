<script lang="ts">
import { Settings, Mic } from "@lucide/svelte";
import { IconButton } from "../../shared/ui";

export let libraryMode: string = "music";
export let appState: any = {};
export let indexing: any = {};

export let openSettings: () => void = () => {};
export let openPodcastRayHistory: (rayId: string) => void = () => {};
export let resumeRay: (rayId: string) => void = () => {};
export let podcastRayContentLabel: (mode: string) => string = () => "";
export let podcastRaySortLabel: (mode: string) => string = () => "";
</script>

<section class="screen active">
	<div class="screen-head">
		<div>
			<h1>{libraryMode === "podcast" ? "История подкастовых лучей" : "История лучей"}</h1>
			<p>
				{libraryMode === "podcast"
					? "Сохранённые смысловые маршруты, их режимы наполнения и ручной порядок."
					: "Ранее собранные лучи. Нажатие активирует луч и возвращает к нему с сохранением позиции."}
			</p>
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
			<div class="podcast-ray-history-list">
				{#if (appState.podcastRays || []).length}
					{#each appState.podcastRays as ray}
						<button
							type="button"
							class="podcast-ray-history-row"
							class:current={ray.id === appState.podcastRay?.id}
							on:click={() => openPodcastRayHistory(ray.id)}
						>
							<span class="podcast-ray-history-icon"><Mic size={19} strokeWidth={1.6} /></span>
							<span class="podcast-ray-history-copy">
								<strong>{ray.title || ray.seed.title}</strong>
								<small>{ray.seed.series || ray.seed.author || ray.folderScope || "Подкастовый луч"}</small>
								<span class="podcast-ray-history-badges">
									<i>{podcastRayContentLabel(ray.contentMode)}</i>
									<i>{podcastRaySortLabel(ray.sortMode)}</i>
									{#if ray.isManualOrder}
										<i>Ручной порядок</i>
									{/if}
									{#if ray.parentRayId}
										<i>Версия {ray.revision}</i>
									{/if}
								</span>
							</span>
							<span class="podcast-ray-history-tail">
								<strong>{ray.itemCount}</strong>
								<small>выпусков</small>
								<small>{ray.createdAtLabel}</small>
							</span>
						</button>
					{/each}
				{:else}
					<div class="empty-state">
						<strong>История подкастовых лучей пуста</strong>
						<span>Запустите подкаст, чтобы построить первый смысловой маршрут.</span>
					</div>
				{/if}
			</div>
		{:else}
			<div class="ray-archive">
				{#if appState.rays?.length}
					{#each appState.rays as ray}
						<button class:active={ray.active} class="ray-card" on:click={() => resumeRay(ray.id)}>
							<div>
								<strong>{ray.name}</strong>
								<span
									>{ray.trackCount} треков · остановлено на {ray.currentTrackName} · {ray.resumeLabel}</span
								>
							</div>
							<div class="ray-status">
								{ray.active ? "текущий" : "архив"}
							</div>
						</button>
					{/each}
				{:else}
					<div class="empty-state">
						<strong>История лучей пуста</strong><span
							>После первого запуска трека здесь появится сохранённый луч.</span
						>
					</div>
				{/if}
			</div>
		{/if}
	</div>
</section>
