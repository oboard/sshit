<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import { animate } from "motion";

  import CircleButton from "$lib/ui/CircleButton.svelte";
  import CircleButtons from "$lib/ui/CircleButtons.svelte";

  export let id: number;
  export let title: string;
  export let x: number;
  export let y: number;
  export let width: number;
  export let height: number;
  export let zIndex = 1;
  export let focused = false;
  export let background = "#111111";
  export let ariaLabel = title;
  export let resizeLabel = `Resize ${title}`;
  export let tiled = false;
  export let layoutAnimating = false;
  export let onClose: (id: number) => void = () => {};
  export let onYellow: (id: number) => void = () => {};
  export let onGreen: (id: number) => void = () => {};
  export let onFocus: (id: number) => void = () => {};
  export let onStartMove: (id: number, event: PointerEvent) => void = () => {};
  export let onStartResize: (id: number, event: PointerEvent, width: number, height: number) => void = () => {};

  let previousTiled = tiled;
  let chromeVisible = true;
  let chromeEl: HTMLDivElement;
  let chromeTimer: number | undefined;
  let chromeAnimation: { stop: () => void } | undefined;

  async function transitionChrome(nextTiled: boolean) {
    chromeAnimation?.stop();
    window.clearTimeout(chromeTimer);
    chromeVisible = true;
    await tick();
    if (!chromeEl) return;

    if (nextTiled) {
      chromeAnimation = animate(
        chromeEl,
        { opacity: [1, 0], height: [42, 0], transform: ["translateY(0)", "translateY(-10px)"] },
        { duration: 0.48, ease: [0.16, 1, 0.3, 1] },
      );
      chromeTimer = window.setTimeout(() => (chromeVisible = false), 480);
    } else {
      chromeAnimation = animate(
        chromeEl,
        { opacity: [0, 1], height: [0, 42], transform: ["translateY(-10px)", "translateY(0)"] },
        { duration: 0.48, ease: [0.16, 1, 0.3, 1] },
      );
    }
  }

  // Keep the title bar mounted long enough for Motion to animate it away.
  $: if (tiled !== previousTiled) {
    previousTiled = tiled;
    void transitionChrome(tiled);
  }

  onDestroy(() => {
    window.clearTimeout(chromeTimer);
    chromeAnimation?.stop();
  });
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="window-frame absolute inline-block select-none rounded-xl border border-white/10 opacity-90 shadow-2xl transition-[opacity,box-shadow] duration-200"
  class:tile-animating={layoutAnimating}
  class:frameless={tiled}
  class:focused
  class:tiled
  class:interacting={focused}
  style="transform: translate({x}px, {y}px); z-index: {zIndex}; background: {background};"
  data-no-pan
  role="group"
  aria-label={ariaLabel}
  on:pointerdown|stopPropagation={() => onFocus(id)}
  on:wheel|stopPropagation
>
  {#if chromeVisible}
    <!-- touch-action: none keeps the browser from stealing window drags. -->
    <div
      bind:this={chromeEl}
      class="flex cursor-move select-none items-center overflow-hidden"
      style="touch-action: none;"
      role="toolbar"
      tabindex="-1"
      aria-label="{title} window controls"
      on:pointerdown|stopPropagation={(event) => { if (!tiled) onStartMove(id, event); }}
    >
      <div class="flex flex-1 items-center px-3">
        <CircleButtons>
          <CircleButton kind="red" on:pointerdown={(event) => event.button === 0 && onClose(id)} />
          <CircleButton kind="yellow" on:pointerdown={(event) => event.button === 0 && onYellow(id)} />
          <CircleButton kind="green" on:pointerdown={(event) => event.button === 0 && onGreen(id)} />
        </CircleButtons>
      </div>
      <div class="w-0 flex-grow-[4] overflow-hidden text-ellipsis whitespace-nowrap p-2 text-center text-sm font-medium text-zinc-300">
        {title}
      </div>
      <div class="flex-1"></div>
    </div>
  {/if}

  <slot />

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="resize-handle absolute -bottom-1 -right-1 h-5 w-5 rounded-sm"
    class:invisible={tiled}
    style="touch-action: none;"
    role="separator"
    aria-label={resizeLabel}
    title={resizeLabel}
    on:pointerdown|stopPropagation={(event) => onStartResize(id, event, width, height)}
  ></div>
</div>

<style lang="postcss">
  .window-frame {
    box-shadow: 0 20px 45px rgb(0 0 0 / 0.28), inset 0 1px rgb(255 255 255 / 0.05);
  }

  .focused {
    @apply opacity-100 ring-1 ring-cyan-300/70;
    box-shadow: 0 0 0 1px rgb(34 211 238 / 0.18), 0 22px 54px rgb(0 0 0 / 0.38), 0 0 34px rgb(34 211 238 / 0.1);
  }

  /* Window coordinates update continuously during a drag. Never transition
     transform: otherwise each update chases the pointer and feels viscous. */
  .interacting { transition-property: opacity, box-shadow; }
  .tiled { @apply rounded-none; }
  .frameless { border-color: transparent; box-shadow: none; }
  .tile-animating { transition: transform 480ms cubic-bezier(.16, 1, .3, 1), opacity 320ms cubic-bezier(.22, 1, .36, 1), box-shadow 320ms cubic-bezier(.22, 1, .36, 1); }


  .resize-handle {
    cursor: se-resize;
    cursor: nwse-resize;
  }

  .resize-handle::after {
    content: "";
    @apply absolute bottom-1 right-1 h-2.5 w-2.5 border-b-2 border-r-2 border-zinc-500;
    pointer-events: none;
  }

  /* Fingers need a much larger grab target than a mouse cursor. */
  @media (pointer: coarse) {
    .resize-handle {
      @apply -bottom-2 -right-2 h-9 w-9;
    }

    .resize-handle::after {
      @apply bottom-2 right-2 h-3 w-3 border-zinc-400;
    }
  }
</style>
