<script lang="ts">
  import { createEventDispatcher } from "svelte";

  import CircleButton from "$lib/ui/CircleButton.svelte";
  import CircleButtons from "$lib/ui/CircleButtons.svelte";
  import MarkdownEditor from "$lib/ui/MarkdownEditor.svelte";
  import type { CollabWindowState } from "$lib/yjsStore";

  export let windowState: CollabWindowState;
  export let zIndex = 1;
  export let focused = false;
  export let activeUsers = 1;
  export let synced = false;

  const dispatch = createEventDispatcher<{
    close: { id: number };
    focus: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
  }>();

  $: title = "Markdown Editor";
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="markdown-container absolute select-none"
  class:focused
  style="transform: translate({windowState.x}px, {windowState.y}px); z-index: {zIndex}; background: #111111;"
  data-no-pan
  role="group"
  aria-label={title}
  on:mousedown|stopPropagation={() => dispatch("focus", { id: windowState.id })}
  on:pointerdown|stopPropagation
  on:wheel|stopPropagation
>
  <div
    class="flex cursor-move select-none items-center"
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
    <div class="w-0 flex-grow-[4] overflow-hidden text-ellipsis whitespace-nowrap p-2 text-center text-sm font-medium text-zinc-300">
      {title}
    </div>
    <div class="flex-1"></div>
  </div>

  <div
    class="overflow-hidden px-4 py-2"
    style="width: {windowState.width}px; height: {windowState.height}px;"
  >
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
  .markdown-container {
    @apply inline-block rounded-lg border border-zinc-700 opacity-90 shadow-2xl;
    transition: opacity 200ms;
  }

  .markdown-container.focused {
    @apply opacity-100 ring-1 ring-indigo-500/50;
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
