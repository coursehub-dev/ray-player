<script>
export let primary = false;
export let compact = false;
export let disabled = false;
export let type = "button";
export let className = "";
</script>

<button
  {type}
  class={`ui-button ${className}`}
  class:primary
  class:compact
  {disabled}
  on:click
>
  <slot />
</button>
