<script lang="ts">
  import { createEventDispatcher } from "svelte";

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

  const dispatch = createEventDispatcher<{
    close: { id: number };
    yellow: { id: number };
    green: { id: number };
    focus: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
  }>();
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="window-frame absolute select-none"
  class:focused
  style="transform: translate({x}px, {y}px); z-index: {zIndex}; background: {background};"
  data-no-pan
  role="group"
  aria-label={ariaLabel}
  on:mousedown|stopPropagation={() => dispatch("focus", { id })}
  on:pointerdown|stopPropagation
  on:wheel|stopPropagation
>
  <div
    class="flex cursor-move select-none items-center"
    role="toolbar"
    tabindex="-1"
    aria-label="{title} window controls"
    on:mousedown|stopPropagation={(event) => dispatch("startMove", { id, event })}
  >
    <div class="flex flex-1 items-center px-3">
      <CircleButtons>
        <CircleButton kind="red" on:mousedown={(event) => event.button === 0 && dispatch("close", { id })} />
        <CircleButton kind="yellow" on:mousedown={(event) => event.button === 0 && dispatch("yellow", { id })} />
        <CircleButton kind="green" on:mousedown={(event) => event.button === 0 && dispatch("green", { id })} />
      </CircleButtons>
    </div>
    <div class="w-0 flex-grow-[4] overflow-hidden text-ellipsis whitespace-nowrap p-2 text-center text-sm font-medium text-zinc-300">
      {title}
    </div>
    <div class="flex-1"></div>
  </div>

  <slot />

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="resize-handle"
    role="separator"
    aria-label={resizeLabel}
    title={resizeLabel}
    on:mousedown|stopPropagation={(event) => dispatch("startResize", { id, event, width, height })}
    on:pointerdown|stopPropagation
  ></div>
</div>

<style lang="postcss">
  .window-frame {
    @apply inline-block rounded-lg border border-zinc-700 opacity-90 shadow-2xl;
    transition: opacity 200ms;
  }

  .window-frame.focused {
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
