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
  export let onClose: (id: number) => void = () => {};
  export let onFocus: (id: number) => void = () => {};
  export let onStartMove: (id: number, event: PointerEvent) => void = () => {};
  export let onStartResize: (id: number, event: PointerEvent, width: number, height: number) => void = () => {};

  const title = "Markdown Editor";
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
  background="#111111"
  ariaLabel={title}
  resizeLabel="Resize {title}"
  onClose={(id) => onClose(id)}
  onYellow={(id) => onFocus(id)}
  onGreen={(id) => onFocus(id)}
  onFocus={(id) => onFocus(id)}
  onStartMove={(id, _event) => onStartMove(id, _event)}
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
