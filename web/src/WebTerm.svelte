<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from "svelte";
  import type { FitAddon } from "@xterm/addon-fit";
  import type { Terminal } from "@xterm/xterm";

  import WindowFrame from "$lib/ui/WindowFrame.svelte";
  import { settings } from "$lib/settings";
  import themes from "$lib/ui/themes";
  import type { WindowState } from "$lib/protocol";

  export type Shell = WindowState;

  export let shell: Shell;
  export let zIndex = 1;
  export let output = "";
  export let focused = false;

  const dispatch = createEventDispatcher<{
    input: { id: number; data: string };
    resize: { id: number; cols: number; rows: number; width: number; height: number };
    close: { id: number };
    startMove: { id: number; event: MouseEvent };
    startResize: { id: number; event: MouseEvent; width: number; height: number };
    focus: { id: number };
    blur: { id: number };
  }>();

  const isMac = navigator.platform.startsWith("Mac");
  let termEl: HTMLDivElement;
  let terminal: Terminal;
  let fitAddon: FitAddon;
  let terminalReady = false;
  let currentTitle = "sshit shell";
  let renderedOutputLength = 0;
  let disposed = false;
  let terminalError = "";

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

  function fitAndReport(report = false) {
    if (!terminal || !fitAddon || !termEl) return;
    fitAddon.fit();
    if (report && (terminal.cols !== shell.cols || terminal.rows !== shell.rows)) {
      dispatch("resize", {
        id: shell.id,
        cols: terminal.cols,
        rows: terminal.rows,
        width: Math.round(shell.width || termEl.offsetWidth),
        height: Math.round(shell.height || termEl.offsetHeight),
      });
    }
  }

  export function fitSize() {
    if (!terminal || !fitAddon || !termEl) {
      return { cols: shell.cols || 80, rows: shell.rows || 24 };
    }
    fitAddon.fit();
    return { cols: terminal.cols, rows: terminal.rows };
  }

  async function waitForTerminalFont() {
    const FontFaceObserver = (await import("fontfaceobserver")).default;
    await new FontFaceObserver("Fira Code VF").load();
  }

  /**
   * The workspace zooms terminal windows with CSS transforms. xterm's mouse
   * service receives transformed client coordinates but divides them by its
   * unscaled cell size, so normalize pointer positions before it resolves a
   * selection or forwards a terminal mouse report.
   */
  function installScaledMouseCoordinateFix() {
    const mouseService = (terminal as any)._core?._mouseService;
    if (!mouseService) return;

    const toTerminalCoordinates = (event: MouseEvent | WheelEvent, element: HTMLElement) => {
      const rect = element.getBoundingClientRect();
      const width = element.offsetWidth;
      const height = element.offsetHeight;
      const scaleX = rect.width / width;
      const scaleY = rect.height / height;
      if (!width || !height || !Number.isFinite(scaleX) || !Number.isFinite(scaleY) || scaleX <= 0 || scaleY <= 0) {
        return event;
      }

      // Keep every event property through its prototype while overriding only
      // the coordinates used by xterm's MouseService.
      return Object.create(event, {
        clientX: { value: rect.left + (event.clientX - rect.left) / scaleX },
        clientY: { value: rect.top + (event.clientY - rect.top) / scaleY },
      });
    };

    const getCoords = mouseService.getCoords.bind(mouseService);
    mouseService.getCoords = (event: MouseEvent, element: HTMLElement, ...args: any[]) =>
      getCoords(toTerminalCoordinates(event, element), element, ...args);

    const getMouseReportCoords = mouseService.getMouseReportCoords.bind(mouseService);
    mouseService.getMouseReportCoords = (event: MouseEvent, element: HTMLElement) =>
      getMouseReportCoords(toTerminalCoordinates(event, element), element);
  }

  async function initializeTerminal() {
    terminalError = "";
    try {
      // Only xterm and fitting are required to show a usable terminal. Optional
      // renderers and enhancements must never turn a transient chunk failure
      // into a blank terminal.
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
      ]);
      const enhancements = await Promise.allSettled([
        import("@xterm/addon-web-links"),
        import("@xterm/addon-webgl"),
        import("@xterm/addon-image"),
        import("$lib/typeahead"),
      ]);
      await waitForTerminalFont().catch(() => undefined);
      if (disposed || !termEl) return;

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

      const [webLinks, webgl, image, typeahead] = enhancements;
      if (webLinks.status === "fulfilled") {
        terminal.loadAddon(new webLinks.value.WebLinksAddon());
      }
      if (image.status === "fulfilled") {
        terminal.loadAddon(new image.value.ImageAddon({ enableSizeReports: false }));
      }
      if (typeahead.status === "fulfilled") {
        const addon = new typeahead.value.TypeAheadAddon();
        terminal.loadAddon(addon);
        addon.reset();
      }
      if (webgl.status === "fulfilled") {
        try {
          terminal.loadAddon(new webgl.value.WebglAddon());
        } catch (error) {
          console.warn("WebGL terminal renderer unavailable; using DOM renderer", error);
        }
      }
      terminal.attachCustomKeyEventHandler((event) => {
        if (
          (isMac && event.metaKey && !event.ctrlKey && !event.altKey) ||
          (!isMac && !event.metaKey && event.ctrlKey && !event.altKey)
        ) {
          if (event.key === "ArrowLeft") {
            dispatch("input", { id: shell.id, data: "\u0001" });
            return false;
          }
          if (event.key === "ArrowRight") {
            dispatch("input", { id: shell.id, data: "\u0005" });
            return false;
          }
          if (event.key === "Backspace") {
            dispatch("input", { id: shell.id, data: "\u0015" });
            return false;
          }
        }
        return true;
      });
      terminal.open(termEl);
      installScaledMouseCoordinateFix();
      terminalReady = true;
      terminal.onTitleChange((title) => (currentTitle = title || "sshit shell"));
      terminal.onData((data) => dispatch("input", { id: shell.id, data }));
      // xterm v6 no longer exposes onFocus/onBlur. Its input textarea is
      // available after open(), so use native focus events instead.
      terminal.textarea?.addEventListener("focus", () => dispatch("focus", { id: shell.id }));
      terminal.textarea?.addEventListener("blur", () => dispatch("blur", { id: shell.id }));

      await tick();
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      if (!disposed) fitAndReport(true);
    } catch (error) {
      if (!disposed) {
        terminalError = "Unable to load terminal features. Check your connection and try again.";
        console.error("terminal initialization failed", error);
      }
    }
  }

  onMount(() => {
    void initializeTerminal();
  });

  onDestroy(() => {
    disposed = true;
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
  >
    {#if terminalError}
      <div class="grid h-full place-items-center gap-3 text-sm text-zinc-300">
        <span>{terminalError}</span>
        <button class="rounded bg-zinc-700 px-3 py-1.5 hover:bg-zinc-600" on:click={initializeTerminal}>Retry</button>
      </div>
    {/if}
  </div>
</WindowFrame>
