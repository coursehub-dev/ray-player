<script lang="ts">
import clsx from "clsx";

export let active = false;
export let disabled = false;
export let title = "";
export let className = "";
export let type: "button" | "submit" | "reset" = "button";
</script>

<button
  {type}
  class={clsx("icon-button", className, { active })}
  {disabled}
  aria-label={title}
  title={title}
  on:click
>
  <slot />
</button>
