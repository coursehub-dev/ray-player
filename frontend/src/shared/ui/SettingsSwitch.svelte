<script lang="ts">
import clsx from "clsx";
import { createEventDispatcher } from "svelte";

export let checked = false;
export let disabled = false;
export let title = "";
export let description = "";

const dispatch = createEventDispatcher<{
	change: { checked: boolean };
}>();

const toggle = () => {
	if (disabled) {
		return;
	}

	checked = !checked;
	dispatch("change", {
		checked,
	});
};

const handleKeydown = (event: KeyboardEvent) => {
	if (event.key !== "Enter" && event.key !== " ") {
		return;
	}

	event.preventDefault();
	toggle();
};
</script>

<div class={clsx("settings-switch-row", { disabled })}>
    <button
        type="button"
        class="settings-switch-copy"
        disabled={disabled}
        on:click={toggle}
    >
        <strong>{title}</strong>

        {#if description}
            <span>{description}</span>
        {/if}
    </button>

    <button
        type="button"
        class={clsx("ios-switch", { checked })}
        role="switch"
        aria-checked={checked}
        aria-label={title}
        disabled={disabled}
        on:click={toggle}
        on:keydown={handleKeydown}
    >
        <span class="ios-switch-thumb"></span>
    </button>
</div>
