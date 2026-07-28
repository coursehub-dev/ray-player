<script lang="ts">
import { Download, LoaderCircle, RotateCcw, X } from "@lucide/svelte";

export let open = false;
export let jobs: any[] = [];
export let reindex: any = {};
export let close: () => void = () => {};
export let cancelJob: (id: string) => void = () => {};
export let retryJob: (id: string) => void = () => {};

const active = (status: string) => ["queued", "downloading", "converting", "fetching_metadata"].includes(status);
const percent = (value: number) => Math.round(Math.max(0, Math.min(1, Number(value) || 0)) * 100);
</script>

{#if open}

	<div class="settings-overlay tasks-overlay" role="presentation" on:pointerdown={(event) => event.target === event.currentTarget && close()}>
		<div class="tasks-modal" role="dialog" aria-modal="true" aria-labelledby="tasks-title">
			<header class="tasks-head">
				<div><h2 id="tasks-title">Задачи</h2><p>Загрузки и фоновые операции.</p></div>
				<button type="button" class="icon-button" on:click={close} aria-label="Закрыть"><X size={18} /></button>
			</header>
			<div class="tasks-list">
				{#if reindex.active || reindex.message}
					<div class="task-row">
						<span class="task-icon">{#if reindex.active}<LoaderCircle class="task-spin" size={17} />{:else}<RotateCcw size={17} />{/if}</span>
						<div class="task-copy"><strong>Переиндексация библиотеки</strong><span>{reindex.message || reindex.stage || "Готово"}</span></div>
						<div class="task-progress"><span>{reindex.total ? `${reindex.index}/${reindex.total}` : ""}</span></div>
					</div>
				{/if}

				{#each jobs as job}
					<div class="task-row">
						<span class="task-icon">{#if active(job.status)}<LoaderCircle class="task-spin" size={17} />{:else}<Download size={17} />{/if}</span>
						<div class="task-copy">
							<strong>{job.title || job.url}</strong>
							<span>{job.libraryType === "podcast" ? "Подкаст" : "Музыка"} · {job.status}{job.error ? ` · ${job.error}` : ""}</span>
							<div class="task-progress-bar"><i style={`width:${percent(job.progress)}%`}></i></div>
						</div>
						<div class="task-actions">
							{#if active(job.status)}<button type="button" on:click={() => cancelJob(job.id)}>Отмена</button>{/if}
							{#if job.status === "error" || job.status === "canceled"}<button type="button" on:click={() => retryJob(job.id)}>Повторить</button>{/if}
						</div>
					</div>
				{/each}

				{#if !reindex.active && !reindex.message && jobs.length === 0}
					<div class="empty-state"><strong>Активных задач нет</strong><span>Здесь появятся yt-dlp загрузки и переиндексация.</span></div>
				{/if}
			</div>
		</div>
	</div>
{/if}
