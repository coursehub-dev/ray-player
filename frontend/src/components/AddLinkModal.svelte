<script>
import { createEventDispatcher, tick } from "svelte";
import { Link, LoaderCircle, X } from "@lucide/svelte";

export let open = false;
export let libraryType = "music";
export let submitting = false;
export let error = "";

let url = "";
let urlInput;
const dispatch = createEventDispatcher();

$: quality = libraryType === "podcast" ? "128 kbps" : "192 kbps";
$: targetLabel = libraryType === "podcast" ? "Подкасты" : "Музыка";

$: if (!open) {
	url = "";
}

function close() {
	if (!submitting) {
		dispatch("close");
	}
}

function submit() {
	const value = url.trim();
	if (!value || submitting) {
		return;
	}
	dispatch("submit", { url: value });
}

$: if (open && !submitting) {
	tick().then(() => {
		if (open) {
			urlInput?.focus();
		}
	});
}
</script>

{#if open}
    <div
        class="modal-backdrop"
        role="presentation"
        on:click|self={close}
    >
        <section
            class="add-link-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="add-link-title"
        >
            <header>
                <span class="add-link-icon">
                    <Link size={18} strokeWidth={1.8} />
                </span>
                <div>
                    <h2 id="add-link-title">Добавить ссылку</h2>
                    <p>
                        yt-dlp загрузит аудио и добавит его в локальную библиотеку.
                    </p>
                </div>
                <button
                    type="button"
                    class="modal-close"
                    aria-label="Закрыть"
                    disabled={submitting}
                    on:click={close}
                >
                    <X size={17} />
                </button>
            </header>

            <form on:submit|preventDefault={submit}>
                <label>
                    <span>URL</span>
                    <input
                        bind:this={urlInput}
                        bind:value={url}
                        type="url"
                        autocomplete="off"
                        placeholder="https://youtube.com/watch?v=..."
                        disabled={submitting}
                    />
                </label>

                <div class="add-link-summary">
                    <span>Будет добавлено в: <strong>{targetLabel}</strong></span>
                    <span>MP3 · {quality}</span>
                </div>

                <p class="legal-note">
                    Убедитесь, что у вас есть право скачивать и использовать это аудио.
                </p>

                {#if error}
                    <p class="form-error">{error}</p>
                {/if}

                <footer>
                    <button
                        type="button"
                        class="secondary-button"
                        disabled={submitting}
                        on:click={close}
                    >
                        Отмена
                    </button>
                    <button
                        type="submit"
                        class="primary-button"
                        disabled={!url.trim() || submitting}
                    >
                        {#if submitting}
                            <LoaderCircle
                                class="spin"
                                size={16}
                            />
                            Проверяем ссылку…
                        {:else}
                            Добавить
                        {/if}
                    </button>
                </footer>
            </form>
        </section>
    </div>
{/if}
