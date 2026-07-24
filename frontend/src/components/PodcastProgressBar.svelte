<script>
export let item = null;
export let className = "";

const clamp = (value) => Math.max(0, Math.min(1, value));

$: stored = Number(item?.completedRatio);
$: position = Number(item?.lastPosition) || 0;
$: duration = Number(item?.duration) || 0;
$: progress = Number.isFinite(stored) && stored > 0 ? clamp(stored) : duration > 0 ? clamp(position / duration) : 0;
$: percentage = Math.round(progress * 100);
</script>

<span
    class={`podcast-progress-track ${className}`.trim()}
    aria-label={`Прослушано ${percentage}%`}
>
    <span
        class:completed={progress >= 0.95}
        class="podcast-progress-fill"
        style={`width:${percentage}%`}
    ></span>
</span>
