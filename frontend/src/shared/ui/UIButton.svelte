<script lang="ts">
import clsx from "clsx";

export let primary = false;
export let compact = false;
export let disabled = false;
export let type: "button" | "submit" | "reset" = "button";
export let className = "";
</script>

<button
  {type}
  class={clsx("ui-button", className, { primary, compact })}
  {disabled}
  on:click
>
  <slot />
</button>
