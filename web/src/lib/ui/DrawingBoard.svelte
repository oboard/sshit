<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import {
    CircleIcon,
    Edit3Icon,
    MousePointerIcon,
    RefreshCcwIcon,
    SquareIcon,
    FileTextIcon,
    Trash2Icon,
  } from "svelte-feather-icons";

  import { drawingShapes, type DrawingShape } from "$lib/yjsStore";

  export let userId = 0;
  export let compact = false;

  type Tool = "pen" | "rect" | "ellipse" | "note" | "select";

  const colors = ["#f472b6", "#818cf8", "#34d399", "#fbbf24", "#f87171", "#e5e7eb"];

  let boardEl: HTMLDivElement;
  let tool: Tool = "pen";
  let color = colors[0];
  let strokeWidth = 4;
  let shapes: DrawingShape[] = [];
  let draft: DrawingShape | null = null;
  let selectedId = "";
  let isDrawing = false;
  let startPoint: [number, number] = [0, 0];

  $: sortedShapes = [...shapes].sort((a, b) => a.id.localeCompare(b.id));

  function refreshShapes() {
    shapes = Array.from(drawingShapes.values());
  }

  function pointFromEvent(event: PointerEvent): [number, number] {
    const rect = boardEl.getBoundingClientRect();
    return [Math.round(event.clientX - rect.left), Math.round(event.clientY - rect.top)];
  }

  function makeId() {
    return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
  }

  function startDrawing(event: PointerEvent) {
    if (event.button !== 0) return;
    const point = pointFromEvent(event);

    if (tool === "select") {
      selectedId = "";
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    boardEl.setPointerCapture(event.pointerId);
    isDrawing = true;
    startPoint = point;

    if (tool === "note") {
      const text = window.prompt("便签内容", "新的想法");
      if (text) {
        const id = makeId();
        drawingShapes.set(id, {
          id,
          type: "note",
          x: point[0],
          y: point[1],
          width: 180,
          height: 96,
          color,
          strokeWidth: 1,
          text,
          createdBy: userId,
        });
      }
      isDrawing = false;
      return;
    }

    draft = {
      id: makeId(),
      type: tool === "pen" ? "path" : tool,
      x: point[0],
      y: point[1],
      width: 0,
      height: 0,
      color,
      strokeWidth,
      points: tool === "pen" ? [point] : undefined,
      createdBy: userId,
    };
  }

  function moveDrawing(event: PointerEvent) {
    if (!isDrawing || !draft) return;
    event.preventDefault();
    event.stopPropagation();
    const point = pointFromEvent(event);

    if (draft.type === "path") {
      draft = { ...draft, points: [...(draft.points ?? []), point] };
    } else {
      draft = {
        ...draft,
        x: Math.min(startPoint[0], point[0]),
        y: Math.min(startPoint[1], point[1]),
        width: Math.abs(point[0] - startPoint[0]),
        height: Math.abs(point[1] - startPoint[1]),
      };
    }
  }

  function finishDrawing(event: PointerEvent) {
    if (!isDrawing) return;
    event.preventDefault();
    event.stopPropagation();
    if (draft && isShapeVisible(draft)) {
      drawingShapes.set(draft.id, draft);
      selectedId = draft.id;
    }
    draft = null;
    isDrawing = false;
  }

  function isShapeVisible(shape: DrawingShape) {
    if (shape.type === "path") return (shape.points?.length ?? 0) > 1;
    if (shape.type === "note") return Boolean(shape.text);
    return (shape.width ?? 0) > 4 && (shape.height ?? 0) > 4;
  }

  function pathData(shape: DrawingShape) {
    const points = shape.points ?? [];
    if (!points.length) return "";
    return points.map((point, index) => `${index === 0 ? "M" : "L"} ${point[0]} ${point[1]}`).join(" ");
  }

  function selectShape(event: MouseEvent, id: string) {
    if (tool !== "select") return;
    event.stopPropagation();
    selectedId = id;
  }

  function deleteSelected() {
    if (!selectedId) return;
    drawingShapes.delete(selectedId);
    selectedId = "";
  }

  function clearBoard() {
    if (!window.confirm("清空协作画板？")) return;
    drawingShapes.doc?.transact(() => {
      for (const id of Array.from(drawingShapes.keys())) drawingShapes.delete(id);
    });
    selectedId = "";
  }

  onMount(() => {
    refreshShapes();
    drawingShapes.observe(refreshShapes);
  });

  onDestroy(() => {
    drawingShapes.unobserve(refreshShapes);
  });
</script>

<section class="panel board-shell" class:compact data-no-pan>
  {#if !compact}
    <div class="board-header">
      <div>
        <p class="eyebrow">Yjs Drawing</p>
        <h2>协作画板</h2>
      </div>
      <div class="text-xs text-zinc-400">{shapes.length} objects</div>
    </div>
  {/if}

  <div class="board-toolbar">
    <button class="tool-button" class:active={tool === "pen"} on:click={() => (tool = "pen")} title="画笔">
      <Edit3Icon strokeWidth={1.5} class="h-4 w-4" />
    </button>
    <button class="tool-button" class:active={tool === "rect"} on:click={() => (tool = "rect")} title="矩形">
      <SquareIcon strokeWidth={1.5} class="h-4 w-4" />
    </button>
    <button class="tool-button" class:active={tool === "ellipse"} on:click={() => (tool = "ellipse")} title="圆形">
      <CircleIcon strokeWidth={1.5} class="h-4 w-4" />
    </button>
    <button class="tool-button" class:active={tool === "note"} on:click={() => (tool = "note")} title="便签">
      <FileTextIcon strokeWidth={1.5} class="h-4 w-4" />
    </button>
    <button class="tool-button" class:active={tool === "select"} on:click={() => (tool = "select")} title="选择">
      <MousePointerIcon strokeWidth={1.5} class="h-4 w-4" />
    </button>

    <div class="divider"></div>

    <label class="slider-label">
      粗细
      <input type="range" min="2" max="12" bind:value={strokeWidth} />
    </label>

    <div class="swatches">
      {#each colors as swatch}
        <button
          class="swatch"
          class:active={color === swatch}
          style="background-color: {swatch}"
          aria-label="选择颜色 {swatch}"
          on:click={() => (color = swatch)}
        ></button>
      {/each}
    </div>

    <div class="ml-auto flex gap-2">
      <button class="tool-button" disabled={!selectedId} on:click={deleteSelected} title="删除选中">
        <Trash2Icon strokeWidth={1.5} class="h-4 w-4" />
      </button>
      <button class="tool-button" on:click={clearBoard} title="清空画板">
        <RefreshCcwIcon strokeWidth={1.5} class="h-4 w-4" />
      </button>
    </div>
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="board-canvas"
    role="application"
    bind:this={boardEl}
    class:selecting={tool === "select"}
    on:pointerdown={startDrawing}
    on:pointermove={moveDrawing}
    on:pointerup={finishDrawing}
    on:pointercancel={finishDrawing}
  >
    <svg class="absolute inset-0 h-full w-full" aria-label="Collaborative drawing surface">
      <defs>
        <pattern id="board-grid" width="28" height="28" patternUnits="userSpaceOnUse">
          <path d="M 28 0 L 0 0 0 28" fill="none" stroke="rgba(113,113,122,0.18)" stroke-width="1" />
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#board-grid)" />

      {#each sortedShapes as shape (shape.id)}
        {#if shape.type === "path"}
          <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
          <path
            d={pathData(shape)}
            fill="none"
            stroke={shape.color}
            stroke-width={shape.strokeWidth}
            stroke-linecap="round"
            stroke-linejoin="round"
            class:selected={selectedId === shape.id}
            on:click={(event) => selectShape(event, shape.id)}
          />
        {:else if shape.type === "rect"}
          <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
          <rect
            x={shape.x}
            y={shape.y}
            width={shape.width}
            height={shape.height}
            rx="12"
            fill={`${shape.color}22`}
            stroke={shape.color}
            stroke-width={shape.strokeWidth}
            class:selected={selectedId === shape.id}
            on:click={(event) => selectShape(event, shape.id)}
          />
        {:else if shape.type === "ellipse"}
          <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
          <ellipse
            cx={shape.x + (shape.width ?? 0) / 2}
            cy={shape.y + (shape.height ?? 0) / 2}
            rx={(shape.width ?? 0) / 2}
            ry={(shape.height ?? 0) / 2}
            fill={`${shape.color}22`}
            stroke={shape.color}
            stroke-width={shape.strokeWidth}
            class:selected={selectedId === shape.id}
            on:click={(event) => selectShape(event, shape.id)}
          />
        {:else if shape.type === "note"}
          <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
          <foreignObject
            x={shape.x}
            y={shape.y}
            width={shape.width}
            height={shape.height}
            class:selected={selectedId === shape.id}
            on:click={(event) => selectShape(event, shape.id)}
          >
            <div class="note" style="border-color: {shape.color}; background: {shape.color}22;">
              {shape.text}
            </div>
          </foreignObject>
        {/if}
      {/each}

      {#if draft}
        {#if draft.type === "path"}
          <path d={pathData(draft)} fill="none" stroke={draft.color} stroke-width={draft.strokeWidth} stroke-linecap="round" stroke-linejoin="round" opacity="0.85" />
        {:else if draft.type === "rect"}
          <rect x={draft.x} y={draft.y} width={draft.width} height={draft.height} rx="12" fill={`${draft.color}22`} stroke={draft.color} stroke-width={draft.strokeWidth} opacity="0.85" />
        {:else if draft.type === "ellipse"}
          <ellipse cx={draft.x + (draft.width ?? 0) / 2} cy={draft.y + (draft.height ?? 0) / 2} rx={(draft.width ?? 0) / 2} ry={(draft.height ?? 0) / 2} fill={`${draft.color}22`} stroke={draft.color} stroke-width={draft.strokeWidth} opacity="0.85" />
        {/if}
      {/if}
    </svg>

    <div class="hint">{tool === "select" ? "点击对象选择，删除或清空" : "拖拽绘制，所有更改会同步给协作者"}</div>
  </div>
</section>

<style lang="postcss">
  .board-shell {
    @apply flex h-full min-h-[520px] flex-col overflow-hidden shadow-2xl shadow-black/30;
  }

  .board-shell.compact {
    @apply min-h-0 rounded-none border-0 bg-transparent shadow-none;
  }

  .board-header {
    @apply flex items-center justify-between border-b border-zinc-800 px-5 py-4;
  }

  .board-header h2 {
    @apply text-lg font-semibold tracking-tight text-zinc-100;
  }

  .eyebrow {
    @apply mb-1 text-xs uppercase tracking-[0.2em] text-indigo-300/80;
  }

  .board-toolbar {
    @apply flex flex-wrap items-center gap-2 border-b border-zinc-800 bg-zinc-950/40 px-4 py-3;
  }

  .compact .board-toolbar {
    @apply px-3 py-2;
  }

  .compact .tool-button {
    @apply p-1.5;
  }

  .tool-button {
    @apply inline-flex items-center justify-center rounded-md border border-zinc-800 bg-zinc-950/60 p-2 text-zinc-300 transition-colors hover:bg-zinc-800 hover:text-white active:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-40;
  }

  .tool-button.active {
    @apply border-indigo-500/60 bg-indigo-600/20 text-white;
  }

  .divider {
    @apply mx-1 h-6 border-l border-zinc-800;
  }

  .slider-label {
    @apply flex items-center gap-2 text-xs text-zinc-400;
  }

  input[type="range"] {
    @apply w-20 accent-indigo-500;
  }

  .swatches {
    @apply flex gap-1.5;
  }

  .swatch {
    @apply h-6 w-6 rounded-full border-2 border-zinc-800 transition-transform hover:scale-110;
  }

  .swatch.active {
    @apply border-white shadow shadow-white/20;
  }

  .board-canvas {
    @apply relative min-h-0 flex-1 overflow-hidden bg-zinc-950/40;
    cursor: crosshair;
    touch-action: none;
  }

  .board-canvas.selecting {
    cursor: default;
  }

  svg :global(.selected) {
    filter: drop-shadow(0 0 8px rgba(129, 140, 248, 0.9));
  }

  .note {
    @apply h-full w-full overflow-hidden rounded-xl border p-3 text-sm leading-5 text-zinc-100 shadow-lg shadow-black/20 backdrop-blur-sm;
  }

  .hint {
    @apply pointer-events-none absolute bottom-3 left-3 rounded-full border border-white/10 bg-zinc-950/80 px-3 py-1 text-xs text-zinc-400;
  }
</style>
