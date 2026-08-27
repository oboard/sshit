<script lang="ts">
  import { onDestroy, type Snippet } from "svelte";
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
    children: Snippet;
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
    children,
  }: Props = $props();

  let frameEl: HTMLDivElement;
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

  function stopPropagation(handler: (event: PointerEvent) => void) {
    return (event: PointerEvent) => {
      event.stopPropagation();
      handler(event);
    };
  }

  function stopPointerDown(event: PointerEvent) {
    event.stopPropagation();
    onFocus(id);
  }

  function startTitlebarInteraction(event: PointerEvent) {
    event.stopPropagation();
    if (tiled) startTiledHandle(event);
    else onStartMove(id, event);
  }

  function finishTitlebar(event: PointerEvent) {
    if (!tiled) return;
    event.stopPropagation();
    finishTiledHandle(event);
  }

  function resize(event: PointerEvent) {
    event.stopPropagation();
    onStartResize(id, event, width, height);
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
    // Track pane geometry explicitly: unlike legacy beforeUpdate, rune effects
    // only rerun for values they read.
    void x;
    void y;
    void width;
    void height;
    // A swap promotes the dragged pane to z-index 1000. Its siblings still need
    // their prior boxes captured so their FLIP transition runs while it moves.
    if (tiled && !layoutAnimating && !tileResizing && frameEl) {
      beforeRect = frameEl.getBoundingClientRect();
    } else {
      beforeRect = null;
    }
  });

  $effect(() => {
    // Mode changes already animate the outer coordinate transform. Clearing the
    // FLIP baseline here prevents an extra scale animation when that transition
    // finishes and `layoutAnimating` becomes false.
    if (!tiled || layoutAnimating || tileResizing || !frameEl || !contentEl) {
      beforeRect = null;
      return;
    }
    const before = beforeRect;
    // The pane under the pointer already follows its ghost coordinates directly.
    // Do not FLIP it, but let every other tile animate into its swapped slot.
    if (isDraggedTile) {
      beforeRect = null;
      return;
    }
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
  class="absolute inline-block select-none {layoutAnimating ? '[transition:transform_480ms_cubic-bezier(.16,1,.3,1),opacity_320ms_cubic-bezier(.22,1,.36,1),box-shadow_320ms_cubic-bezier(.22,1,.36,1)]' : ''}"
  style="transform: translate({x}px, {y}px); z-index: {zIndex};"
  data-no-pan
  data-tile-pane={tilePaneId ?? undefined}
  role="group"
  aria-label={ariaLabel}
  onpointerdown={stopPointerDown}
  onwheel={(event) => event.stopPropagation()}
>
  <div
    bind:this={contentEl}
    class="rounded-xl border border-white/10 opacity-90 shadow-[0_20px_45px_rgb(0_0_0/0.28),inset_0_1px_rgb(255_255_255/0.05)] transition-[opacity,box-shadow] duration-200 origin-top-left will-change-transform {focused ? 'opacity-100 ring-1 ring-cyan-300/70 shadow-[0_0_0_1px_rgb(34_211_238/0.18),0_22px_54px_rgb(0_0_0/0.38),0_0_34px_rgb(34_211_238/0.1)]' : ''}"
    style="background: {background};"
  >
  {#if tiled && !tiledDecorationsOpen}
    <div class="absolute left-1/2 z-20 flex -translate-x-1/2 flex-col items-center">
      <button
        class="flex h-[26px] w-[42px] cursor-grab items-center justify-center gap-[3px] p-0 active:cursor-grabbing focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300/90 [@media(pointer:coarse)]:h-8 [@media(pointer:coarse)]:w-[50px] {tiledHandleStart !== null ? 'cursor-grabbing' : ''}"
        type="button"
        aria-label={`Move or show controls for ${title}`}
        aria-expanded={tiledDecorationsOpen}
        title="Drag to rearrange; click to show window controls"
        style="touch-action: none;"
        onpointerdown={stopPropagation(startTiledHandle)}
        onpointermove={stopPropagation(moveTiledHandle)}
        onpointerup={stopPropagation(finishTiledHandle)}
        onpointercancel={stopPropagation(finishTiledHandle)}
      ><span class="h-[3px] w-[3px] rounded-full bg-zinc-200/90 shadow-[0_1px_rgb(0_0_0/0.25)]"></span><span class="h-[3px] w-[3px] rounded-full bg-zinc-200/90 shadow-[0_1px_rgb(0_0_0/0.25)]"></span><span class="h-[3px] w-[3px] rounded-full bg-zinc-200/90 shadow-[0_1px_rgb(0_0_0/0.25)]"></span></button>
    </div>
  {:else}
    <!-- Floating windows and explicitly opened tiled decorations share this titlebar. -->
    <div
      class="flex cursor-move select-none items-center overflow-hidden rounded-t-xl {tiled ? 'absolute inset-x-0 top-0 z-20 shadow-[0_8px_18px_rgb(0_0_0/0.25),inset_0_-1px_rgb(255_255_255/0.06)] backdrop-blur-xl' : 'will-change-transform'}"
      style="touch-action: none;"
      role="toolbar"
      tabindex="-1"
      aria-label="{title} window controls"
      onpointerdown={startTitlebarInteraction}
      onpointermove={moveTitlebar}
      onpointerup={finishTitlebar}
      onpointercancel={finishTitlebar}
      ondblclick={(event) => {
        event.stopPropagation();
        if (!tiled) onTitlebarDoubleClick(id);
      }}
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

  {@render children()}

  {#if !tiled}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="absolute -bottom-1 -right-1 h-5 w-5 cursor-nwse-resize rounded-sm after:absolute after:bottom-1 after:right-1 after:h-2.5 after:w-2.5 after:rounded-br-xl after:border-b-2 after:border-r-2 after:border-zinc-500 after:content-[''] [@media(pointer:coarse)]:-bottom-2 [@media(pointer:coarse)]:-right-2 [@media(pointer:coarse)]:h-9 [@media(pointer:coarse)]:w-9 [@media(pointer:coarse)]:after:bottom-2 [@media(pointer:coarse)]:after:right-2 [@media(pointer:coarse)]:after:h-3 [@media(pointer:coarse)]:after:w-3 [@media(pointer:coarse)]:after:border-zinc-400"
      style="touch-action: none;"
      role="separator"
      aria-label={resizeLabel}
      title={resizeLabel}
      onpointerdown={resize}
    ></div>
  {/if}
  </div>
</div>
