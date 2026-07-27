<script lang="ts">
import clsx from "clsx";

export let active: boolean = false;
export let disabled: boolean = false;
export let title: string = "";
export let className: string = "";
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
