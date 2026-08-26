<script lang="ts">
  import WindowFrame from "$lib/ui/WindowFrame.svelte";

  export let id: number;
  export let title: string;
  export let x: number;
  export let y: number;
  export let width: number;
  export let height: number;
  export let zIndex = 1;
  export let focused = false;
  export let tiled = false;
  export let layoutAnimating = false;
  export let tileResizing = false;
  export let tilePaneId: number | null = null;
  export let background = "#111111";
  export let ariaLabel = title;
  export let resizeLabel = `Resize ${title}`;
  export let onClose: (id: number) => void = () => {};
  export let onYellow: (id: number) => void = () => {};
  export let onGreen: (id: number) => void = () => {};
  export let onFocus: (id: number) => void = () => {};
  export let onStartMove: (id: number, event: PointerEvent) => void = () => {};
  export let onStartResize: (id: number, event: PointerEvent, width: number, height: number) => void = () => {};
  export let onTitlebarDoubleClick: (id: number) => void = () => {};
</script>

<WindowFrame
  {id}
  {title}
  {x}
  {y}
  {width}
  {height}
  {zIndex}
  {focused}
  {background}
  {ariaLabel}
  {resizeLabel}
  {tiled}
  {layoutAnimating}
  {tileResizing}
  {tilePaneId}
  {onClose}
  {onYellow}
  {onGreen}
  {onFocus}
  {onStartMove}
  {onStartResize}
  {onTitlebarDoubleClick}
>
  <slot />
</WindowFrame>
