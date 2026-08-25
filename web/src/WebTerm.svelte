<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
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
  export let tiled = false;
  export let layoutAnimating = false;
  export let tileResizing = false;
  export let tilePaneId: number | null = null;
  export let onInput: (id: number, data: string) => void = () => {};
  export let onResize: (id: number, cols: number, rows: number, width: number, height: number) => void = () => {};
  export let onClose: (id: number) => void = () => {};
  export let onStartMove: (id: number, event: PointerEvent) => void = () => {};
  export let onStartResize: (id: number, event: PointerEvent, width: number, height: number) => void = () => {};
  export let onTiledResize: (id: number, cols: number, rows: number) => void = () => {};
  export let onFocus: (id: number) => void = () => {};
  export let onBlur: (id: number) => void = () => {};
  export let onTitleChange: (id: number, title: string) => void = () => {};

  const isMac = navigator.platform.startsWith("Mac");
  let termEl: HTMLDivElement;
  let terminal: Terminal;
  let fitAddon: FitAddon;
  let terminalReady = false;
  let currentTitle = "sshit shell";
  let renderedOutputLength = 0;
  let disposed = false;
  let terminalError = "";
  let terminalResizeObserver: ResizeObserver | undefined;
  let terminalFitFrame: number | undefined;
  let terminalFitSettleFrame: number | undefined;

  $: theme = themes[$settings.theme];
  $: if (terminal) {
    terminal.options.theme = theme;
    terminal.options.scrollback = $settings.scrollback;
  }

  // Make the derived tiled pane geometry an explicit dependency. Ctrl-drag
  // reordering changes these props even when the terminal DOM observer fires
  // before Motion finishes laying out its new slot.
  $: if (tiled && terminalReady && shell.width >= 0 && shell.height >= 0) {
    scheduleTerminalFit();
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
      onResize(
        shell.id,
        terminal.cols,
        terminal.rows,
        Math.round(shell.width || termEl.offsetWidth),
        Math.round(shell.height || termEl.offsetHeight),
      );
    }
  }

  export function fitSize() {
    if (!terminal || !fitAddon || !termEl) {
      return { cols: shell.cols || 80, rows: shell.rows || 24 };
    }
    fitAddon.fit();
    return { cols: terminal.cols, rows: terminal.rows };
  }

  /** Fit against the floating window's real content box and update its PTY. */
  export function fitAndReportSize() {
    fitAndReport(true);
  }

  function scheduleTerminalFit() {
    if (!tiled || !terminal || !fitAddon || !termEl) return;
    window.cancelAnimationFrame(terminalFitFrame);
    window.cancelAnimationFrame(terminalFitSettleFrame);
    // First frame observes Svelte's updated width/height. A second frame handles
    // the slot change produced by a Ctrl-drag reorder after Motion has committed
    // its FLIP layout, so FitAddon measures the new pane rather than its old one.
    terminalFitFrame = window.requestAnimationFrame(() => {
      fitAddon.fit();
      terminalFitSettleFrame = window.requestAnimationFrame(() => {
        fitAddon.fit();
        if (terminal.cols !== shell.cols || terminal.rows !== shell.rows) {
          onTiledResize(shell.id, terminal.cols, terminal.rows);
        }
      });
    });
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
            onInput(shell.id, "\u0001");
            return false;
          }
          if (event.key === "ArrowRight") {
            onInput(shell.id, "\u0005");
            return false;
          }
          if (event.key === "Backspace") {
            onInput(shell.id, "\u0015");
            return false;
          }
        }
        return true;
      });
      terminal.open(termEl);
      installScaledMouseCoordinateFix();
      terminalResizeObserver = new ResizeObserver(scheduleTerminalFit);
      terminalResizeObserver.observe(termEl);
      terminalReady = true;
      terminal.onTitleChange((title) => {
        currentTitle = title || "sshit shell";
        onTitleChange(shell.id, currentTitle);
      });
      onTitleChange(shell.id, currentTitle);
      terminal.onData((data) => onInput(shell.id, data));
      // xterm v6 no longer exposes onFocus/onBlur. Its input textarea is
      // available after open(), so use native focus events instead.
      terminal.textarea?.addEventListener("focus", () => onFocus(shell.id));
      terminal.textarea?.addEventListener("blur", () => onBlur(shell.id));

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
    terminalResizeObserver?.disconnect();
    window.cancelAnimationFrame(terminalFitFrame);
    window.cancelAnimationFrame(terminalFitSettleFrame);
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
  {tiled}
  {layoutAnimating}
  {tileResizing}
  background={theme.background}
  ariaLabel={currentTitle}
  resizeLabel="Resize terminal"
  tilePaneId={tilePaneId ?? null}
  onClose={(id) => onClose(id)}
  onYellow={() => terminal?.blur()}
  onGreen={() => fitAndReport(true)}
  onFocus={(id) => onFocus(id)}
  onStartMove={(id, event) => onStartMove(id, event)}
  onStartResize={(id, event, width, height) => onStartResize(id, event, width, height)}
>
  <div
    class="overflow-hidden px-4 py-2"
    class:p-0={tiled}
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
