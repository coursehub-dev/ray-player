<script>
export let active = false;
export let disabled = false;
export let title = "";
export let className = "";
export let type = "button";
</script>

<button
  {type}
  class={`icon-button ${className}`}
  class:active
  {disabled}
  aria-label={title}
  title={title}
  on:click
>
  <slot />
</button>
