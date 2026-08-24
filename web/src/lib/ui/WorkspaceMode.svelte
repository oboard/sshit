<script lang="ts">
  import { createEventDispatcher, onDestroy, tick } from "svelte";
  import { animate, stagger } from "motion";
  import { GridIcon, LayersIcon, Maximize2Icon, MonitorIcon, MousePointerIcon } from "svelte-feather-icons";
  import type { WindowState } from "$lib/protocol";

  export let mode: "floating" | "tiled" = "floating";
  export let windows: WindowState[] = [];

  const dispatch = createEventDispatcher<{
    change: { mode: "floating" | "tiled" };
    arrange: void;
  }>();
  let floatingButton: HTMLButtonElement;
  let tiledButton: HTMLButtonElement;
  let arrangeButton: HTMLButtonElement;
  let tileActive = false;
  let cleanup: (() => void) | undefined;

  $: windowCount = windows.length;

  function setMode(nextMode: "floating" | "tiled") {
    if (nextMode === mode) return;
    mode = nextMode;
    dispatch("change", { mode });
    void animateControl();
  }

  async function animateControl() {
    await tick();
    cleanup?.();
    // Animate each explicitly bound DOM element. This keeps the component in
    // Svelte's ownership and avoids passing a NodeList/collection to Motion.
    const controls = [floatingButton, tiledButton, arrangeButton].filter(Boolean);
    if (!controls.length) return;
    const animation = animate(
      controls,
      { transform: ["translateY(-3px) scale(.96)", "translateY(0) scale(1)"], opacity: [0.65, 1] },
      { duration: 0.28, delay: stagger(0.045), ease: [0.16, 1, 0.3, 1] },
    );
    cleanup = () => animation.stop();
  }

  function arrangeTiles() {
    tileActive = true;
    dispatch("arrange");
    window.setTimeout(() => (tileActive = false), 460);
  }

  onDestroy(() => cleanup?.());
</script>

<div class="workspace-mode" data-no-pan aria-label="Window layout controls">
  <div class="mode-group" role="group" aria-label="Window layout mode">
    <button
      bind:this={floatingButton}
      class:active={mode === "floating"}
      aria-pressed={mode === "floating"}
      title="Floating windows — drag any title bar"
      on:click={() => setMode("floating")}
    >
      <LayersIcon size="15" strokeWidth="1.9" />
      <span>浮动</span>
    </button>
    <button
      bind:this={tiledButton}
      class:active={mode === "tiled"}
      aria-pressed={mode === "tiled"}
      title="Tiled windows — automatically arrange the workspace"
      on:click={() => setMode("tiled")}
    >
      <GridIcon size="15" strokeWidth="1.9" />
      <span>平铺</span>
    </button>
  </div>

  <button bind:this={arrangeButton} class="tile-action" title="Arrange tiled windows" disabled={mode !== "tiled" || windowCount < 2} on:click={arrangeTiles}>
    <Maximize2Icon size="14" strokeWidth="1.9" />
    <span>{windowCount || 0}</span>
  </button>

  {#if tileActive}
    <div class="layout-toast" role="status">
      <MonitorIcon size="15" />
      <span>Hyprland layout synced</span>
    </div>
  {/if}
</div>

<div class="mode-hint" class:tiled={mode === "tiled"}>
  {#if mode === "floating"}
    <MousePointerIcon size="13" /> <span>自由窗口</span>
  {:else}
    <GridIcon size="13" /> <span>自动平铺</span>
  {/if}
</div>

<style lang="postcss">
  .workspace-mode {
    @apply relative flex items-center gap-1 rounded-xl border border-white/10 bg-zinc-950/75 p-1 shadow-2xl backdrop-blur-xl;
    box-shadow: 0 16px 40px rgb(0 0 0 / 0.3), inset 0 1px rgb(255 255 255 / 0.06);
  }

  .mode-group { @apply flex items-center rounded-lg bg-black/30 p-0.5; }
  .mode-group button, .tile-action {
    @apply flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium text-zinc-400 transition duration-200 hover:text-white focus:outline-none focus:ring-2 focus:ring-cyan-300/60;
  }
  .mode-group button.active {
    @apply bg-white/10 text-white;
    box-shadow: inset 0 1px rgb(255 255 255 / 0.1), 0 4px 12px rgb(0 0 0 / 0.2);
  }
  .tile-action { @apply gap-1 border-l border-white/10 px-2 text-cyan-200 hover:bg-cyan-300/10 disabled:cursor-not-allowed disabled:opacity-35 disabled:hover:bg-transparent; }
  .layout-toast {
    @apply absolute right-0 top-[calc(100%+10px)] flex w-max items-center gap-2 rounded-lg border border-cyan-300/20 bg-zinc-950/90 px-3 py-2 text-[11px] text-cyan-100 shadow-xl backdrop-blur;
    animation: appear 460ms cubic-bezier(.16,1,.3,1) both;
  }
  .mode-hint {
    @apply absolute right-0 top-[calc(100%+14px)] flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-[.18em] text-zinc-500;
  }
  .mode-hint.tiled { @apply text-cyan-300; }
  @keyframes appear { from { opacity: 0; transform: translateY(-5px) scale(.96); } to { opacity: 1; transform: translateY(0) scale(1); } }
</style>
