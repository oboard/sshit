<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";

  import WindowFrame from "$lib/ui/WindowFrame.svelte";
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
  export let focused = false;

  const dispatch = createEventDispatcher<{
    input: { id: number; data: string };
    close: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
    focus: { id: number };
    blur: { id: number };
  }>();

  let termEl: HTMLDivElement;
  let terminal: Terminal;
  let fitAddon: FitAddon;
  let terminalReady = false;
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
  }

  export function fitSize() {
    if (!terminal || !fitAddon || !termEl) {
      return { cols: shell.cols || 80, rows: shell.rows || 24 };
    }
    fitAddon.fit();
    return { cols: terminal.cols, rows: terminal.rows };
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
      dispatch("focus", { id: shell.id });
    });
    terminal.onBlur(() => {
      dispatch("blur", { id: shell.id });
    });

    await tick();
    fitAndReport();
  });

  onDestroy(() => {
    terminal?.dispose();
  });
</script>

<WindowFrame
  id={shell.id}
  title={currentTitle}
  x={shell.x}
  y={shell.y}
  width={shell.width || termEl?.offsetWidth || 760}
  height={shell.height || termEl?.offsetHeight || 420}
  {zIndex}
  {focused}
  background={theme.background}
  ariaLabel={currentTitle}
  resizeLabel="Resize terminal"
  on:close={(event) => dispatch("close", event.detail)}
  on:yellow={() => terminal?.blur()}
  on:green={fitAndReport}
  on:focus={(event) => dispatch("focus", event.detail)}
  on:startMove={(event) => dispatch("startMove", event.detail)}
  on:startResize={(event) => dispatch("startResize", event.detail)}
>
  <div
    class="overflow-hidden px-4 py-2"
    bind:this={termEl}
    style="width: {shell.width || 760}px; height: {shell.height || 420}px;"
  ></div>
</WindowFrame>
