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
  import { cursorAssetName, cursorUrl } from "$lib/cursors";

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

  $: src = cursorUrl(cursorAssetName(user.cursorStyle));
</script>

{#if showName || hovering || time - lastMove < 1500}
  <div
    class="relative h-8 w-8"
    role="presentation"
    on:mouseenter={() => (hovering = true)}
    on:mouseleave={() => (hovering = false)}
  >
    <img {src} alt="" draggable="false" class="h-8 w-8 select-none" />
    <p
      class="absolute left-0 top-8 mt-4 w-max bg-zinc-700 text-xs px-1.5 py-[1px] rounded font-medium"
      transition:fade|local={{ duration: 150 }}
    >
      {user.name}
    </p>
  </div>
{/if}
