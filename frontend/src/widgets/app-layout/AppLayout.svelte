<script lang="ts">
export let indexing = false;
export let visualMode: "music" | "podcast" = "music";
export let shellStyle = "";
export let compact = false;
export let sidebarCollapsed = false;
</script>

<div
	class="app app-shell"
	class:compact-player={compact}
	class:sidebar-collapsed={sidebarCollapsed}
	class:indexing
	class:mode-music={visualMode === "music"}
	class:mode-podcast={visualMode === "podcast"}
	style={shellStyle}
>
	<aside class="sidebar">
		<slot name="sidebar" />
	</aside>

	<main class="content">
		<slot name="main" />
	</main>

	<div class="app-layout-player">
		<slot name="player" />
	</div>
</div>
