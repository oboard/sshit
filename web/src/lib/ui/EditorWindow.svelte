<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";

  let MarkdownEditor: typeof import("$lib/ui/MarkdownEditor.svelte").default | null = null;

  onMount(async () => {
    MarkdownEditor = (await import("$lib/ui/MarkdownEditor.svelte")).default;
  });
  import WindowFrame from "$lib/ui/WindowFrame.svelte";
  import type { WindowState } from "$lib/protocol";

  export let windowState: WindowState;
  export let zIndex = 1;
  export let focused = false;

  const dispatch = createEventDispatcher<{
    close: { id: number };
    focus: { id: number };
    startMove: { id: number; event: PointerEvent };
    startResize: { id: number; event: PointerEvent; width: number; height: number };
  }>();

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
  background="#111111"
  ariaLabel={title}
  resizeLabel="Resize {title}"
  on:close={(event) => dispatch("close", event.detail)}
  on:yellow={(event) => dispatch("focus", event.detail)}
  on:green={(event) => dispatch("focus", event.detail)}
  on:focus={(event) => dispatch("focus", event.detail)}
  on:startMove={(event) => dispatch("startMove", event.detail)}
  on:startResize={(event) => dispatch("startResize", event.detail)}
>
  <div class="overflow-hidden" style="width: {windowState.width}px; height: {windowState.height}px;">
    {#if MarkdownEditor}
      <MarkdownEditor docId={windowState.docId ?? `doc-${windowState.id}`} />
    {:else}
      <div class="grid h-full place-items-center text-sm text-zinc-400">Loading editor…</div>
    {/if}
  </div>
</WindowFrame>
