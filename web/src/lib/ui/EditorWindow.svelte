<script lang="ts">
  import { createEventDispatcher } from "svelte";

  import MarkdownEditor from "$lib/ui/MarkdownEditor.svelte";
  import WindowFrame from "$lib/ui/WindowFrame.svelte";
  import type { EditorWindowState } from "$lib/editorWindows";

  export let windowState: EditorWindowState;
  export let zIndex = 1;
  export let focused = false;

  const dispatch = createEventDispatcher<{
    close: { id: number };
    focus: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
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
    <MarkdownEditor docId={windowState.docId} />
  </div>
</WindowFrame>
