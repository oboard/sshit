<!-- @component Interactive terminal rendered with xterm.js -->
<script lang="ts" context="module">
  import { makeToast } from "$lib/toast";

  // Deduplicated terminal font loading.
  const waitForFonts = (() => {
    let state: "initial" | "loading" | "loaded" = "initial";
    const waitlist: (() => void)[] = [];

    return async function waitForFonts() {
      if (state === "loaded") return;
      else if (state === "initial") {
        const FontFaceObserver = (await import("fontfaceobserver")).default;
        state = "loading";
        try {
          await new FontFaceObserver("Fira Code VF").load();
        } catch (error) {
          makeToast({
            kind: "error",
            message: "Could not load terminal font.",
          });
        }
        state = "loaded";
        for (const fn of waitlist) fn();
      } else {
        await new Promise<void>((resolve) => {
          if (state === "loaded") resolve();
          else waitlist.push(resolve);
        });
      }
    };
  })();
</script>

<script lang="ts">
  import { browser } from "$app/environment";

  import { onDestroy, onMount } from "svelte";
  import type { Terminal } from "@xterm/xterm";
  import { Buffer } from "buffer";

  import themes from "./themes";
  import CircleButton from "./CircleButton.svelte";
  import CircleButtons from "./CircleButtons.svelte";
  import { settings } from "$lib/settings";
  import { TypeAheadAddon } from "$lib/typeahead";

  /** Used to determine Cmd versus Ctrl keyboard shortcuts. */
  const isMac = browser && navigator.platform.startsWith("Mac");

  const typeahead = new TypeAheadAddon();

  export let rows: number, cols: number;
  export let write: (data: string) => void; // bound function prop

  export let termEl: HTMLDivElement = null as any; // suppress "missing prop" warning
  export let onData: (data: Uint8Array) => void = () => {};
  export let onClose: () => void = () => {};
  export let onShrink: () => void = () => {};
  export let onExpand: () => void = () => {};
  export let onBringToFront: () => void = () => {};
  export let onStartMove: (event: MouseEvent) => void = () => {};
  export let onFocus: () => void = () => {};
  export let onBlur: () => void = () => {};
  let term: Terminal | null = null;

  $: theme = themes[$settings.theme];

  $: if (term) {
    // If the theme changes, update existing terminals' appearance.
    term.options.theme = theme;
    term.options.scrollback = $settings.scrollback;
  }

  let loaded = false;
  let focused = false;
  let currentTitle = "Remote Terminal";

  function handleWheelSkipXTerm(event: WheelEvent) {
    event.preventDefault(); // Stop native macOS Chrome zooming on pinch.

    // We stop the event from propagating to the main `.xterm` terminal element,
    // so the xterm.js's event handlers do not fire and scroll the buffer.
    event.stopPropagation();

    // However, we still want it to propagate upward to our pan/zoom handlers,
    // so we re-dispatch the event higher up, skipping xterm.
    termEl?.dispatchEvent(new WheelEvent(event.type, event));
  }

  function setFocused(isFocused: boolean, cursorLayer: HTMLDivElement) {
    if (isFocused && !focused) {
      focused = isFocused;
      cursorLayer.removeEventListener("wheel", handleWheelSkipXTerm);
      onFocus();
    } else if (!isFocused && focused) {
      focused = isFocused;
      cursorLayer.addEventListener("wheel", handleWheelSkipXTerm);
      onBlur();
    }
  }

  const preloadBuffer: string[] = [];

  write = (data: string) => {
    if (!term) {
      // Before the terminal is loaded, push data into a buffer.
      preloadBuffer.push(data);
    } else {
      if (data) data = typeahead.onBeforeProcessData(data);
      term.write(data);
    }
  };

  $: term?.resize(cols, rows);

  onMount(async () => {
    const [{ Terminal }, { WebLinksAddon }, { WebglAddon }, { ImageAddon }] =
      await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-web-links"),
        import("@xterm/addon-webgl"),
        import("@xterm/addon-image"),
      ]);

    await waitForFonts();

    term = new Terminal({
      allowTransparency: false,
      cursorBlink: false,
      cursorStyle: "block",
      // This is the monospace font family configured in Tailwind.
      fontFamily:
        '"Fira Code VF", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 14,
      fontWeight: 400,
      fontWeightBold: 500,
      lineHeight: 1.06,
      scrollback: $settings.scrollback,
      theme,
    });

    // Keyboard shortcuts for natural text editing.
    term.attachCustomKeyEventHandler((event) => {
      if (
        (isMac && event.metaKey && !event.ctrlKey && !event.altKey) ||
        (!isMac && !event.metaKey && event.ctrlKey && !event.altKey)
      ) {
        if (event.key === "ArrowLeft") {
          onData(new Uint8Array([0x01]));
          return false;
        } else if (event.key === "ArrowRight") {
          onData(new Uint8Array([0x05]));
          return false;
        } else if (event.key === "Backspace") {
          onData(new Uint8Array([0x15]));
          return false;
        }
      }
      return true;
    });

    term.loadAddon(new WebLinksAddon());
    term.loadAddon(new WebglAddon());
    term.loadAddon(new ImageAddon({ enableSizeReports: false }));

    term.open(termEl);

    term.resize(cols, rows);
    term.onTitleChange((title) => {
      currentTitle = title;
    });

    // Hack: We artificially disable scrolling when the terminal is not focused.
    // ("termEl" > div.terminal.xterm > div.xterm-screen)
    const screenEl = termEl.querySelector(".xterm-screen")! as HTMLDivElement;
    screenEl.addEventListener("wheel", handleWheelSkipXTerm);

    const focusObserver = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (
          mutation.type === "attributes" &&
          mutation.attributeName === "class"
        ) {
          // The "focus" class is set directly by xterm.js, but there isn't any way to listen for it.
          const target = mutation.target as HTMLElement;
          const isFocused = target.classList.contains("focus");
          setFocused(isFocused, screenEl);
        }
      }
    });
    focusObserver.observe(term.element!, { attributeFilter: ["class"] });

    loaded = true;
    for (const data of preloadBuffer) {
      term.write(data);
    }

    typeahead.reset();
    term.loadAddon(typeahead);

    const utf8 = new TextEncoder();
    term.onData((data: string) => {
      onData(utf8.encode(data));
    });
    term.onBinary((data: string) => {
      onData(Buffer.from(data, "binary"));
    });
  });

  onDestroy(() => term?.dispose());
</script>

<div
  class="inline-block rounded-lg border border-zinc-700 opacity-90 transition-[transform,opacity] duration-200 [&:not(.focused)_.xterm]:cursor-default"
  class:focused
  class:opacity-100={focused}
  style:background={theme.background}
  on:mousedown={() => onBringToFront()}
  on:pointerdown={(event) => event.stopPropagation()}
>
  <div
    class="flex select-none"
    on:mousedown={(event) => onStartMove(event)}
  >
    <div class="flex-1 flex items-center px-3">
      <CircleButtons>
        <!--
          TODO: This should be on:click, but that is not working due to the
          containing element's on:pointerdown `stopPropagation()` call.
        -->
        <CircleButton
          kind="red"
          on:mousedown={(event) => event.button === 0 && onClose()}
        />
        <CircleButton
          kind="yellow"
          on:mousedown={(event) => event.button === 0 && onShrink()}
        />
        <CircleButton
          kind="green"
          on:mousedown={(event) => event.button === 0 && onExpand()}
        />
      </CircleButtons>
    </div>
    <div
      class="p-2 text-sm text-zinc-300 text-center font-medium overflow-hidden whitespace-nowrap text-ellipsis w-0 flex-grow-[4]"
    >
      {currentTitle}
    </div>
    <div class="flex-1" />
  </div>
  <div
    class="inline-block px-4 py-2 transition-opacity duration-500"
    bind:this={termEl}
    style:opacity={loaded ? 1.0 : 0.0}
    on:wheel={(event) => {
      if (focused) {
        // Don't pan the page when scrolling while the terminal is selected.
        // Conversely, we manually disable terminal scrolling unless it is currently selected.
        event.stopPropagation();
      }
    }}
  />
</div>
