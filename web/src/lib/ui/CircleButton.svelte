<script lang="ts">
  import { MinusIcon, PlusIcon, XIcon } from "svelte-feather-icons";

  export let kind: keyof typeof details;

  const details = {
    red: {
      cls: "bg-red-500 active:bg-red-700",
      icon: XIcon,
    },
    yellow: {
      cls: "bg-yellow-500 active:bg-yellow-700",
      icon: MinusIcon,
    },
    green: {
      cls: "bg-green-500 active:bg-green-700",
      icon: PlusIcon,
    },
  };
</script>

<button
  class="relative w-3 h-3 p-[1px] rounded-full before:absolute before:hidden before:-inset-[11px] [@media(pointer:coarse)]:before:block {details[kind].cls}"
  on:pointerdown|stopPropagation
  on:click
>
  <svelte:component
    this={details[kind].icon}
    class="w-full h-full"
    strokeWidth={3}
  />
</button>
