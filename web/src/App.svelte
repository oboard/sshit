<script lang="ts">
  import "@fontsource-variable/inter";

  import { onDestroy, onMount, tick } from "svelte";

  import Avatars from "$lib/ui/Avatars.svelte";
  import ChooseName from "$lib/ui/ChooseName.svelte";
  import EditorWindow from "$lib/ui/EditorWindow.svelte";
  import LiveCursor from "$lib/ui/LiveCursor.svelte";
  import NameList from "$lib/ui/NameList.svelte";
  import NetworkInfo from "$lib/ui/NetworkInfo.svelte";
  import Settings from "$lib/ui/Settings.svelte";
  import ToastContainer from "$lib/ui/ToastContainer.svelte";
  import Toolbar from "$lib/ui/Toolbar.svelte";
  import { CollabConnection, type CollabStatus, collabConnection } from "$lib/collab";
  import { makeToast } from "$lib/toast";
  import { settings } from "$lib/settings";
  import type { WsUser } from "$lib/protocol";
  import type { EditorWindowState } from "$lib/editorWindows";
  import {
    drawingShapes,
    type DrawingAnchor,
    type DrawingShape,
  } from "$lib/yjsStore";

  import { arrangeNewTerminal } from "./arrange";
  import WebTerm, { type Shell } from "./WebTerm.svelte";

  type ServerUser = {
    id: number;
    name: string;
    x: number;
    y: number;
    cursor: boolean;
    cursorStyle?: string;
  };

  type Message = {
    type: string;
    id?: number;
    name?: string;
    x?: number;
    y?: number;
    cols?: number;
    rows?: number;
    width?: number;
    height?: number;
    data?: string;
    password?: string;
    users?: ServerUser[];
    shells?: Shell[];
    user?: ServerUser;
    shell?: Shell;
    editorWindows?: EditorWindowState[];
    editorWindow?: EditorWindowState;
    windowId?: number;
    patch?: Partial<EditorWindowState>;
  };

  let fabricEl: HTMLDivElement;
  let passwordInputEl: HTMLInputElement;
  let socket: WebSocket | null = null;
  let connected = false;
  let authRequired = false;
  let authenticated = false;
  let password = "";
  let authError = "";
  let clientID = 0;
  let users: ServerUser[] = [];
  let shells: Shell[] = [];
  let termRefs: Record<number, WebTerm> = {};
  let outputs: Record<number, string> = {};
  let zOrder: Record<number, number> = {};
  let topZ = 1;
  let settingsOpen = false;
  let showNetworkInfo = false;
  let editorWindows: EditorWindowState[] = [];
  let focusedShellID = -1;
  let focusedEditorWindowID = -1;
  const drawingColors = [
    { value: "#f472b6", name: "Pink" },
    { value: "#a78bfa", name: "Purple" },
    { value: "#60a5fa", name: "Blue" },
    { value: "#34d399", name: "Green" },
    { value: "#fbbf24", name: "Amber" },
    { value: "#f8fafc", name: "White" },
  ];
  const drawingWidths = [
    { value: 2, name: "Fine" },
    { value: 4, name: "Medium" },
    { value: 8, name: "Bold" },
  ];
  let drawingMode = false;
  let drawingColor = "#f472b6";
  let drawingStrokeWidth = 4;
  let shapes: DrawingShape[] = [];
  let draftShape: DrawingShape | null = null;
  let drawing = false;
  let activeDrawingAnchor: DrawingAnchor = { kind: "world" };
  let collabStatus: CollabStatus = "disconnected";
  let collabConn: CollabConnection | null = null;
  let serverLatency: number | null = null;
  let shellLatency: number | null = 0;
  let pingTimer: number | undefined;
  let reconnectTimer: number | undefined;
  let manualClose = false;
  let lastSentName = "";
  let viewportX = 0;
  let viewportY = 0;
  let zoom = 1;
  let panning = false;
  let panStart = [0, 0];
  let panOrigin = [0, 0];

  type ResizeTarget = { kind: "shell" | "collab"; id: number } | null;

  let movingShellID = -1;
  let movingEditorWindowID = -1;
  let movingStart = [0, 0];
  let movingOrigin = [0, 0];
  let resizeTarget: ResizeTarget = null;
  let resizingStart = [0, 0];
  let resizingOrigin = [0, 0];

  $: usersForUI = users.map((user) => [
    user.id,
    {
      name: user.name,
      cursor: user.cursor ? [user.x, user.y] : null,
      focus: null,
      canWrite: true,
      cursorStyle: user.cursorStyle,
    } satisfies WsUser & { cursorStyle?: string },
  ]) as [number, WsUser & { cursorStyle?: string }][];

  $: otherUsersForUI = usersForUI.filter(([id]) => id !== clientID);

  function setEditorWindows(windows: EditorWindowState[]) {
    editorWindows = [...windows].sort((a, b) => a.id - b.id);
    const highestSharedZ = editorWindows.reduce((max, window) => Math.max(max, window.zIndex || 1), 1);
    if (highestSharedZ > topZ) topZ = highestSharedZ;
  }

  function upsertEditorWindow(window: EditorWindowState) {
    setEditorWindows([...editorWindows.filter((item) => item.id !== window.id), window]);
  }

  function patchEditorWindowLocal(id: number, patch: Partial<EditorWindowState>) {
    const current = editorWindows.find((item) => item.id === id);
    if (!current) return;
    upsertEditorWindow({ ...current, ...patch });
  }

  function removeEditorWindow(id: number) {
    editorWindows = editorWindows.filter((item) => item.id !== id);
  }

  function refreshShapes() {
    shapes = Array.from(drawingShapes.values())
      .filter((shape) => shape.type === "path" && Array.isArray(shape.points))
      .sort((a, b) => a.id.localeCompare(b.id));
  }

  $: if (authRequired && !authenticated && passwordInputEl) {
    tick().then(() => passwordInputEl?.focus());
  }
  $: if (connected && $settings.name && $settings.name !== lastSentName) {
    send({ type: "setName", name: $settings.name });
    lastSentName = $settings.name;
  }

  function wsURL() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws`;
  }

  function send(msg: Message) {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(msg));
    }
  }

  function applyState(message: Message) {
    if (message.users !== undefined) users = message.users;
    if (message.shells !== undefined) {
      const previousShellCount = shells.length;
      const localShells = new Map(shells.map((shell) => [shell.id, shell]));
      shells = message.shells.map((incoming) => {
        const local = localShells.get(incoming.id);
        if (local && incoming.id === movingShellID) {
          return { ...incoming, x: local.x, y: local.y };
        }
        if (local && resizeTarget?.kind === "shell" && incoming.id === resizeTarget.id) {
          return {
            ...incoming,
            width: local.width,
            height: local.height,
            cols: local.cols,
            rows: local.rows,
          };
        }
        return incoming;
      });
      const nextOutputs = { ...outputs };
      for (const shell of message.shells) {
        zOrder[shell.id] ??= ++topZ;
        if (shell.buffer !== undefined) {
          nextOutputs[shell.id] = shell.buffer;
        }
      }
      for (const id of Object.keys(nextOutputs)) {
        if (!shells.some((shell) => shell.id === Number(id))) {
          delete nextOutputs[Number(id)];
        }
      }
      outputs = nextOutputs;
      zOrder = zOrder;
      if (previousShellCount === 0 && shells.length === 1) {
        const shell = shells[0];
        requestAnimationFrame(() => moveCanvasTo(shell.x, shell.y, 1));
      }
    }
    if (message.editorWindows !== undefined) setEditorWindows(message.editorWindows);
  }

  function connect() {
    socket?.close();
    socket = new WebSocket(wsURL());

    socket.onopen = () => {
      serverLatency = null;
      shellLatency = 0;
    };

    socket.onmessage = (event) => {
      let message: Message;
      try {
        message = JSON.parse(event.data);
      } catch (error) {
        console.warn("invalid websocket message", error);
        return;
      }

      if (message.type === "authRequired") {
        authRequired = true;
        authenticated = false;
        authError = "";
      } else if (message.type === "authFailed") {
        authRequired = true;
        authenticated = false;
        authError = "Incorrect password.";
      } else if (message.type === "hello") {
        authRequired = false;
        authenticated = true;
        connected = true;
        authError = "";
        const collabPassword = password;
        password = "";
        makeToast({ kind: "success", message: "Connected to sshit." });
        collabConn?.connect(collabPassword, $settings.name || `user-${message.id ?? 0}`, message.id ?? 0);
        if ($settings.name) {
          send({ type: "setName", name: $settings.name });
          lastSentName = $settings.name;
        }
        clientID = message.id ?? 0;
        applyState(message);
      } else if (message.type === "state") {
        applyState(message);
      } else if (message.type === "cursor" && message.user) {
        users = [...users.filter((user) => user.id !== message.user!.id), message.user];
      } else if (message.type === "output" && message.id && message.data !== undefined) {
        outputs = { ...outputs, [message.id]: (outputs[message.id] ?? "") + message.data };
      } else if (message.type === "editorWindowCreated" && message.editorWindow) {
        upsertEditorWindow(message.editorWindow);
      } else if (message.type === "editorWindowPatched" && message.windowId !== undefined && message.patch) {
        patchEditorWindowLocal(message.windowId, message.patch);
      } else if (message.type === "editorWindowClosed" && message.windowId !== undefined) {
        removeEditorWindow(message.windowId);
      }
    };

    socket.onclose = () => {
      connected = false;
      if (!manualClose) {
        makeToast({ kind: "error", message: "Disconnected. Reconnecting…" });
        reconnectTimer = window.setTimeout(connect, 1000);
      }
    };

    socket.onerror = () => {
      makeToast({ kind: "error", message: "WebSocket connection error." });
    };
  }

  function screenToWorld(clientX: number, clientY: number) {
    const rect = fabricEl.getBoundingClientRect();
    return [
      Math.round((clientX - rect.left - viewportX) / zoom),
      Math.round((clientY - rect.top - viewportY) / zoom),
    ];
  }

  function makeDrawingId() {
    return `${Date.now().toString(36)}-${clientID}-${Math.random().toString(36).slice(2, 8)}`;
  }

  function anchorOrigin(anchor: DrawingAnchor): [number, number] {
    if (anchor.kind === "editorWindow") {
      const window = editorWindows.find((item) => item.id === anchor.id);
      return [window?.x ?? 0, window?.y ?? 0];
    }
    if (anchor.kind === "shell") {
      const shell = shells.find((item) => item.id === anchor.id);
      return [shell?.x ?? 0, shell?.y ?? 0];
    }
    return [0, 0];
  }

  function drawingAnchorAt(x: number, y: number): DrawingAnchor {
    const collabHit = [...editorWindows]
      .sort((a, b) => (b.zIndex ?? 1) - (a.zIndex ?? 1))
      .find((window) => x >= window.x && x <= window.x + window.width && y >= window.y && y <= window.y + window.height);
    if (collabHit) return { kind: "editorWindow", id: collabHit.id };

    const shellHit = [...shells]
      .sort((a, b) => (zOrder[b.id] ?? 1) - (zOrder[a.id] ?? 1))
      .find((shell) => x >= shell.x && x <= shell.x + (shell.width || 760) && y >= shell.y && y <= shell.y + (shell.height || 420) + 42);
    if (shellHit) return { kind: "shell", id: shellHit.id };

    return { kind: "world" };
  }

  function pointForAnchor(anchor: DrawingAnchor, x: number, y: number): [number, number] {
    const [originX, originY] = anchorOrigin(anchor);
    return [x - originX, y - originY];
  }

  function pathData(shape: DrawingShape) {
    const [originX, originY] = anchorOrigin(shape.anchor);
    return shape.points
      .map((point, index) => `${index === 0 ? "M" : "L"} ${originX + point[0]} ${originY + point[1]}`)
      .join(" ");
  }

  function submitPassword() {
    if (!password) return;
    send({ type: "auth", password });
  }

  function moveCanvasTo(x: number, y: number, nextZoom = 1) {
    if (!fabricEl) return;
    const rect = fabricEl.getBoundingClientRect();
    const startX = viewportX;
    const startY = viewportY;
    const startZoom = zoom;
    const targetZoom = nextZoom;
    const targetX = Math.round(rect.width / 2 - (x + 380) * targetZoom);
    const targetY = Math.round(rect.height / 2 - (y + 220) * targetZoom);
    const start = performance.now();
    const duration = 350;

    function smoothstep(t: number) {
      const x = Math.max(0, Math.min(1, t));
      return x * x * (3 - 2 * x);
    }

    function frame(now: number) {
      const k = smoothstep((now - start) / duration);
      zoom = startZoom + (targetZoom - startZoom) * k;
      viewportX = Math.round(startX + (targetX - startX) * k);
      viewportY = Math.round(startY + (targetY - startY) * k);
      if (k < 1) requestAnimationFrame(frame);
    }

    requestAnimationFrame(frame);
  }

  function existingWindows() {
    return [
      ...shells.map((shell) => ({
        x: shell.x,
        y: shell.y,
        width: shell.width || 760,
        height: shell.height || 420,
      })),
      ...editorWindows.map((window) => ({
        x: window.x,
        y: window.y,
        width: window.width,
        height: window.height,
      })),
    ];
  }

  function createShell() {
    const existing = existingWindows();
    const { x, y } = arrangeNewTerminal(existing);
    send({
      type: "create",
      x,
      y,
      cols: 80,
      rows: 24,
    });
    moveCanvasTo(x, y, 1);
  }

  function createEditorWindow() {
    const width = 980;
    const height = 620;
    const { x, y } = arrangeNewTerminal(existingWindows());
    const id = Date.now() * 1000 + clientID;
    const zIndex = ++topZ;
    const window: EditorWindowState = { id, docId: `doc-${id.toString(36)}`, kind: "editor", x, y, width, height, zIndex };
    send({ type: "editorWindowCreate", editorWindow: window });
    upsertEditorWindow(window);
    moveCanvasTo(x, y, 1);
  }

  function focusShell(id: number) {
    focusedShellID = id;
    focusedEditorWindowID = -1;
    zOrder[id] = ++topZ;
    zOrder = zOrder;
  }

  function focusEditorWindow(id: number) {
    focusedShellID = -1;
    focusedEditorWindowID = id;
    const zIndex = ++topZ;
    send({ type: "editorWindowPatch", windowId: id, patch: { zIndex } });
    patchEditorWindowLocal(id, { zIndex });
  }

  function closeEditorWindow(id: number) {
    if (focusedEditorWindowID === id) focusedEditorWindowID = -1;
    send({ type: "editorWindowClose", windowId: id });
    removeEditorWindow(id);
  }

  function startMove(id: number, event: MouseEvent) {
    event.preventDefault();
    focusShell(id);
    const shell = shells.find((shell) => shell.id === id);
    if (!shell) return;
    movingShellID = id;
    movingEditorWindowID = -1;
    movingStart = [event.clientX, event.clientY];
    movingOrigin = [shell.x, shell.y];
  }

  function startEditorWindowMove(id: number, event: MouseEvent) {
    event.preventDefault();
    focusEditorWindow(id);
    const window = editorWindows.find((window) => window.id === id);
    if (!window) return;
    movingEditorWindowID = id;
    movingShellID = -1;
    movingStart = [event.clientX, event.clientY];
    movingOrigin = [window.x, window.y];
  }

  function startWindowResize(target: Exclude<ResizeTarget, null>, event: MouseEvent, width: number, height: number) {
    event.preventDefault();
    if (target.kind === "shell") {
      focusShell(target.id);
    } else {
      focusEditorWindow(target.id);
    }
    resizeTarget = target;
    resizingStart = [event.clientX, event.clientY];
    resizingOrigin = [width, height];
  }

  function startResize(id: number, event: MouseEvent, width: number, height: number) {
    startWindowResize({ kind: "shell", id }, event, width, height);
  }

  function startEditorWindowResize(id: number, event: MouseEvent, width: number, height: number) {
    startWindowResize({ kind: "collab", id }, event, width, height);
  }

  function startPan(event: MouseEvent) {
    if (event.button !== 0) return;
    if (drawingMode) return;
    const target = event.target as HTMLElement;
    if (target.closest("[data-no-pan]")) return;
    if (target.closest("[data-pan-surface]") === null) return;
    focusedShellID = -1;
    focusedEditorWindowID = -1;
    panning = true;
    panStart = [event.clientX, event.clientY];
    panOrigin = [viewportX, viewportY];
  }

  function startCanvasDrawing(event: PointerEvent) {
    if (!drawingMode || event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    (event.currentTarget as SVGSVGElement).setPointerCapture?.(event.pointerId);
    const [worldX, worldY] = screenToWorld(event.clientX, event.clientY);
    activeDrawingAnchor = drawingAnchorAt(worldX, worldY);
    const point = pointForAnchor(activeDrawingAnchor, worldX, worldY);
    draftShape = {
      id: makeDrawingId(),
      type: "path",
      anchor: activeDrawingAnchor,
      x: point[0],
      y: point[1],
      color: drawingColor,
      strokeWidth: drawingStrokeWidth,
      points: [point],
      createdBy: clientID,
    };
    drawing = true;
  }

  function continueCanvasDrawing(event: PointerEvent) {
    if (!drawingMode || !drawing || !draftShape) return;
    event.preventDefault();
    event.stopPropagation();
    const [worldX, worldY] = screenToWorld(event.clientX, event.clientY);
    const point = pointForAnchor(activeDrawingAnchor, worldX, worldY);
    draftShape = { ...draftShape, points: [...draftShape.points, point] };
  }

  function finishCanvasDrawing(event?: PointerEvent) {
    if (!drawing) return;
    event?.preventDefault();
    event?.stopPropagation();
    const target = event?.currentTarget as SVGSVGElement | null;
    if (event && target?.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId);
    if (draftShape && draftShape.points.length > 1) {
      drawingShapes.set(draftShape.id, draftShape);
    }
    draftShape = null;
    drawing = false;
  }

  function undoLastDrawing() {
    const latest = shapes
      .filter((shape) => shape.createdBy === clientID)
      .sort((a, b) => b.id.localeCompare(a.id))[0];
    if (latest) drawingShapes.delete(latest.id);
  }

  function clearMyDrawings() {
    for (const shape of shapes) {
      if (shape.createdBy === clientID) drawingShapes.delete(shape.id);
    }
  }

  function handleDrawingKeydown(event: KeyboardEvent) {
    if (!drawingMode || event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "Escape") {
      drawingMode = false;
      return;
    }
    if (event.key === "[") drawingStrokeWidth = Math.max(2, drawingStrokeWidth - 1);
    if (event.key === "]") drawingStrokeWidth = Math.min(12, drawingStrokeWidth + 1);
  }

  function handleWheel(event: WheelEvent) {
    if (drawingMode) return;
    event.preventDefault();
    const rect = fabricEl.getBoundingClientRect();
    const mouseX = event.clientX - rect.left;
    const mouseY = event.clientY - rect.top;
    const beforeX = (mouseX - viewportX) / zoom;
    const beforeY = (mouseY - viewportY) / zoom;
    const nextZoom = Math.min(2.5, Math.max(0.25, zoom * Math.exp(-event.deltaY * 0.001)));
    zoom = nextZoom;
    viewportX = mouseX - beforeX * zoom;
    viewportY = mouseY - beforeY * zoom;
  }

  function detectCursorStyle(clientX: number, clientY: number): string {
    // Find the topmost element at the given screen coordinates, excluding our own
    // remote-cursor overlays, and read its computed cursor style.
    const el = document.elementFromPoint(clientX, clientY);
    if (!el) return "default";
    // Walk up through the DOM to find the first element with a non-auto cursor.
    // This handles cases where the element itself has cursor: auto but a parent
    // sets cursor: pointer, etc.
    let target: Element | null = el;
    while (target) {
      const style = window.getComputedStyle(target).cursor;
      if (style && style !== "auto") return style;
      target = target.parentElement;
    }
    return "default";
  }

  function handleMouseMove(event: MouseEvent) {
    if (fabricEl) {
      const [x, y] = screenToWorld(event.clientX, event.clientY);
      const cursorStyle = detectCursorStyle(event.clientX, event.clientY);
      send({ type: "cursor", x, y, cursorStyle });
    }

    if (drawingMode) return;

    if (panning) {
      viewportX = Math.round(panOrigin[0] + event.clientX - panStart[0]);
      viewportY = Math.round(panOrigin[1] + event.clientY - panStart[1]);
      return;
    }

    if (resizeTarget) {
      const minWidth = resizeTarget.kind === "shell" ? 320 : 520;
      const minHeight = resizeTarget.kind === "shell" ? 180 : 360;
      const width = Math.max(minWidth, Math.round(resizingOrigin[0] + (event.clientX - resizingStart[0]) / zoom));
      const height = Math.max(minHeight, Math.round(resizingOrigin[1] + (event.clientY - resizingStart[1]) / zoom));
      if (resizeTarget.kind === "shell") {
        shells = shells.map((shell) => {
          if (shell.id !== resizeTarget?.id) return shell;
          const next = { ...shell, width, height };
          const fitted = termRefs[shell.id]?.fitSize();
          if (fitted) {
            next.cols = fitted.cols;
            next.rows = fitted.rows;
          }
          return next;
        });
      } else {
        send({ type: "editorWindowPatch", windowId: resizeTarget.id, patch: { width, height } });
        patchEditorWindowLocal(resizeTarget.id, { width, height });
      }
      return;
    }

    if (movingShellID !== -1) {
      const x = Math.round(movingOrigin[0] + (event.clientX - movingStart[0]) / zoom);
      const y = Math.round(movingOrigin[1] + (event.clientY - movingStart[1]) / zoom);
      shells = shells.map((shell) =>
        shell.id === movingShellID ? { ...shell, x, y } : shell,
      );
      return;
    }

    if (movingEditorWindowID !== -1) {
      const x = Math.round(movingOrigin[0] + (event.clientX - movingStart[0]) / zoom);
      const y = Math.round(movingOrigin[1] + (event.clientY - movingStart[1]) / zoom);
      send({ type: "editorWindowPatch", windowId: movingEditorWindowID, patch: { x, y } });
      patchEditorWindowLocal(movingEditorWindowID, { x, y });
    }
  }

  function stopMove() {
    panning = false;
    if (resizeTarget) {
      if (resizeTarget.kind === "shell") {
        const shell = shells.find((shell) => shell.id === resizeTarget?.id);
        if (shell) {
          const fitted = termRefs[shell.id]?.fitSize();
          const cols = fitted?.cols ?? shell.cols;
          const rows = fitted?.rows ?? shell.rows;
          send({ type: "resize", id: shell.id, width: shell.width, height: shell.height, cols, rows });
        }
      }
      resizeTarget = null;
    }
    if (movingShellID !== -1) {
      const shell = shells.find((shell) => shell.id === movingShellID);
      if (shell) send({ type: "move", id: shell.id, x: shell.x, y: shell.y });
      movingShellID = -1;
    }
    if (movingEditorWindowID !== -1) {
      movingEditorWindowID = -1;
    }
  }

  onMount(() => {
    refreshShapes();
    drawingShapes.observe(refreshShapes);
    collabConn = new CollabConnection((status) => {
      collabStatus = status;
      if (status === "auth-failed") {
        makeToast({ kind: "error", message: "Collaborative workspace authentication failed." });
      }
    });
    connect();
    pingTimer = window.setInterval(() => {
      const start = performance.now();
      fetch("/", { method: "HEAD", cache: "no-store" })
        .then(() => {
          serverLatency = Math.max(1, Math.round(performance.now() - start));
        })
        .catch(() => {
          serverLatency = null;
        });
    }, 2000);
  });

  onDestroy(() => {
    manualClose = true;
    window.clearInterval(pingTimer);
    window.clearTimeout(reconnectTimer);
    drawingShapes.unobserve(refreshShapes);
    collabConn?.destroy();
    socket?.close();
  });
</script>

<ToastContainer />
<svelte:window on:keydown={handleDrawingKeydown} />
{#if authenticated}
  <ChooseName />
{/if}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<main
  class="relative h-full w-full overflow-hidden text-zinc-100"
  role="application"
  aria-label="sshit workspace"
  bind:this={fabricEl}
  class:cursor-grabbing={panning}
  on:mousedown={startPan}
  on:mousemove={handleMouseMove}
  on:mouseup={stopMove}
  on:mouseleave={stopMove}
  on:wheel={handleWheel}
>
  <div class="absolute top-4 left-4 z-[12000]">
    <Toolbar
      {connected}
      hasWriteAccess={true}
      {drawingMode}
      on:create={createShell}
      on:createEditor={createEditorWindow}
      on:toggleDrawing={() => (drawingMode = !drawingMode)}
      on:settings={() => (settingsOpen = true)}
      on:networkInfo={() => (showNetworkInfo = !showNetworkInfo)}
    />
  </div>

  <div class="absolute top-5 right-5 z-[10000] flex items-center gap-3">
    <NameList users={usersForUI} />
    <Avatars users={usersForUI} />
  </div>

  {#if showNetworkInfo}
    <div class="absolute top-24 left-4 z-[10000] w-[360px] max-w-[calc(100vw-2rem)]">
      <NetworkInfo
        status={connected ? "connected" : "no-server"}
        {serverLatency}
        {shellLatency}
      />
    </div>
  {/if}

  <div data-pan-surface class="absolute inset-0 bg-[radial-gradient(circle_at_30%_0%,rgba(192,38,211,0.22),transparent_32rem),radial-gradient(circle_at_80%_20%,rgba(37,99,235,0.2),transparent_28rem),#111111]" class:cursor-crosshair={drawingMode}>
    <div class="absolute cursor-grab inset-0 opacity-20" style="background-image: radial-gradient(circle, #71717a 1px, transparent 1px); background-size: {32 * zoom}px {32 * zoom}px; background-position: {viewportX}px {viewportY}px;"></div>
    <div class="absolute left-4 bottom-4 z-[10000] rounded-full border border-white/10 bg-zinc-900/80 px-3 py-1 text-xs text-zinc-300">zoom {Math.round(zoom * 100)}%</div>

    <div class="absolute left-0 top-0 origin-top-left" style="transform: translate({viewportX}px, {viewportY}px) scale({zoom}); width: 1px; height: 1px;">
    {#each editorWindows as windowState (windowState.id)}
      <EditorWindow
        {windowState}
        zIndex={windowState.zIndex ?? 1}
        focused={focusedEditorWindowID === windowState.id}
        on:focus={(event) => focusEditorWindow(event.detail.id)}
        on:startMove={(event) => startEditorWindowMove(event.detail.id, event.detail.event)}
        on:startResize={(event) => startEditorWindowResize(event.detail.id, event.detail.event, event.detail.width, event.detail.height)}
        on:close={(event) => closeEditorWindow(event.detail.id)}
      />
    {/each}

    {#each shells as shell (shell.id)}
      <WebTerm
        bind:this={termRefs[shell.id]}
        {shell}
        output={outputs[shell.id] ?? ""}
        zIndex={zOrder[shell.id] ?? 1}
        focused={focusedShellID === shell.id}
        on:focus={(event) => focusShell(event.detail.id)}
        on:blur={() => { focusedShellID = -1; }}
        on:startMove={(event) => startMove(event.detail.id, event.detail.event)}
        on:startResize={(event) => startResize(event.detail.id, event.detail.event, event.detail.width, event.detail.height)}
        on:input={(event) => send({ type: "input", id: event.detail.id, data: event.detail.data })}
        on:resize={(event) => {
          shells = shells.map((shell) => shell.id === event.detail.id ? { ...shell, cols: event.detail.cols, rows: event.detail.rows, width: event.detail.width, height: event.detail.height } : shell);
          send({ type: "resize", id: event.detail.id, cols: event.detail.cols, rows: event.detail.rows, width: event.detail.width, height: event.detail.height });
        }}
        on:close={(event) => send({ type: "close", id: event.detail.id })}
      />
    {/each}

    {#each otherUsersForUI as [id, user] (id)}
      {#if user.cursor}
        <div class="pointer-events-none absolute z-[9999]" style="transform: translate({user.cursor[0]}px, {user.cursor[1]}px);">
          <LiveCursor {user} />
        </div>
      {/if}
    {/each}

    <svg class="pointer-events-none absolute left-0 top-0 z-[9998] h-px w-px overflow-visible" aria-label="Collaborative world drawings">
      {#each shapes as shape (shape.id)}
        <path
          d={pathData(shape)}
          fill="none"
          stroke={shape.color}
          stroke-width={shape.strokeWidth}
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      {/each}
      {#if draftShape}
        <path
          d={pathData(draftShape)}
          fill="none"
          stroke={draftShape.color}
          stroke-width={draftShape.strokeWidth}
          stroke-linecap="round"
          stroke-linejoin="round"
          opacity="0.9"
        />
      {/if}
    </svg>
    </div>
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <svg
    class={drawingMode ? "pointer-events-auto fixed inset-0 z-[11000] h-full w-full cursor-crosshair" : "pointer-events-none fixed inset-0 z-[11000] h-full w-full"}
    role="application"
    on:pointerdown={startCanvasDrawing}
    on:pointermove={continueCanvasDrawing}
    on:pointerup={finishCanvasDrawing}
    on:pointercancel={finishCanvasDrawing}
    on:pointerleave={finishCanvasDrawing}
    aria-label="Collaborative drawing capture layer"
  ></svg>

  {#if drawingMode}
    <section class="fixed bottom-4 left-1/2 z-[12000] w-[min(calc(100vw-2rem),620px)] -translate-x-1/2 rounded-2xl border border-indigo-300/25 bg-zinc-950/95 p-3 shadow-2xl backdrop-blur" data-no-pan aria-label="Drawing tools">
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <span class="grid h-8 w-8 place-items-center rounded-xl bg-indigo-500/20 text-indigo-200">✎</span>
          <div>
            <p class="text-sm font-semibold text-white">Draw</p>
            <p class="text-[11px] text-zinc-400">Esc returns to pointer</p>
          </div>
        </div>
        <button class="rounded-lg bg-zinc-800 px-3 py-2 text-xs font-medium text-zinc-200 transition hover:bg-zinc-700 focus:outline-none focus:ring-2 focus:ring-indigo-400" on:click={() => (drawingMode = false)}>Done <kbd class="ml-1 text-zinc-500">Esc</kbd></button>
      </div>

      <div class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-3 border-t border-white/10 pt-3">
        <fieldset class="flex items-center gap-2" aria-label="Stroke color">
          <legend class="sr-only">Stroke color</legend>
          <span class="text-xs font-medium text-zinc-400">Color</span>
          <div class="flex gap-1.5">
            {#each drawingColors as color (color.value)}
              <button
                type="button"
                class="grid h-7 w-7 place-items-center rounded-full transition hover:scale-110 focus:outline-none focus:ring-2 focus:ring-white"
                class:ring-2={drawingColor === color.value}
                class:ring-indigo-300={drawingColor === color.value}
                aria-label={color.name}
                aria-pressed={drawingColor === color.value}
                title={color.name}
                on:click={() => (drawingColor = color.value)}
              ><span class="h-5 w-5 rounded-full border border-black/20" style={`background: ${color.value}`}></span></button>
            {/each}
            <label class="relative grid h-7 w-7 cursor-pointer place-items-center overflow-hidden rounded-full border border-zinc-600 bg-zinc-800 text-base" title="Custom color">
              <span aria-hidden="true">+</span>
              <input class="absolute inset-0 h-full w-full cursor-pointer opacity-0" type="color" bind:value={drawingColor} aria-label="Custom drawing color" />
            </label>
          </div>
        </fieldset>

        <fieldset class="flex items-center gap-2" aria-label="Stroke width">
          <legend class="sr-only">Stroke width</legend>
          <span class="text-xs font-medium text-zinc-400">Size</span>
          <div class="flex rounded-lg bg-zinc-900 p-1 ring-1 ring-white/10">
            {#each drawingWidths as width (width.value)}
              <button type="button" class="grid h-7 w-9 place-items-center rounded-md transition hover:bg-zinc-800 focus:outline-none focus:ring-2 focus:ring-indigo-400" class:bg-indigo-500={drawingStrokeWidth === width.value} aria-label={`${width.name} stroke`} aria-pressed={drawingStrokeWidth === width.value} title={width.name} on:click={() => (drawingStrokeWidth = width.value)}>
                <span class="rounded-full bg-white" style={`width: ${Math.max(5, width.value * 2)}px; height: ${Math.max(5, width.value * 2)}px`}></span>
              </button>
            {/each}
          </div>
        </fieldset>

        <div class="ml-auto flex items-center gap-2">
          <button class="rounded-lg px-2.5 py-2 text-xs text-zinc-300 transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-40" disabled={!shapes.some((shape) => shape.createdBy === clientID)} on:click={undoLastDrawing} title="Undo your last stroke">Undo</button>
          <button class="rounded-lg px-2.5 py-2 text-xs text-zinc-300 transition hover:bg-red-500/15 hover:text-red-200 disabled:cursor-not-allowed disabled:opacity-40" disabled={!shapes.some((shape) => shape.createdBy === clientID)} on:click={clearMyDrawings} title="Remove all of your strokes">Clear mine</button>
        </div>
      </div>
    </section>
  {/if}

  {#if authRequired && !authenticated}
    <div class="fixed inset-0 z-[20000] grid place-items-center bg-black/40 backdrop-blur-sm">
      <form class="panel w-[min(92vw,420px)] p-6" on:submit|preventDefault={submitPassword}>
        <h2 class="mb-2 text-xl font-medium">Password Required</h2>
        <p class="mb-4 text-sm text-zinc-400">Enter the shared password to access this sshit session.</p>
        <input
          class="mb-2 w-full rounded border border-zinc-700 bg-zinc-800 px-3 py-2 outline-none focus:ring-2 focus:ring-indigo-500/50"
          type="password"
          placeholder="Password"
          bind:value={password}
          bind:this={passwordInputEl}
        />
        {#if authError}
          <p class="mb-3 text-sm text-red-300">{authError}</p>
        {/if}
        <button class="mt-2 rounded bg-pink-700 px-4 py-2 font-medium hover:bg-pink-600" type="submit">
          Unlock
        </button>
      </form>
    </div>
  {/if}

  <Settings open={settingsOpen} on:close={() => (settingsOpen = false)} />
</main>
