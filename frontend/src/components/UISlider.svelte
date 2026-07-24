<script>
import { createEventDispatcher } from "svelte";

export let value = 1;
export let min = 0;
export let max = 1;
export let step = 0.01;
export let label = "";
export let disabled = false;
export let showValue = true;
export let format = "percent";
export let id = "";
export let accentReactive = false;

const dispatch = createEventDispatcher();
let dragging = false;
let localValue = value;
let trackEl;

$: if (!dragging) localValue = Number(value) || 0;
$: clamped = Math.max(min, Math.min(max, Number(localValue) || 0));
$: ratio = max > min ? (clamped - min) / (max - min) : 0;
$: fillPercent = ratio * 100;
$: displayValue = format === "percent" ? `${Math.round(ratio * 100)}%` : String(clamped);

function valueFromPointer(event) {
	const rect = trackEl.getBoundingClientRect();
	const x = Math.min(rect.width, Math.max(0, event.clientX - rect.left));
	const raw = min + (x / Math.max(rect.width, 1)) * (max - min);
	const stepped = step > 0 ? Math.round(raw / step) * step : raw;
	return Math.max(min, Math.min(max, stepped));
}

function startDrag(event) {
	if (disabled) return;
	dragging = true;
	trackEl?.setPointerCapture?.(event.pointerId);
	localValue = valueFromPointer(event);
	dispatch("input", localValue);
	dispatch("preview", localValue);
}

function moveDrag(event) {
	if (!dragging || disabled) return;
	localValue = valueFromPointer(event);
	dispatch("input", localValue);
	dispatch("preview", localValue);
}

function endDrag(event) {
	if (!dragging || disabled) return;
	dragging = false;
	localValue = valueFromPointer(event);
	dispatch("change", localValue);
	dispatch("commit", localValue);
	trackEl?.releasePointerCapture?.(event.pointerId);
}
</script>

<div class="slider-row" data-slider-id={id}>
  {#if label}
    <div class="slider-label">
      <span>{label}</span>
      {#if showValue}<span class="slider-value">{displayValue}</span>{/if}
    </div>
  {/if}

  <div
    bind:this={trackEl}
    class:dragging
    class:accent-reactive={accentReactive}
    class="media-slider-hitarea"
    aria-disabled={disabled}
    on:pointerdown={startDrag}
    on:pointermove={moveDrag}
    on:pointerup={endDrag}
    on:pointercancel={endDrag}
  >
    <div class="media-slider-track">
      <div class="media-slider-fill" style={`width:${fillPercent}%`} />
      <div class="media-slider-thumb" style={`left:${fillPercent}%`} />
    </div>
  </div>
</div>
