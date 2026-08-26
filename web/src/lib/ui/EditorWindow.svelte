<script lang="ts">
  import { onMount } from "svelte";

  let MarkdownEditor: typeof import("$lib/ui/MarkdownEditor.svelte").default | null = null;

  onMount(async () => {
    MarkdownEditor = (await import("$lib/ui/MarkdownEditor.svelte")).default;
  });
  import WindowFrame from "$lib/ui/WindowFrame.svelte";
  import type { WindowState } from "$lib/protocol";

  export let windowState: WindowState;
  export let zIndex = 1;
  export let focused = false;
  export let tiled = false;
  export let layoutAnimating = false;
  export let tileResizing = false;
  export let tilePaneId: number | null = null;
  export let onClose: (id: number) => void = () => {};
  export let onFocus: (id: number) => void = () => {};
  export let onStartMove: (id: number, event: PointerEvent) => void = () => {};
  export let onStartTiledMove: (id: number, event: PointerEvent) => void = () => {};
  export let onMoveTiledMove: (event: PointerEvent) => void = () => {};
  export let onFinishTiledMove: () => void = () => {};
  export let onStartResize: (id: number, event: PointerEvent, width: number, height: number) => void = () => {};
  export let onTitlebarDoubleClick: (id: number) => void = () => {};

  $: title = windowState.title || "Markdown Editor";
</script>

<WindowFrame
  id={windowState.id}
  {title}
  x={windowState.x}
  y={windowState.y}
  width={windowState.width}
  height={windowState.height}
  {zIndex}
  {focused}
  {tiled}
  {layoutAnimating}
  {tileResizing}
  background="#111111"
  ariaLabel={title}
  resizeLabel="Resize {title}"
  tilePaneId={tilePaneId ?? null}
  onClose={(id) => onClose(id)}
  onYellow={(id) => onFocus(id)}
  onGreen={(id) => onFocus(id)}
  onFocus={(id) => onFocus(id)}
  onTitlebarDoubleClick={(id) => onTitlebarDoubleClick(id)}
  onStartMove={(id, event) => onStartMove(id, event)}
  onStartTiledMove={(id, event) => onStartTiledMove(id, event)}
  {onMoveTiledMove}
  {onFinishTiledMove}
  onStartResize={(id, event, width, height) => onStartResize(id, event, width, height)}
>
  <div class="overflow-hidden" style="width: {windowState.width}px; height: {windowState.height}px;">
    {#if MarkdownEditor}
      <MarkdownEditor docId={windowState.docId ?? `doc-${windowState.id}`} />
    {:else}
      <div class="grid h-full place-items-center text-sm text-zinc-400">Loading editor…</div>
    {/if}
  </div>
</WindowFrame>
