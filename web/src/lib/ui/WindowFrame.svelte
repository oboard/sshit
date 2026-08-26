<script lang="ts">
  import { afterUpdate, beforeUpdate, onDestroy } from "svelte";
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
  export let tileResizing = false;
  export let tilePaneId: number | null = null;
  export let onClose: (id: number) => void = () => {};
  export let onYellow: (id: number) => void = () => {};
  export let onGreen: (id: number) => void = () => {};
  export let onFocus: (id: number) => void = () => {};
  export let onStartMove: (id: number, event: PointerEvent) => void = () => {};
  export let onStartTiledMove: (id: number, event: PointerEvent) => void = () => {};
  export let onMoveTiledMove: (event: PointerEvent) => void = () => {};
  export let onFinishTiledMove: () => void = () => {};
  export let onStartResize: (id: number, event: PointerEvent, width: number, height: number) => void = () => {};
  export let onTitlebarDoubleClick: (id: number) => void = () => {};

  let frameEl: HTMLDivElement;
  let tiledHandleEl: HTMLButtonElement;
  let tiledHandleStart: { x: number; y: number } | null = null;
  let tiledHandleDragged = false;

  function startTiledHandle(event: PointerEvent) {
    if (event.button !== 0) return;
    event.preventDefault();
    onFocus(id);
    tiledHandleStart = { x: event.clientX, y: event.clientY };
    tiledHandleDragged = false;
    tiledHandleEl?.setPointerCapture(event.pointerId);
    onStartTiledMove(id, event);
  }

  function moveTiledHandle(event: PointerEvent) {
    if (!tiledHandleStart) return;
    if (Math.hypot(event.clientX - tiledHandleStart.x, event.clientY - tiledHandleStart.y) > 8) {
      tiledHandleDragged = true;
    }
    onMoveTiledMove(event);
  }

  function finishTiledHandle(event: PointerEvent) {
    if (!tiledHandleStart) return;
    tiledHandleEl?.releasePointerCapture(event.pointerId);
    onFinishTiledMove();
    tiledHandleStart = null;
  }

  let contentEl: HTMLDivElement;
  let beforeRect: DOMRect | null = null;
  let flipAnimation: { stop: () => void } | undefined;

  // The dragged frame follows the cursor directly. All other tiled frames use
  // Motion's FLIP transform so they glide into the slots they are reordered to.
  $: isDraggedTile = tiled && zIndex >= 1000;

  beforeUpdate(() => {
    if (tiled && !layoutAnimating && !tileResizing && !isDraggedTile && frameEl) {
      beforeRect = frameEl.getBoundingClientRect();
    } else {
      beforeRect = null;
    }
  });

  afterUpdate(() => {
    // Mode changes already animate the outer coordinate transform. Clearing the
    // FLIP baseline here prevents an extra scale animation when that transition
    // finishes and `layoutAnimating` becomes false.
    if (!tiled || layoutAnimating || tileResizing || isDraggedTile || !frameEl || !contentEl) {
      beforeRect = null;
      return;
    }
    const before = beforeRect;
    const after = frameEl.getBoundingClientRect();
    beforeRect = null;
    if (!before) return;
    const deltaX = before.left - after.left;
    const deltaY = before.top - after.top;
    const scaleX = before.width / after.width;
    const scaleY = before.height / after.height;
    if (Math.abs(deltaX) < 0.5 && Math.abs(deltaY) < 0.5 && Math.abs(scaleX - 1) < 0.005 && Math.abs(scaleY - 1) < 0.005) return;
    flipAnimation?.stop();
    flipAnimation = animate(
      contentEl,
      { transform: [`translate(${deltaX}px, ${deltaY}px) scale(${scaleX}, ${scaleY})`, "translate(0px, 0px) scale(1, 1)"] },
      { duration: 0.32, ease: [0.16, 1, 0.3, 1] },
    );
  });

  onDestroy(() => flipAnimation?.stop());
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  bind:this={frameEl}
  class="window-frame absolute inline-block select-none"
  class:mode-animating={layoutAnimating}
  style="transform: translate({x}px, {y}px); z-index: {zIndex};"
  data-no-pan
  data-tile-pane={tilePaneId ?? undefined}
  role="group"
  aria-label={ariaLabel}
  on:pointerdown|stopPropagation={() => onFocus(id)}
  on:wheel|stopPropagation
>
  <div
    bind:this={contentEl}
    class="window-content rounded-xl border border-white/10 opacity-90 shadow-2xl transition-[opacity,box-shadow] duration-200"
    class:focused
    class:interacting={focused}
    style="background: {background};"
  >
  {#if tiled && !focused}
    <div class="tiled-window-controls">
      <button
        bind:this={tiledHandleEl}
        class="tiled-drag-handle"
        class:dragging={tiledHandleStart !== null}
        type="button"
        aria-label={`Move or focus ${title}`}
        title="Drag to rearrange; click to show window controls"
        style="touch-action: none;"
        on:pointerdown|stopPropagation={startTiledHandle}
        on:pointermove|stopPropagation={moveTiledHandle}
        on:pointerup|stopPropagation={finishTiledHandle}
        on:pointercancel|stopPropagation={finishTiledHandle}
      ><span></span><span></span><span></span></button>
    </div>
  {:else}
    <!-- The focused tiled window restores the regular window titlebar. -->
    <div
      class="flex cursor-move select-none items-center overflow-hidden"
      style="touch-action: none;"
      role="toolbar"
      tabindex="-1"
      aria-label="{title} window controls"
      on:pointerdown|stopPropagation={(event) => !tiled && onStartMove(id, event)}
      on:dblclick|stopPropagation={() => !tiled && onTitlebarDoubleClick(id)}
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

  {#if !tiled}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="resize-handle absolute -bottom-1 -right-1 h-5 w-5 rounded-sm"
      style="touch-action: none;"
      role="separator"
      aria-label={resizeLabel}
      title={resizeLabel}
      on:pointerdown|stopPropagation={(event) => onStartResize(id, event, width, height)}
    ></div>
  {/if}
  </div>
</div>

<style lang="postcss">
  .window-content {
    box-shadow: 0 20px 45px rgb(0 0 0 / 0.28), inset 0 1px rgb(255 255 255 / 0.05);
    transform-origin: top left;
    will-change: transform;
  }

  .focused {
    @apply opacity-100 ring-1 ring-cyan-300/70;
    box-shadow: 0 0 0 1px rgb(34 211 238 / 0.18), 0 22px 54px rgb(0 0 0 / 0.38), 0 0 34px rgb(34 211 238 / 0.1);
  }

  /* Normal drags update coordinates continuously; only workspace-mode changes
     transition the outer position. Ctrl-reorder uses Motion on the inner layer. */
  .interacting { transition-property: opacity, box-shadow; }
  .mode-animating {
    transition: transform 480ms cubic-bezier(.16, 1, .3, 1), opacity 320ms cubic-bezier(.22, 1, .36, 1), box-shadow 320ms cubic-bezier(.22, 1, .36, 1);
  }

  .tiled-window-controls {
    position: absolute;
    z-index: 20;
    left: 50%;
    display: flex;
    flex-direction: column;
    align-items: center;
    transform: translateX(-50%);
  }

  .tiled-drag-handle {
    display: flex;
    gap: 3px;
    align-items: center;
    justify-content: center;
    width: 42px;
    height: 26px;
    padding: 0;
    cursor: grab;
  }

  .tiled-drag-handle:active,
  .tiled-drag-handle.dragging { cursor: grabbing; }
  .tiled-drag-handle:focus-visible { outline: 2px solid rgb(103 232 249 / 0.9); outline-offset: 2px; }

  .tiled-drag-handle span {
    width: 3px;
    height: 3px;
    border-radius: 9999px;
    background: rgb(228 228 231 / 0.9);
    box-shadow: 0 1px rgb(0 0 0 / 0.25);
  }

  @media (pointer: coarse) {
    .tiled-drag-handle {
      width: 50px;
      height: 32px;
    }
  }

  .resize-handle {
    cursor: se-resize;
    cursor: nwse-resize;
  }

  .resize-handle::after {
    content: "";
    @apply absolute bottom-1 right-1 h-2.5 w-2.5 border-b-2 border-r-2 rounded-br-xl border-zinc-500;
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
