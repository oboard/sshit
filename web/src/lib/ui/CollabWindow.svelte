<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { FileTextIcon } from "svelte-feather-icons";

  import CircleButton from "$lib/ui/CircleButton.svelte";
  import CircleButtons from "$lib/ui/CircleButtons.svelte";
  import MarkdownEditor from "$lib/ui/MarkdownEditor.svelte";
  import type { CollabWindowState } from "$lib/yjsStore";

  export let windowState: CollabWindowState;
  export let zIndex = 1;
  export let activeUsers = 1;
  export let synced = false;

  const dispatch = createEventDispatcher<{
    close: { id: number };
    focus: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
  }>();

  $: title = "Markdown Editor";
  $: subtitle = synced ? `Yjs synced · ${windowState.docId}` : `local · ${windowState.docId}`;
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="collab-window absolute select-none"
  class:editor={true}
  style="transform: translate({windowState.x}px, {windowState.y}px); z-index: {zIndex}; width: {windowState.width}px; height: {windowState.height}px;"
  data-no-pan
  role="group"
  aria-label={title}
  on:mousedown|stopPropagation={() => dispatch("focus", { id: windowState.id })}
  on:pointerdown|stopPropagation
  on:wheel|stopPropagation
>
  <div
    class="window-titlebar"
    role="toolbar"
    tabindex="-1"
    aria-label="{title} window controls"
    on:mousedown|stopPropagation={(event) => dispatch("startMove", { id: windowState.id, event })}
  >
    <div class="flex flex-1 items-center px-3">
      <CircleButtons>
        <CircleButton kind="red" on:mousedown={(event) => event.button === 0 && dispatch("close", { id: windowState.id })} />
        <CircleButton kind="yellow" on:mousedown={(event) => event.button === 0 && dispatch("focus", { id: windowState.id })} />
        <CircleButton kind="green" on:mousedown={(event) => event.button === 0 && dispatch("focus", { id: windowState.id })} />
      </CircleButtons>
    </div>

    <div class="title-center">
      <FileTextIcon strokeWidth={1.5} class="h-4 w-4 text-pink-200" />
      <span>{title}</span>
      <em>{subtitle}</em>
    </div>

    <div class="flex flex-1 justify-end px-3 text-xs text-zinc-500">
      {windowState.width}×{windowState.height}
    </div>
  </div>

  <div class="window-body" style="height: {Math.max(260, windowState.height - 42)}px;">
    <MarkdownEditor docId={windowState.docId} {activeUsers} {synced} compact />
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="resize-handle"
    role="separator"
    aria-label="Resize {title}"
    title="Resize {title}"
    on:mousedown|stopPropagation={(event) =>
      dispatch("startResize", {
        id: windowState.id,
        event,
        width: windowState.width,
        height: windowState.height,
      })}
    on:pointerdown|stopPropagation
  ></div>
</div>

<style lang="postcss">
  .collab-window {
    @apply overflow-hidden rounded-lg border border-zinc-700 bg-zinc-950/95 opacity-95 shadow-2xl backdrop-blur-sm;
    transition: opacity 160ms, box-shadow 160ms;
  }

  .collab-window:hover {
    @apply opacity-100 ring-1 ring-indigo-500/30;
  }

  .collab-window.editor {
    @apply shadow-pink-950/30;
  }

  .window-titlebar {
    @apply flex h-[42px] cursor-move select-none items-center border-b border-zinc-800 bg-zinc-900/95;
  }

  .title-center {
    @apply flex min-w-0 flex-grow-[4] items-center justify-center gap-2 overflow-hidden text-ellipsis whitespace-nowrap p-2 text-center text-sm font-medium text-zinc-200;
  }

  .title-center em {
    @apply ml-1 text-xs not-italic text-zinc-500;
  }

  .window-body {
    @apply min-h-0 overflow-hidden;
  }

  .resize-handle {
    @apply absolute -bottom-1 -right-1 h-5 w-5 rounded-sm;
    cursor: se-resize;
    cursor: nwse-resize;
  }

  .resize-handle::after {
    content: "";
    @apply absolute bottom-1 right-1 h-2.5 w-2.5 border-b-2 border-r-2 border-zinc-500;
    pointer-events: none;
  }
</style>
