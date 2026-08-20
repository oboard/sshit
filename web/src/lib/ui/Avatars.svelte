<script lang="ts">
  import { fade } from "svelte/transition";

  import type { WsUser } from "$lib/protocol";
  import { nameToHue } from "./LiveCursor.svelte";

  export let users: [number, WsUser][];

  function nameToInitials(name: string): string {
    const parts = name.split(/\s/).filter((s) => s);
    if (parts.length === 0) {
      return "-";
    } else if (parts.length === 1) {
      return parts[0][0].toLocaleUpperCase();
    } else {
      return (parts[0][0] + parts[1][0]).toLocaleUpperCase();
    }
  }
</script>

<div class="flex flex-row-reverse">
  {#each users as [id, user] (id)}
    <div
      class="mr-1 flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium first:mr-0"
      style:background="hsla({nameToHue(user.name)}, 80%, 30%, 90%)"
      transition:fade|local={{ duration: 200 }}
    >
      {nameToInitials(user.name)}
    </div>
  {/each}
</div>
