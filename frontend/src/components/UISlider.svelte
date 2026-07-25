<script>
import { createEventDispatcher } from "svelte";
import { clampSliderValue, sliderValueFromKey, snapSliderValue } from "../lib/sliderUi.js";

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
let dragStartValue = value;
let trackEl;

$: if (!dragging) localValue = clampSliderValue(value, min, max);
$: clamped = clampSliderValue(localValue, min, max);
$: ratio = max > min ? (clamped - min) / (max - min) : 0;
$: fillPercent = ratio * 100;
$: displayValue = format === "percent" ? `${Math.round(ratio * 100)}%` : String(clamped);

function valueFromPointer(event) {
	if (!trackEl) return clamped;

	const rect = trackEl.getBoundingClientRect();
	const x = Math.min(rect.width, Math.max(0, event.clientX - rect.left));
	const raw = min + (x / Math.max(rect.width, 1)) * (max - min);
	return snapSliderValue(raw, min, max, step);
}

function emitPreview(nextValue) {
	localValue = nextValue;
	dispatch("input", nextValue);
	dispatch("preview", nextValue);
}

function emitCommit(nextValue) {
	dispatch("change", nextValue);
	dispatch("commit", nextValue);
}

function startDrag(event) {
	if (disabled || (event.pointerType === "mouse" && event.button !== 0)) return;

	event.preventDefault();
	dragStartValue = clampSliderValue(value, min, max);
	dragging = true;
	trackEl?.focus();
	trackEl?.setPointerCapture?.(event.pointerId);
	emitPreview(valueFromPointer(event));
}

function moveDrag(event) {
	if (!dragging || disabled) return;
	emitPreview(valueFromPointer(event));
}

function endDrag(event) {
	if (!dragging || disabled) return;

	const nextValue = valueFromPointer(event);
	dragging = false;
	emitPreview(nextValue);
	emitCommit(nextValue);
	trackEl?.releasePointerCapture?.(event.pointerId);
}

function cancelDrag(event) {
	if (!dragging) return;

	const restoredValue = dragStartValue;
	dragging = false;
	emitPreview(restoredValue);
	trackEl?.releasePointerCapture?.(event.pointerId);
}

function handleKeydown(event) {
	if (disabled) return;

	const nextValue = sliderValueFromKey({
		key: event.key,
		value: clamped,
		min,
		max,
		step,
	});
	if (nextValue === null) return;

	event.preventDefault();
	emitPreview(nextValue);
	emitCommit(nextValue);
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
		role="slider"
		tabindex={disabled ? -1 : 0}
		aria-label={label || "Значение"}
		aria-orientation="horizontal"
		aria-valuemin={min}
		aria-valuemax={max}
		aria-valuenow={clamped}
		aria-valuetext={displayValue}
		aria-disabled={disabled}
		on:keydown={handleKeydown}
		on:pointerdown={startDrag}
		on:pointermove={moveDrag}
		on:pointerup={endDrag}
		on:pointercancel={cancelDrag}
	>
		<div class="media-slider-track">
			<div class="media-slider-fill" style={`width:${fillPercent}%`}></div>
			<div class="media-slider-thumb" style={`left:${fillPercent}%`}></div>
		</div>
	</div>
</div>
