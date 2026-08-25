<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import { animate, stagger } from "motion";
  import { GridIcon, LayersIcon, MousePointerIcon } from "svelte-feather-icons";

  export let mode: "floating" | "tiled" = "floating";
  export let onChange: (mode: "floating" | "tiled") => void = () => {};

  let floatingButton: HTMLButtonElement;
  let tiledButton: HTMLButtonElement;
  let cleanup: (() => void) | undefined;

  function setMode(nextMode: "floating" | "tiled") {
    if (nextMode === mode) return;
    mode = nextMode;
    onChange(mode);
    void animateControl();
  }

  async function animateControl() {
    await tick();
    cleanup?.();
    // Animate each explicitly bound DOM element. This keeps the component in
    // Svelte's ownership and avoids passing a NodeList/collection to Motion.
    const controls = [floatingButton, tiledButton].filter(Boolean);
    if (!controls.length) return;
    const animation = animate(
      controls,
      {
        transform: ["translateY(-3px) scale(.96)", "translateY(0) scale(1)"],
        opacity: [0.65, 1],
      },
      { duration: 0.28, delay: stagger(0.045), ease: [0.16, 1, 0.3, 1] },
    );
    cleanup = () => animation.stop();
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
      <span>自由</span>
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
</div>

<style lang="postcss">
  .workspace-mode {
    @apply relative flex items-center gap-1 rounded-xl border border-white/10 bg-zinc-950/75 p-1 shadow-2xl backdrop-blur-xl;
    box-shadow:
      0 16px 40px rgb(0 0 0 / 0.3),
      inset 0 1px rgb(255 255 255 / 0.06);
  }

  .mode-group {
    @apply flex items-center rounded-lg bg-black/30 p-0.5;
  }
  .mode-group button {
    @apply flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium text-zinc-400 transition duration-200 hover:text-white focus:outline-none focus:ring-2 focus:ring-cyan-300/60;
  }
  .mode-group button.active {
    @apply bg-white/10 text-white;
    box-shadow:
      inset 0 1px rgb(255 255 255 / 0.1),
      0 4px 12px rgb(0 0 0 / 0.2);
  }
  @keyframes appear {
    from {
      opacity: 0;
      transform: translateY(-5px) scale(0.96);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }
</style>
