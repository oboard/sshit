<script lang="ts">
  import { onDestroy } from "svelte";
  import { animate } from "motion";

  import CircleButton from "$lib/ui/CircleButton.svelte";
  import CircleButtons from "$lib/ui/CircleButtons.svelte";

  type Props = {
    id: number;
    title: string;
    x: number;
    y: number;
    width: number;
    height: number;
    zIndex?: number;
    focused?: boolean;
    background?: string;
    ariaLabel?: string;
    resizeLabel?: string;
    tiled?: boolean;
    layoutAnimating?: boolean;
    tileResizing?: boolean;
    tilePaneId?: number | null;
    onClose?: (id: number) => void;
    onYellow?: (id: number) => void;
    onGreen?: (id: number) => void;
    onFocus?: (id: number) => void;
    onStartMove?: (id: number, event: PointerEvent) => void;
    onStartTiledMove?: (id: number, event: PointerEvent) => void;
    onMoveTiledMove?: (event: PointerEvent) => void;
    onFinishTiledMove?: () => void;
    onStartResize?: (id: number, event: PointerEvent, width: number, height: number) => void;
    onTitlebarDoubleClick?: (id: number) => void;
  };

  let {
    id,
    title,
    x,
    y,
    width,
    height,
    zIndex = 1,
    focused = false,
    background = "#111111",
    ariaLabel = title,
    resizeLabel = `Resize ${title}`,
    tiled = false,
    layoutAnimating = false,
    tileResizing = false,
    tilePaneId = null,
    onClose = () => {},
    onYellow = () => {},
    onGreen = () => {},
    onFocus = () => {},
    onStartMove = () => {},
    onStartTiledMove = () => {},
    onMoveTiledMove = () => {},
    onFinishTiledMove = () => {},
    onStartResize = () => {},
    onTitlebarDoubleClick = () => {},
  }: Props = $props();

  let frameEl: HTMLDivElement;
  let tiledHandleEl: HTMLButtonElement;
  let tiledDragTarget: HTMLElement | null = null;
  let tiledDecorationsOpen = $state(false);
  let tiledHandleStart = $state<{ x: number; y: number } | null>(null);
  let tiledHandleDragged = $state(false);

  // Decorations are explicitly opened from this pane's ellipsis, not derived
  // from whichever tiled pane happens to own workspace focus.
  $effect(() => {
    if (!tiled || !focused) tiledDecorationsOpen = false;
  });

  function startTiledHandle(event: PointerEvent) {
    if (event.button !== 0) return;
    event.preventDefault();
    onFocus(id);
    tiledHandleStart = { x: event.clientX, y: event.clientY };
    tiledHandleDragged = false;
    tiledDragTarget = event.currentTarget as HTMLElement;
    tiledDragTarget.setPointerCapture(event.pointerId);
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
    tiledDragTarget?.releasePointerCapture(event.pointerId);
    if (!tiledHandleDragged) tiledDecorationsOpen = true;
    onFinishTiledMove();
    tiledDragTarget = null;
    tiledHandleStart = null;
  }

  // Tiled titlebar events stay inside the pane because they own reordering.
  // Floating titlebar moves must bubble to the workspace's drag loop.
  function moveTitlebar(event: PointerEvent) {
    if (!tiled) return;
    event.stopPropagation();
    moveTiledHandle(event);
  }

  function finishTitlebar(event: PointerEvent) {
    if (!tiled) return;
    event.stopPropagation();
    finishTiledHandle(event);
  }

  let contentEl: HTMLDivElement;
  let beforeRect = $state<DOMRect | null>(null);
  let flipAnimation: { stop: () => void } | undefined;

  // The dragged frame follows the cursor directly. All other tiled frames use
  // Motion's FLIP transform so they glide into the slots they are reordered to.
  const isDraggedTile = $derived(tiled && zIndex >= 1000);

  // `$effect.pre` preserves the previous layout box before Svelte applies DOM
  // updates; `$effect` below then performs the corresponding FLIP animation.
  $effect.pre(() => {
    if (tiled && !layoutAnimating && !tileResizing && !isDraggedTile && frameEl) {
      beforeRect = frameEl.getBoundingClientRect();
    } else {
      beforeRect = null;
    }
  });

  $effect(() => {
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
  {#if tiled && !tiledDecorationsOpen}
    <div class="tiled-window-controls">
      <button
        bind:this={tiledHandleEl}
        class="tiled-drag-handle"
        class:dragging={tiledHandleStart !== null}
        type="button"
        aria-label={`Move or show controls for ${title}`}
        aria-expanded={tiledDecorationsOpen}
        title="Drag to rearrange; click to show window controls"
        style="touch-action: none;"
        on:pointerdown|stopPropagation={startTiledHandle}
        on:pointermove|stopPropagation={moveTiledHandle}
        on:pointerup|stopPropagation={finishTiledHandle}
        on:pointercancel|stopPropagation={finishTiledHandle}
      ><span></span><span></span><span></span></button>
    </div>
  {:else}
    <!-- Floating windows and explicitly opened tiled decorations share this titlebar. -->
    <div
      class="window-titlebar rounded-t-xl flex cursor-move select-none items-center overflow-hidden"
      class:tiled-titlebar-overlay={tiled}
      class:floating-titlebar={!tiled}
      style="touch-action: none;"
      role="toolbar"
      tabindex="-1"
      aria-label="{title} window controls"
      on:pointerdown|stopPropagation={(event) => tiled ? startTiledHandle(event) : onStartMove(id, event)}
      on:pointermove={moveTitlebar}
      on:pointerup={finishTitlebar}
      on:pointercancel={finishTitlebar}
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

  /* Keep floating titlebars composited while their coordinate transform updates
     every pointer frame; the tiled overlay deliberately remains in its pane. */
  .floating-titlebar { will-change: transform; }

  .tiled-titlebar-overlay {
    position: absolute;
    z-index: 20;
    top: 0;
    right: 0;
    left: 0;
    box-shadow: 0 8px 18px rgb(0 0 0 / 0.25), inset 0 -1px rgb(255 255 255 / 0.06);
    backdrop-filter: blur(12px);
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
