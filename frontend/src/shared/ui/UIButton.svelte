<script lang="ts">
import clsx from "clsx";

export let primary: boolean = false;
export let compact: boolean = false;
export let disabled: boolean = false;
export let type: "button" | "submit" | "reset" = "button";
export let className: string = "";
</script>

<button
  {type}
  class={clsx("ui-button", className, { primary, compact })}
  {disabled}
  on:click
>
  <slot />
</button>
