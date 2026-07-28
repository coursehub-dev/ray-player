<script lang="ts">
import { Bell, Play, Search, Settings, X } from "@lucide/svelte";
import { IconButton } from "../../shared/ui";
import { shouldShowSuggestions } from "./searchUi";

export let libraryMode = "music";
export let query = "";
export let moodFilter = "";
export let suggestions: any[] = [];
export let focused = false;
export let taskCount = 0;
export let inputEl: HTMLInputElement | null = null;
export let onInput: (value: string) => void = () => {};
export let onFocus: () => void = () => {};
export let onBlur: () => void = () => {};
export let onMood: (value: string) => void = () => {};
export let onStart: (item: any) => void = () => {};
export let openTasks: () => void = () => {};
export let openSettings: () => void = () => {};

const moodTitle: Record<string, string> = {
	combat: "Combat / pressure",
	joy_party: "Joy / party",
	night_smooth: "Night / melancholy",
	serene: "Serene / calm",
};

$: showSuggestions = shouldShowSuggestions({
	focused,
	query,
	count: suggestions.length,
});
</script>

<div class="global-search-shell">
	<div class="global-search-row">
		<div class="global-search-slot">
			<div class="global-search-field" class:focused>
				<Search size={17} strokeWidth={1.8} />
				<input
					bind:this={inputEl}
					value={query}
					placeholder={libraryMode === "podcast" ? "Искать подкаст…" : "Искать трек, артиста, альбом…"}
					on:input={(event) => onInput(event.currentTarget.value)}
					on:focus={onFocus}
					on:blur={onBlur}
				/>
				{#if focused && libraryMode === "music"}
					<div class="mood-filter-row" aria-label="Фильтр настроения">
						{#each ["combat", "joy_party", "night_smooth", "serene"] as mood}
							<button
								type="button"
								class={`mood-orb mood-${mood}`}
								class:active={moodFilter === mood}
								aria-label={moodTitle[mood]}
								aria-pressed={moodFilter === mood}
								title={moodTitle[mood]}
								on:mousedown|preventDefault={() => onMood(moodFilter === mood ? "" : mood)}
							></button>
						{/each}
						{#if moodFilter}
							<button
								type="button"
								class="mood-clear"
								aria-label="Сбросить фильтр настроения"
								title="Сбросить фильтр"
								on:mousedown|preventDefault={() => onMood("")}
							>
								<X size={12} />
							</button>
						{/if}
					</div>
				{/if}
				<span class="global-kbd">⌘ / Ctrl + K</span>
			</div>

			{#if showSuggestions}
				<div class="global-suggestions" role="listbox" tabindex="-1" on:mousedown|preventDefault>
					{#each suggestions.slice(0, 5) as item}
						<button
							type="button"
							role="option"
							aria-selected="false"
							aria-label={`Начать луч с ${item.track?.title || "трека"}`}
							class={`global-suggestion mood-${item.moodGroup || "unknown"}`}
							on:click={() => onStart(item)}
						>
							<span class="suggestion-copy">
								<strong>{item.track?.title || "Без названия"}</strong>
								<small>
									{[item.track?.artist, item.track?.album].filter(Boolean).join(" · ") || "Локальная библиотека"}
								</small>
							</span>
							<span class="suggestion-action">начать луч</span>
							<span class="suggestion-play" aria-hidden="true"><Play size={15} strokeWidth={1.9} /></span>
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<div class="global-tools">
			<IconButton className="gear-btn task-bell" on:click={openTasks} title="Активные задачи">
				<Bell size={18} strokeWidth={1.8} />
				{#if taskCount > 0}<span class="task-count">{taskCount}</span>{/if}
			</IconButton>
			<IconButton className="gear-btn" on:click={openSettings} title="Системные настройки">
				<Settings size={18} strokeWidth={1.8} />
			</IconButton>
		</div>
	</div>
</div>
