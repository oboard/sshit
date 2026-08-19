<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from "svelte";
  import { Terminal } from "xterm";
  import { FitAddon } from "xterm-addon-fit";

  import CircleButton from "$lib/ui/CircleButton.svelte";
  import CircleButtons from "$lib/ui/CircleButtons.svelte";
  import { settings } from "$lib/settings";
  import themes from "$lib/ui/themes";

  export type Shell = {
    id: number;
    x: number;
    y: number;
    width: number;
    height: number;
    cols: number;
    rows: number;
    buffer?: string;
  };

  export let shell: Shell;
  export let zIndex = 1;
  export let output = "";

  const dispatch = createEventDispatcher<{
    input: { id: number; data: string };
    resize: { id: number; cols: number; rows: number; width: number; height: number };
    close: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
    focus: { id: number };
  }>();

  let termEl: HTMLDivElement;
  let terminal: Terminal;
  let fitAddon: FitAddon;
  let resizeObserver: ResizeObserver;
  let terminalReady = false;
  let focused = false;
  let currentTitle = "sshit shell";
  let renderedOutputLength = 0;

  $: theme = themes[$settings.theme];
  $: if (terminal) {
    terminal.options.theme = theme;
    terminal.options.scrollback = $settings.scrollback;
  }

  $: if (terminalReady && terminal && output.length !== renderedOutputLength) {
    if (output.length < renderedOutputLength) {
      terminal.reset();
      terminal.write(output);
    } else {
      terminal.write(output.slice(renderedOutputLength));
    }
    renderedOutputLength = output.length;
  }

  function fitAndReport() {
    if (!terminal || !fitAddon || !termEl) return;
    fitAddon.fit();
    dispatch("resize", {
      id: shell.id,
      cols: terminal.cols,
      rows: terminal.rows,
      width: Math.round(shell.width || termEl.offsetWidth),
      height: Math.round(shell.height || termEl.offsetHeight),
    });
  }

  $: if (terminalReady && terminal && fitAddon && termEl && shell.width && shell.height) {
    tick().then(fitAndReport);
  }

  onMount(async () => {
    terminal = new Terminal({
      allowTransparency: false,
      cursorBlink: true,
      cursorStyle: "block",
      fontFamily:
        '"Fira Code VF", "Symbols Nerd Font", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 14,
      lineHeight: 1.06,
      scrollback: $settings.scrollback,
      theme,
    });
    fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(termEl);
    terminalReady = true;
    terminal.onTitleChange((title) => (currentTitle = title || "sshit shell"));
    terminal.onData((data) => dispatch("input", { id: shell.id, data }));
    terminal.onFocus(() => {
      focused = true;
      dispatch("focus", { id: shell.id });
    });
    terminal.onBlur(() => (focused = false));

    await tick();
    fitAndReport();

    resizeObserver = new ResizeObserver(fitAndReport);
    resizeObserver.observe(termEl);
  });

  onDestroy(() => {
    resizeObserver?.disconnect();
    terminal?.dispose();
  });
</script>

<div
  class="term-container absolute select-none"
  class:focused
  style="transform: translate({shell.x}px, {shell.y}px); z-index: {zIndex};"
  data-no-pan
  on:mousedown|stopPropagation={() => dispatch("focus", { id: shell.id })}
  on:pointerdown|stopPropagation
  on:wheel|stopPropagation
>
  <div
    class="flex cursor-move select-none items-center"
    on:mousedown|stopPropagation={(event) => dispatch("startMove", { id: shell.id, event })}
  >
    <div class="flex-1 flex items-center px-3">
      <CircleButtons>
        <CircleButton kind="red" on:mousedown={(event) => event.button === 0 && dispatch("close", { id: shell.id })} />
        <CircleButton kind="yellow" on:mousedown={(event) => event.button === 0 && terminal?.blur()} />
        <CircleButton kind="green" on:mousedown={(event) => event.button === 0 && fitAndReport()} />
      </CircleButtons>
    </div>
    <div class="w-0 flex-grow-[4] overflow-hidden text-ellipsis whitespace-nowrap p-2 text-center text-sm font-medium text-zinc-300">
      {currentTitle}
    </div>
    <div class="flex-1" />
  </div>
  <div
    class="overflow-hidden px-4 py-2"
    bind:this={termEl}
    style="width: {shell.width || 760}px; height: {shell.height || 420}px;"
  />

  <div
    class="resize-handle"
    title="Resize terminal"
    on:mousedown|stopPropagation={(event) =>
      dispatch("startResize", {
        id: shell.id,
        event,
        width: shell.width || termEl?.offsetWidth || 760,
        height: shell.height || termEl?.offsetHeight || 420,
      })}
    on:pointerdown|stopPropagation
  />
</div>

<style lang="postcss">
  .term-container {
    @apply inline-block rounded-lg border border-zinc-700 opacity-90 shadow-2xl;
    background: #181818;
    transition: opacity 200ms;
  }

  .term-container.focused {
    @apply opacity-100 ring-1 ring-indigo-500/50;
  }

  .resize-handle {
    @apply absolute -bottom-1 -right-1 h-5 w-5 cursor-nwse-resize rounded-sm;
  }

  .resize-handle::after {
    content: "";
    @apply absolute bottom-1 right-1 h-2.5 w-2.5 border-b-2 border-r-2 border-zinc-500;
  }
</style>
