<script lang="ts">
import clsx from "clsx";
import { createEventDispatcher } from "svelte";
import { clampSliderValue, sliderValueFromKey, snapSliderValue } from "../lib/sliderUi.js";

export let value: number = 1;
export let min: number = 0;
export let max: number = 1;
export let step: number = 0.01;
export let label: string = "";
export let disabled: boolean = false;
export let showValue: boolean = true;
export let format: "percent" | "raw" = "percent";
export let id: string = "";
export let accentReactive: boolean = false;

const dispatch = createEventDispatcher<{
	input: number;
	preview: number;
	change: number;
	commit: number;
}>();

let dragging = false;
let localValue = value;
let dragStartValue = value;
let trackEl: HTMLDivElement | undefined;

$: if (!dragging) localValue = clampSliderValue(value, min, max);
$: clamped = clampSliderValue(localValue, min, max);
$: ratio = max > min ? (clamped - min) / (max - min) : 0;
$: fillPercent = ratio * 100;
$: displayValue = format === "percent" ? `${Math.round(ratio * 100)}%` : String(clamped);

function valueFromPointer(event: PointerEvent) {
	if (!trackEl) return clamped;

	const rect = trackEl.getBoundingClientRect();
	const x = Math.min(rect.width, Math.max(0, event.clientX - rect.left));
	const raw = min + (x / Math.max(rect.width, 1)) * (max - min);
	return snapSliderValue(raw, min, max, step);
}

function emitPreview(nextValue: number) {
	localValue = nextValue;
	dispatch("input", nextValue);
	dispatch("preview", nextValue);
}

function emitCommit(nextValue: number) {
	dispatch("change", nextValue);
	dispatch("commit", nextValue);
}

function startDrag(event: PointerEvent) {
	if (disabled || (event.pointerType === "mouse" && event.button !== 0)) return;

	event.preventDefault();
	dragStartValue = clampSliderValue(value, min, max);
	dragging = true;
	trackEl?.focus();
	trackEl?.setPointerCapture?.(event.pointerId);
	emitPreview(valueFromPointer(event));
}

function moveDrag(event: PointerEvent) {
	if (!dragging || disabled) return;
	emitPreview(valueFromPointer(event));
}

function endDrag(event: PointerEvent) {
	if (!dragging || disabled) return;

	const nextValue = valueFromPointer(event);
	dragging = false;
	emitPreview(nextValue);
	emitCommit(nextValue);
	trackEl?.releasePointerCapture?.(event.pointerId);
}

function cancelDrag(event: PointerEvent) {
	if (!dragging) return;

	const restoredValue = dragStartValue;
	dragging = false;
	emitPreview(restoredValue);
	trackEl?.releasePointerCapture?.(event.pointerId);
}

function handleKeydown(event: KeyboardEvent) {
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
		class={clsx("media-slider-hitarea", {
			dragging,
			"accent-reactive": accentReactive,
		})}
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
