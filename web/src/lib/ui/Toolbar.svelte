<script lang="ts">
  import {
    FileTextIcon,
    PenToolIcon,
    PlusCircleIcon,
    SettingsIcon,
    WifiIcon,
  } from "svelte-feather-icons";

  export let connected: boolean;
  export let hasWriteAccess: boolean | undefined;
  export let drawingMode = false;
  export let onCreate: () => void = () => {};
  export let onCreateEditor: () => void = () => {};
  export let onToggleDrawing: () => void = () => {};
  export let onSettings: () => void = () => {};
  export let onNetworkInfo: () => void = () => {};
</script>

<div class="panel toolbar inline-block px-3 py-2">
  <div class="flex items-center select-none">
    <a href="/" class="flex-shrink-0 text-lg font-semibold tracking-tight">
      sshit
    </a>

    <div class="mx-2 h-5 border-l-4 border-zinc-800"></div>

    <div class="flex space-x-1">
      <button
        class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700 disabled:bg-transparent disabled:opacity-50"
        on:click={onCreate}
        disabled={!connected || !hasWriteAccess}
        title={!connected
          ? "Not connected"
          : hasWriteAccess === false // Only show the "No write access" title after confirming read-only mode.
          ? "No write access"
          : "Create new terminal"}
      >
        <PlusCircleIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700" on:click={onCreateEditor} title="Create Markdown editor window">
        <FileTextIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button class="icon-button" class:active={drawingMode} on:click={onToggleDrawing} title={drawingMode ? "Switch to pointer mode" : "Switch to drawing mode"}>
        <PenToolIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700" on:click={onSettings}>
        <SettingsIcon strokeWidth={1.5} class="p-0.5" />
      </button>
    </div>

    <div class="mx-2 h-5 border-l-4 border-zinc-800"></div>

    <div class="flex space-x-1">
      <button class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700" on:click={onNetworkInfo}>
        <WifiIcon strokeWidth={1.5} class="p-0.5" />
      </button>
    </div>
  </div>
</div>

<style lang="postcss">
  .icon-button {
    @apply relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700;
  }

  .icon-button.active {
    @apply bg-indigo-600/30 text-white ring-1 ring-indigo-400/40;
  }

  /* Comfortable 40px+ tap targets on phones and tablets. */
  @media (pointer: coarse) {
    .toolbar button {
      @apply p-2.5;
    }
  }
</style>
