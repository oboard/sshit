<script lang="ts" context="module">
  import type { WsUser } from "$lib/protocol";

  /** Convert a string into a unique, hashed hue from 0 to 360. */
  export function nameToHue(name: string): number {
    // Hashes the string with FNV.
    let hash = 2166136261;
    for (let i = 0; i < name.length; i++) {
      hash = (hash ^ name.charCodeAt(i)) * 16777619;
    }
    hash = (hash * 16777619) ^ -1;
    return 360 * (hash / (1 << 31));
  }
</script>

<script lang="ts">
  import { fade } from "svelte/transition";

  export let user: WsUser & { cursorStyle?: string };
  export let showName = false;

  let hovering = false;
  let lastMove = Date.now();

  let lastCursor: [number, number] | null = null;
  let time = Date.now();
  $: if (
    !lastCursor ||
    (user.cursor &&
      (lastCursor[0] !== user.cursor[0] || lastCursor[1] != user.cursor[1]))
  ) {
    lastCursor = user.cursor;
    lastMove = Date.now();
    setTimeout(() => {
      time = Date.now();
    }, 1600);
  }

  /** Determine the SVG cursor shape based on the remote cursorStyle. */
  function cursorShape(style: string | undefined): string {
    if (!style || style === "auto") return "default";
    // Map common cursor CSS values to recognizable glyphs.
    switch (style) {
      case "text":
        return "text";
      case "pointer":
        return "pointer";
      case "crosshair":
        return "crosshair";
      case "move":
        return "move";
      case "grab":
      case "grabbing":
        return "grab";
      case "not-allowed":
        return "not-allowed";
      case "wait":
      case "progress":
        return "wait";
      case "ew-resize":
      case "ns-resize":
      case "nesw-resize":
      case "nwse-resize":
      case "col-resize":
      case "row-resize":
        return "resize";
      case "zoom-in":
      case "zoom-out":
        return "zoom";
      case "cell":
        return "cell";
      case "copy":
        return "copy";
      case "alias":
        return "alias";
      case "context-menu":
        return "context-menu";
      case "vertical-text":
        return "vertical-text";
      case "n-resize":
      case "s-resize":
      case "e-resize":
      case "w-resize":
      case "ne-resize":
      case "nw-resize":
      case "se-resize":
      case "sw-resize":
        return "resize";
      case "all-scroll":
        return "all-scroll";
      case "no-drop":
        return "no-drop";
      case "none":
        return "none";
      case "help":
        return "help";
      default:
        return "default";
    }
  }

  $: shape = cursorShape(user.cursorStyle);
  $: hue = nameToHue(user.name);
  $: fillColor = `hsl(${hue}, 100%, 50%)`;
</script>

<div
  class="flex items-start"
  role="presentation"
  on:mouseenter={() => (hovering = true)}
  on:mouseleave={() => (hovering = false)}
>
  {#if shape === "text"}
    <!-- I-beam cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <rect x="8" y="2" width="2" height="16" rx="1" fill={fillColor} stroke="white" stroke-width="0.8" />
      <rect x="4" y="2" width="10" height="2" rx="1" fill={fillColor} stroke="white" stroke-width="0.8" />
      <rect x="4" y="16" width="10" height="2" rx="1" fill={fillColor} stroke="white" stroke-width="0.8" />
    </svg>
  {:else if shape === "pointer"}
    <!-- Hand pointer cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <path
        d="M10 1C8.9 1 8 1.9 8 3v7H6.5C5.1 10 4 11.1 4 12.5S5.1 15 6.5 15H10v2.5c0 1.4 1.1 2.5 2.5 2.5s2.5-1.1 2.5-2.5V15h1.5c1.4 0 2.5-1.1 2.5-2.5S17.9 10 16.5 10H15V3c0-1.1-.9-2-2-2s-2 .9-2 2z"
        fill={fillColor}
        stroke="white"
        stroke-width="0.8"
        transform="scale(0.85) translate(1.5, 1.5)"
      />
    </svg>
  {:else if shape === "crosshair"}
    <!-- Crosshair cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <circle cx="10" cy="10" r="7" fill="none" stroke={fillColor} stroke-width="1.5" />
      <line x1="10" y1="0" x2="10" y2="6" stroke={fillColor} stroke-width="1.5" />
      <line x1="10" y1="14" x2="10" y2="20" stroke={fillColor} stroke-width="1.5" />
      <line x1="0" y1="10" x2="6" y2="10" stroke={fillColor} stroke-width="1.5" />
      <line x1="14" y1="10" x2="20" y2="10" stroke={fillColor} stroke-width="1.5" />
      <circle cx="10" cy="10" r="1.5" fill={fillColor} />
    </svg>
  {:else if shape === "move"}
    <!-- Move cursor (4 arrows) -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <path d="M10 1L7 4h2v4H5v2h4v4H7l3 3 3-3h-2v-4h4V8h-4V4h2z" fill={fillColor} stroke="white" stroke-width="0.8" />
    </svg>
  {:else if shape === "grab" || shape === "grabbing"}
    <!-- Grabbing hand cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <ellipse cx="10" cy="12" rx="6" ry="5" fill={fillColor} stroke="white" stroke-width="0.8" />
      <rect x="5" y="7" width="3" height="5" rx="1.5" fill={fillColor} stroke="white" stroke-width="0.5" />
      <rect x="9" y="6" width="3" height="6" rx="1.5" fill={fillColor} stroke="white" stroke-width="0.5" />
      <rect x="13" y="7" width="3" height="5" rx="1.5" fill={fillColor} stroke="white" stroke-width="0.5" />
    </svg>
  {:else if shape === "wait"}
    <!-- Wait/progress cursor (hourglass) -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <path d="M4 2h12v3l-5 4 5 4v3H4v-3l5-4-5-4z" fill="none" stroke={fillColor} stroke-width="1.5" />
    </svg>
  {:else if shape === "resize"}
    <!-- Resize cursor (double-headed arrow) -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <path d="M3 10h14M10 3v14" stroke={fillColor} stroke-width="1.5" />
      <path d="M6 6l-3 4 3 4M14 6l3 4-3 4M6 10l-3 4 3 4M14 10l3 4-3 4" stroke={fillColor} stroke-width="1.5" fill="none" transform="rotate(-45 10 10)" />
    </svg>
  {:else if shape === "zoom"}
    <!-- Zoom cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <circle cx="8" cy="8" r="6" fill="none" stroke={fillColor} stroke-width="1.5" />
      <line x1="13" y1="13" x2="19" y2="19" stroke={fillColor} stroke-width="1.5" />
    </svg>
  {:else if shape === "not-allowed" || shape === "no-drop"}
    <!-- Not-allowed cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <circle cx="10" cy="10" r="8" fill="none" stroke={fillColor} stroke-width="1.5" />
      <line x1="4" y1="4" x2="16" y2="16" stroke={fillColor} stroke-width="1.5" />
    </svg>
  {:else if shape === "cell"}
    <!-- Cell cursor (cross) -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <line x1="10" y1="2" x2="10" y2="18" stroke={fillColor} stroke-width="1.5" />
      <line x1="2" y1="10" x2="18" y2="10" stroke={fillColor} stroke-width="1.5" />
    </svg>
  {:else if shape === "copy"}
    <!-- Copy cursor (arrow with plus) -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <path d="M10 18L2 6l10-4z" fill={fillColor} stroke="white" stroke-width="0.8" />
      <line x1="16" y1="14" x2="20" y2="18" stroke={fillColor} stroke-width="1.5" />
      <line x1="20" y1="14" x2="16" y2="18" stroke={fillColor} stroke-width="1.5" />
    </svg>
  {:else if shape === "alias"}
    <!-- Alias cursor (arrow with arrow) -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <path d="M10 18L2 6l10-4z" fill={fillColor} stroke="white" stroke-width="0.8" />
      <path d="M14 14l4 4-4 4z" fill={fillColor} stroke="white" stroke-width="0.8" transform="translate(0, 0)" />
    </svg>
  {:else if shape === "all-scroll"}
    <!-- All-scroll cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <circle cx="10" cy="10" r="6" fill="none" stroke={fillColor} stroke-width="1.5" />
      <path d="M10 4v12M4 10h12" stroke={fillColor} stroke-width="1" />
    </svg>
  {:else if shape === "help"}
    <!-- Help cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <circle cx="10" cy="10" r="8" fill="none" stroke={fillColor} stroke-width="1.5" />
      <text x="10" y="14" text-anchor="middle" fill={fillColor} font-size="12" font-weight="bold">?</text>
    </svg>
  {:else if shape === "vertical-text"}
    <!-- Vertical text cursor -->
    <svg width="20" height="20" viewBox="0 0 20 20">
      <rect x="8" y="2" width="4" height="16" rx="1" fill={fillColor} stroke="white" stroke-width="0.8" />
    </svg>
  {:else}
    <!-- Default arrow cursor -->
    <svg width="23" height="23" viewBox="0 0 23 23">
      <path
        d="M11 22L2 2L22 11L14 14Z"
        fill={fillColor}
        stroke="white"
      />
    </svg>
  {/if}
  {#if showName || hovering || time - lastMove < 1500}
    <p
      class="mt-4 bg-zinc-700 text-xs px-1.5 py-[1px] rounded font-medium"
      transition:fade|local={{ duration: 150 }}
    >
      {user.name}
    </p>
  {/if}
</div>
