<script lang="ts">
  import {
    FileTextIcon,
    GridIcon,
    LayersIcon,
    PenToolIcon,
    PlusCircleIcon,
    SettingsIcon,
    WifiIcon,
  } from "svelte-feather-icons";

  export let connected: boolean;
  export let hasWriteAccess: boolean | undefined;
  export let drawingMode = false;
  export let workspaceMode: "floating" | "tiled" = "floating";
  export let title = "sshit";
  export let onCreate: () => void = () => {};
  export let onCreateEditor: () => void = () => {};
  export let onToggleDrawing: () => void = () => {};
  export let onToggleWorkspaceMode: () => void = () => {};
  export let onSettings: () => void = () => {};
  export let onNetworkInfo: () => void = () => {};
</script>

<div class="panel toolbar w-full px-3 py-2">
  <div class="flex min-w-0 items-center gap-3 select-none">
    <a
      href="/"
      class="flex min-w-0 shrink items-center gap-2 overflow-hidden text-lg font-semibold tracking-tight"
    >
      <span class="toolbar-title truncate text-sm font-medium"
        >{title}</span
      >
    </a>

    <div class="h-5 border-l-4 border-zinc-800"></div>

    <div
      class="flex min-w-0 flex-1 items-center justify-end gap-1 overflow-x-auto"
    >
      <button
        class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700 disabled:bg-transparent disabled:opacity-50"
        on:click={onCreate}
        disabled={!connected || !hasWriteAccess}
        title={!connected
          ? "Not connected"
          : hasWriteAccess === false
            ? "No write access"
            : "Create new terminal"}
      >
        <PlusCircleIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button
        class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700"
        on:click={onCreateEditor}
        title="Create Markdown editor window"
      >
        <FileTextIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button
        class="icon-button"
        class:active={drawingMode}
        on:click={onToggleDrawing}
        title={drawingMode
          ? "Switch to pointer mode"
          : "Switch to drawing mode"}
      >
        <PenToolIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button
        class="icon-button workspace-mode-button"
        on:click={onToggleWorkspaceMode}
        title={workspaceMode === "tiled"
          ? "Switch to floating mode"
          : "Switch to tiled mode"}
      >
        {#if workspaceMode === "tiled"}
          <GridIcon strokeWidth={1.5} class="p-0.5" />
        {:else}
          <LayersIcon strokeWidth={1.5} class="p-0.5" />
        {/if}
      </button>
      <button
        class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700"
        on:click={onSettings}
      >
        <SettingsIcon strokeWidth={1.5} class="p-0.5" />
      </button>
      <button
        class="relative rounded-md p-1 transition-colors hover:bg-zinc-700 active:bg-indigo-700"
        on:click={onNetworkInfo}
      >
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

  .workspace-mode-button {
    @apply text-zinc-300 ring-0 bg-transparent border-transparent;
  }

  .workspace-mode-button:hover,
  .workspace-mode-button:active {
    @apply bg-zinc-700;
  }

  @media (max-width: 640px) {
    .toolbar-title {
      @apply text-xs;
    }
  }

  @media (pointer: coarse) {
    .toolbar button {
      @apply p-2.5;
    }
  }
</style>
