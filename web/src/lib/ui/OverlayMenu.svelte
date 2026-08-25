<script lang="ts">
  import { fade, scale } from "svelte/transition";
  import { XIcon } from "svelte-feather-icons";

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return;
    if (event.key === "Escape") {
      event.stopPropagation();
      onClose();
    }
  }

  export let title: string;
  export let description: string;
  export let showCloseButton = false;
  export let maxWidth: number = 768; // screen-md
  export let open: boolean;
  export let onClose: () => void = () => {};
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div
    class="fixed inset-0 z-50 grid place-items-center"
    role="dialog"
    aria-modal="true"
    aria-labelledby="overlay-title"
    aria-describedby="overlay-description"
  >
    <button
      class="fixed inset-0 -z-10 cursor-default bg-black/20 backdrop-blur-sm"
      aria-label="Close dialog"
      type="button"
      on:click={onClose}
      transition:fade={{ duration: 150 }}
    ></button>

    <div
      class="w-full sm:w-[calc(100%-32px)]"
      style="max-width: {maxWidth}px"
      transition:scale={{ duration: 200, start: 0.95 }}
    >
      <div
        class="relative bg-[#111] sm:border border-zinc-800 px-6 py-10 sm:py-6
         h-screen sm:h-auto max-h-screen sm:rounded-lg overflow-y-auto"
      >
        {#if showCloseButton}
          <button
            class="absolute top-4 right-4 p-1 rounded hover:bg-zinc-700 active:bg-indigo-700 transition-colors"
            aria-label="Close {title}"
            on:click={onClose}
            type="button"
          >
            <XIcon class="h-5 w-5" />
          </button>
        {/if}

        <div class="mb-8 text-center">
          <h2 id="overlay-title" class="text-xl font-medium mb-2">
            {title}
          </h2>
          <p id="overlay-description" class="text-zinc-400">
            {description}
          </p>
        </div>

        <slot />
      </div>
    </div>
  </div>
{/if}
