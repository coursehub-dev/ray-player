<script lang="ts">
import { LoaderCircle, Sparkles } from "@lucide/svelte";

export let rows = 7;
export let seedTitle = "";
</script>

<section
    class="ray-build-panel accent-reactive"
    aria-live="polite"
    aria-busy="true"
>
    <header class="ray-build-header">
        <span class="ray-build-icon">
            <LoaderCircle size={17} strokeWidth={1.8} />
        </span>

        <div>
            <strong>Строим новый луч</strong>
            <span>
                {seedTitle
                    ? `Подбираем продолжение для «${seedTitle}»`
                    : "Анализируем переходы и настроение"}
            </span>
        </div>

        <Sparkles
            class="ray-build-sparkles"
            size={16}
            strokeWidth={1.6}
        />
    </header>

    <div class="ray-build-progress">
        <span></span>
    </div>

    <div class="ray-build-rows">
        {#each Array(rows) as _, index}
            <div
                class="ray-build-row"
                style={`--ray-build-index:${index}`}
            >
                <i class="ray-build-circle"></i>
                <div class="ray-build-lines">
                    <i class="ray-build-title"></i>
                    <i class="ray-build-meta"></i>
                </div>
                <i class="ray-build-badge"></i>
            </div>
        {/each}
    </div>
</section>
