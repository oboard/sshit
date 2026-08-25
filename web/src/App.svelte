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
  import WorkspaceMode from "$lib/ui/WorkspaceMode.svelte";
  import { CollabConnection, type CollabStatus } from "$lib/collab";
  import { makeToast } from "$lib/toast";
  import { settings } from "$lib/settings";
  import type { WsUser, WindowState, WindowPatch } from "$lib/protocol";
  import {
    drawingShapes,
    type DrawingAnchor,
    type DrawingShape,
  } from "$lib/yjsStore";

  import { arrangeNewTerminal } from "./arrange";
  import {
    leafOrder,
    neighborPane,
    moveWindowDirection,
    toggleSplitAxis,
    swapLeaves,
    type TileNode,
    type TileAxis,
  } from "$lib/tiling/tiling";
  import { handleTilingKey, type ActionTable } from "$lib/tiling/keybinds";
  import WebTerm from "./WebTerm.svelte";

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
    windows?: WindowState[];
    user?: ServerUser;
    windowId?: number;
    patch?: WindowPatch;
    tileLayout?: { tree: TileNode | null; floated: number[] };
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
  let windows: WindowState[] = [];
  let termRefs: Record<number, WebTerm> = {};
  let outputs: Record<number, string> = {};
  let pendingTiledPtySizes: Record<number, { cols: number; rows: number }> = {};
  let tiledPtyFlushTimer: number | undefined;
  let topZ = 1;
  let settingsOpen = false;
  let showNetworkInfo = false;
  let focusedWindowID = -1;
  let workspaceMode: "floating" | "tiled" = "floating";
  let tiledViewport = { width: 0, height: 0 };
  let tileTree: TileNode | null = null;
  let layoutAnimating = false;
  let layoutAnimationTimer: number | undefined;
  // Windows floated out of the tiled tree (drag-out). They render as floating
  // overlays above the tiled grid. This is the presentational counterpart of
  // Hyprland floating a window during `mousemove`.
  let floatedTileIds: number[] = [];
  // A remote tree can arrive before this client's shell WebSocket delivers the
  // matching window list. Keep it until that list is ready instead of dropping
  // a valid collaborator reorder.
  let pendingSharedTileLayout: {
    tree: TileNode | null;
    floated: number[];
  } | null = null;
  // The pane that currently owns keyboard/mouse focus in tiled mode. Mirrors
  // Hyprland's `focusState`: a single shared focus target for both input paths.
  let tiledFocusId = -1;
  // Ctrl is the modifier that primes tiled chords and drag-to-reorder.
  let ctrlModifierHeld = false;
  const TILED_DRAG_THRESHOLD = 8;
  let tiledReorderPotential: {
    windowId: number;
    downX: number;
    downY: number;
  } | null = null;
  let tiledReorderDrag: {
    windowId: number;
    width: number;
    height: number;
    originX: number;
    originY: number;
    downX: number;
    downY: number;
    ghostX: number;
    ghostY: number;
    lastTarget: number | null;
    targets: Array<{
      windowId: number;
      x: number;
      y: number;
      width: number;
      height: number;
    }>;
  } | null = null;
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
  let floatingCamera: { x: number; y: number; zoom: number } | null = null;
  let panning = false;
  let panStart = [0, 0];
  let panOrigin = [0, 0];

  // Touch state: pointers that went down on the empty canvas (pan surface).
  // One finger pans; a second finger promotes the gesture to a pinch zoom.
  let surfacePointers = new Map<number, [number, number]>();
  let pinch: {
    zoom0: number;
    viewport0: [number, number];
    mid0: [number, number];
    dist0: number;
  } | null = null;
  let tapCandidate: {
    id: number;
    x: number;
    y: number;
    time: number;
    moved: boolean;
  } | null = null;
  let lastTap = { time: 0, x: 0, y: 0 };

  // Reused across every binary frame: terminal output is high-frequency, so
  // creating a new TextDecoder per message would add avoidable allocation.
  const binDecoder = new TextDecoder();
  const MIN_ZOOM = 0.25;
  const MAX_ZOOM = 2.5;
  const clampZoom = (z: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z));

  let movingWindowID = -1;
  let movingStart = [0, 0];
  let movingOrigin = [0, 0];
  let resizingWindowID = -1;
  let resizingStart = [0, 0];
  let resizingOrigin = [0, 0];

  $: shellWindows = windows.filter((w) => w.kind === "shell");
  $: if (workspaceMode === "tiled") {
    const pending = pendingSharedTileLayout;
    const currentWindowIds = new Set(
      windows.map((windowState) => windowState.id),
    );
    const pendingLeaves = tileLeaves(pending?.tree ?? null);
    const pendingTiledIds = new Set(
      [...currentWindowIds].filter((id) => !pending?.floated.includes(id)),
    );
    const pendingMatches =
      pending !== null &&
      pendingLeaves.length === pendingTiledIds.size &&
      pendingLeaves.every((id) => pendingTiledIds.has(id));
    if (pendingMatches) {
      tileTree = pending.tree;
      floatedTileIds = pending.floated;
      pendingSharedTileLayout = null;
      void applyTiledLayout();
    } else {
      syncTileTree(windows.map((windowState) => windowState.id));
    }
  }
  // Keep tileTree as an explicit dependency. Svelte's legacy reactivity does
  // not discover variables referenced only inside tiledWindows(), which meant
  // splitter drags updated the tree but did not redraw panes until mouseup.
  $: displayWindows = (
    workspaceMode === "tiled"
      ? tiledWindows(windows, tiledViewport, tileTree)
      : windows
  )
    // Keep the real pane under the pointer while every other pane reflows around it.
    .map((windowState) =>
      windowState.id === tiledReorderDrag?.windowId
        ? {
            ...windowState,
            x: tiledReorderDrag.ghostX,
            y: tiledReorderDrag.ghostY,
            zIndex: 1000,
          }
        : windowState,
    );

  // Floating overlays that escape the tiled grid while still being part of the
  // same workspace (windows floated out via Ctrl+drag).
  $: floatedOverlays =
    workspaceMode === "tiled"
      ? floatedTileIds
          .map((id) => windows.find((w) => w.id === id))
          .filter((w): w is WindowState => !!w)
      : [];

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

  function windowById(id: number) {
    return windows.find((w) => w.id === id);
  }

  function isShell(id: number) {
    return windowById(id)?.kind === "shell";
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
    if (message.tileLayout !== undefined)
      pendingSharedTileLayout = message.tileLayout;
    if (message.windows !== undefined) {
      const previousCount = windows.length;
      const localById = new Map(windows.map((w) => [w.id, w]));
      windows = message.windows.map((incoming) => {
        topZ = Math.max(topZ, incoming.zIndex || 1);
        const local = localById.get(incoming.id);
        if (!local) return incoming;
        // Preserve in-flight local geometry while dragging/resizing.
        if (incoming.id === movingWindowID) {
          return { ...incoming, x: local.x, y: local.y };
        }
        if (incoming.id === resizingWindowID) {
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
      for (const w of message.windows) {
        if (w.kind === "shell" && w.buffer !== undefined) {
          nextOutputs[w.id] = w.buffer;
        }
      }
      for (const id of Object.keys(nextOutputs)) {
        if (!windows.some((w) => w.id === Number(id) && w.kind === "shell")) {
          delete nextOutputs[Number(id)];
        }
      }
      outputs = nextOutputs;

      if (
        previousCount === 0 &&
        windows.length >= 1 &&
        workspaceMode === "floating"
      ) {
        const target = windows.find((w) => w.kind === "shell") ?? windows[0];
        const fitZoom = fitZoomFor(target.width || 760);
        requestAnimationFrame(() =>
          moveCanvasTo(
            target.x,
            target.y,
            fitZoom,
            target.width || 760,
            target.height || 420,
          ),
        );
      }
    }
  }

  function connect() {
    socket?.close();
    socket = new WebSocket(wsURL());
    // Output arrives as raw binary frames to cut JSON overhead; reading as
    // ArrayBuffer keeps decoding cheap (no Blob async round-trip).
    socket.binaryType = "arraybuffer";

    const currentSocket = socket;
    currentSocket.onopen = () => {
      serverLatency = null;
      shellLatency = 0;
    };

    currentSocket.onmessage = (event) => {
      if (socket !== currentSocket) return;
      // Binary frames carry terminal output for a specific shell window:
      // [version][window id (int64 BE)][raw bytes]. JSON text frames are the
      // control plane (auth/state/hello/…). Separation means the hot path
      // never has to JSON-escape or parse terminal data.
      if (typeof event.data !== "string") {
        const d = new DataView(event.data);
        if (d.byteLength >= 9 && d.getUint8(0) === 1) {
          const wid = d.getBigInt64(1, false);
          if (wid >= 0n) {
            const bytes = new Uint8Array(event.data, 9);
            const data = binDecoder.decode(bytes);
            if (wid <= 0x7fffffff) {
              outputs = {
                ...outputs,
                [Number(wid)]: (outputs[Number(wid)] ?? "") + data,
              };
            }
          }
        }
        return;
      }

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
        collabConn?.connect(
          collabPassword,
          $settings.name || `user-${message.id ?? 0}`,
          message.id ?? 0,
        );
        if ($settings.name) {
          send({ type: "setName", name: $settings.name });
          lastSentName = $settings.name;
        }
        clientID = message.id ?? 0;
        applyState(message);
      } else if (message.type === "state") {
        applyState(message);
      } else if (message.type === "cursor" && message.user) {
        users = [
          ...users.filter((user) => user.id !== message.user!.id),
          message.user,
        ];
      } else if (
        message.type === "output" &&
        message.id &&
        message.data !== undefined
      ) {
        outputs = {
          ...outputs,
          [message.id]: (outputs[message.id] ?? "") + message.data,
        };
      }
    };

    currentSocket.onclose = () => {
      // `connect()` replaces and closes a previous socket. Its late close event
      // must not schedule another connection (and another /collab auth cycle).
      if (socket !== currentSocket) return;
      socket = null;
      connected = false;
      if (!manualClose) {
        makeToast({ kind: "error", message: "Disconnected. Reconnecting…" });
        window.clearTimeout(reconnectTimer);
        reconnectTimer = window.setTimeout(connect, 1000);
      }
    };

    currentSocket.onerror = () => {
      if (socket === currentSocket) {
        makeToast({ kind: "error", message: "WebSocket connection error." });
      }
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
    if (anchor.kind === "editorWindow" || anchor.kind === "shell") {
      const win = windowById(anchor.id);
      return [win?.x ?? 0, win?.y ?? 0];
    }
    return [0, 0];
  }

  function drawingAnchorAt(x: number, y: number): DrawingAnchor {
    const hit = [...windows]
      .sort((a, b) => (b.zIndex ?? 1) - (a.zIndex ?? 1))
      .find(
        (w) =>
          x >= w.x &&
          x <= w.x + w.width &&
          y >= w.y &&
          y <= w.y + w.height + 42,
      );
    if (hit) {
      return hit.kind === "editor"
        ? { kind: "editorWindow", id: hit.id }
        : { kind: "shell", id: hit.id };
    }
    return { kind: "world" };
  }

  function pointForAnchor(
    anchor: DrawingAnchor,
    x: number,
    y: number,
  ): [number, number] {
    const [originX, originY] = anchorOrigin(anchor);
    return [x - originX, y - originY];
  }

  function pathData(shape: DrawingShape) {
    const [originX, originY] = anchorOrigin(shape.anchor);
    return shape.points
      .map(
        (point, index) =>
          `${index === 0 ? "M" : "L"} ${originX + point[0]} ${originY + point[1]}`,
      )
      .join(" ");
  }

  function submitPassword() {
    if (!password) return;
    send({ type: "auth", password });
  }

  /**
   * Zoom level that fits a window of the given world width on small screens.
   * On desktop viewports this always returns 1.
   */
  function fitZoomFor(width: number) {
    if (!fabricEl) return 1;
    const rect = fabricEl.getBoundingClientRect();
    return Math.min(1, (rect.width - 24) / (width + 40));
  }

  function moveCanvasTo(x: number, y: number, nextZoom = 1, w = 760, h = 420) {
    if (!fabricEl) return;
    const rect = fabricEl.getBoundingClientRect();
    const startX = viewportX;
    const startY = viewportY;
    const startZoom = zoom;
    const targetZoom = nextZoom;
    const targetX = Math.round(rect.width / 2 - (x + w / 2) * targetZoom);
    const targetY = Math.round(rect.height / 2 - (y + h / 2) * targetZoom);
    animateViewTo(targetX, targetY, targetZoom, startX, startY, startZoom, 350);
  }

  let activeViewAnimation: number | undefined;

  // Cubic ease-out: it covers distance quickly then settles gently, avoiding
  // the mechanical feel of the old linear-ish smoothstep camera transition.
  function cameraEase(t: number) {
    const x = Math.max(0, Math.min(1, t));
    return 1 - Math.pow(1 - x, 4);
  }

  function animateViewTo(
    targetX: number,
    targetY: number,
    targetZoom: number,
    startX = viewportX,
    startY = viewportY,
    startZoom = zoom,
    duration = 420,
  ) {
    window.cancelAnimationFrame(activeViewAnimation);
    const start = performance.now();

    function frame(now: number) {
      const progress = Math.max(0, Math.min(1, (now - start) / duration));
      const k = cameraEase(progress);
      zoom = startZoom + (targetZoom - startZoom) * k;
      viewportX = Math.round(startX + (targetX - startX) * k);
      viewportY = Math.round(startY + (targetY - startY) * k);
      if (progress < 1) activeViewAnimation = requestAnimationFrame(frame);
    }

    activeViewAnimation = requestAnimationFrame(frame);
  }

  /** Animate a zoom change while keeping a screen point stationary. */
  function zoomAtPoint(clientX: number, clientY: number, targetZoom: number) {
    if (!fabricEl) return;
    const rect = fabricEl.getBoundingClientRect();
    const px = clientX - rect.left;
    const py = clientY - rect.top;
    const worldX = (px - viewportX) / zoom;
    const worldY = (py - viewportY) / zoom;
    animateViewTo(
      px - worldX * targetZoom,
      py - worldY * targetZoom,
      targetZoom,
    );
  }

  function resetZoom() {
    if (!fabricEl) return;
    const rect = fabricEl.getBoundingClientRect();
    zoomAtPoint(rect.left + rect.width / 2, rect.top + rect.height / 2, 1);
  }

  function handleDoubleTap(clientX: number, clientY: number) {
    // Toggle between overview (100%) and a close-up (200%) at the tapped point.
    zoomAtPoint(clientX, clientY, Math.abs(zoom - 1) < 0.01 ? 2 : 1);
  }

  function existingWindows() {
    return windows.map((w) => ({
      x: w.x,
      y: w.y,
      width: w.width,
      height: w.height,
    }));
  }

  function createShell() {
    const { x, y } = arrangeNewTerminal(existingWindows());
    send({
      type: "create",
      kind: "shell",
      x,
      y,
      cols: 80,
      rows: 24,
      width: 760,
      height: 420,
    });
    if (workspaceMode === "floating")
      moveCanvasTo(x, y, fitZoomFor(760), 760, 420);
  }

  function createEditorWindow() {
    const { x, y } = arrangeNewTerminal(existingWindows());
    send({ type: "create", kind: "editor", x, y, width: 980, height: 620 });
    if (workspaceMode === "floating")
      moveCanvasTo(x, y, fitZoomFor(980), 980, 620);
  }

  let nextTileSplitID = 1;

  /** Publish one complete tiled-layout snapshot through the workspace socket. */
  function persistTiledLayout() {
    send({
      type: "tileLayout",
      tileLayout: { tree: tileTree, floated: floatedTileIds },
    });
  }

  function reportTiledPtyResize(id: number, cols: number, rows: number) {
    if (workspaceMode !== "tiled") return;
    windows = windows.map((windowState) =>
      windowState.id === id ? { ...windowState, cols, rows } : windowState,
    );
    // Never broadcast a PTY resize while Ctrl-dragging: the user may pause over
    // a pane, and any server state broadcast would disrupt that active gesture.
    pendingTiledPtySizes = { ...pendingTiledPtySizes, [id]: { cols, rows } };
    if (!tiledReorderDrag) {
      window.clearTimeout(tiledPtyFlushTimer);
      tiledPtyFlushTimer = window.setTimeout(flushTiledPtySizes, 180);
    }
  }

  function flushTiledPtySizes() {
    window.clearTimeout(tiledPtyFlushTimer);
    if (workspaceMode !== "tiled") {
      pendingTiledPtySizes = {};
      return;
    }
    for (const [id, { cols, rows }] of Object.entries(pendingTiledPtySizes)) {
      send({ type: "patch", id: Number(id), patch: { cols, rows } });
    }
    pendingTiledPtySizes = {};
  }

  function tileLeaves(node: TileNode | null): number[] {
    if (!node) return [];
    return "windowId" in node
      ? [node.windowId]
      : [...tileLeaves(node.first), ...tileLeaves(node.second)];
  }

  function pruneTileTree(
    node: TileNode | null,
    valid: Set<number>,
  ): TileNode | null {
    if (!node) return null;
    if ("windowId" in node) return valid.has(node.windowId) ? node : null;
    const first = pruneTileTree(node.first, valid);
    const second = pruneTileTree(node.second, valid);
    if (!first) return second;
    if (!second) return first;
    return { ...node, first, second };
  }

  function syncTileTree(ids: number[]) {
    const valid = new Set(ids);
    // Windows floated out of the grid are not part of the tiled tree.
    for (const floated of floatedTileIds) valid.delete(floated);
    const currentLeaves = tileLeaves(tileTree);
    // Compare against the tiled IDs, not all workspace IDs: floated overlays are
    // intentionally absent from the tree. The old comparison caused needless
    // reconciliation/publish attempts whenever a pane was floated.
    if (
      currentLeaves.length === valid.size &&
      currentLeaves.every((id) => valid.has(id))
    )
      return;
    let next = pruneTileTree(tileTree, valid);
    const present = new Set(tileLeaves(next));
    for (const id of ids) {
      if (present.has(id)) continue;
      const leaf: TileNode = { id: `pane-${id}`, windowId: id };
      if (!next) {
        next = leaf;
      } else {
        // Alternate the outer split direction for each new pane. This produces
        // a flexible binary layout instead of a fixed 2×2 grid.
        const axis: TileAxis =
          tileLeaves(next).length % 2 === 1 ? "vertical" : "horizontal";
        next = {
          id: `split-${nextTileSplitID++}`,
          axis,
          ratio: 0.5,
          first: next,
          second: leaf,
        };
      }
      present.add(id);
    }
    // Update viewport synchronously so displayWindows can use the correct dimensions.
    if (workspaceMode === "tiled" && fabricEl) {
      const rect = fabricEl.getBoundingClientRect();
      tiledViewport = { width: rect.width, height: rect.height };
    }
    tileTree = next;
    // Publish the reconciled tree so the room sees the same tiled arrangement.
    persistTiledLayout();
    // Apply the new layout to fit terminals to their panes.
    void applyTiledLayout();
    // Default the tiled focus to the first pane if nothing focused or the old
    // focus drifted out of the tree.
    const leaves = tileLeaves(next);
    if (tiledFocusId === -1 || !leaves.includes(tiledFocusId)) {
      tiledFocusId = leaves[0] ?? -1;
      if (tiledFocusId !== -1) focusedWindowID = tiledFocusId;
    }
  }

  function tiledWindows(
    source: WindowState[],
    viewport: { width: number; height: number },
    tree: TileNode | null,
  ) {
    if (!source.length || !viewport.width || !viewport.height || !tree)
      return source;
    const topInset = 72;
    const gap = 10; // Gap between tiled windows
    const byID = new Map(
      source.map((windowState) => [windowState.id, windowState]),
    );

    const result: WindowState[] = [];

    function visit(
      node: TileNode,
      x: number,
      y: number,
      width: number,
      height: number,
    ) {
      if ("windowId" in node) {
        const windowState = byID.get(node.windowId);
        // Apply gap by shrinking the window and offsetting position
        if (windowState)
          result.push({
            ...windowState,
            x: x + gap / 2,
            y: y + gap / 2,
            width: Math.max(1, width - gap),
            height: Math.max(1, height - gap),
            zIndex: result.length + 1,
          });
        return;
      }
      if (node.axis === "vertical") {
        const firstWidth = Math.round(width * node.ratio);
        visit(node.first, x, y, firstWidth, height);
        visit(node.second, x + firstWidth, y, width - firstWidth, height);
      } else {
        const firstHeight = Math.round(height * node.ratio);
        visit(node.first, x, y, width, firstHeight);
        visit(node.second, x, y + firstHeight, width, height - firstHeight);
      }
    }

    visit(
      tree,
      0,
      topInset,
      viewport.width,
      Math.max(1, viewport.height - topInset),
    );
    return result;
  }

  async function applyTiledLayout() {
    if (!fabricEl) return;
    const rect = fabricEl.getBoundingClientRect();
    tiledViewport = { width: rect.width, height: rect.height };
    // Tiled panes use the canonical camera. Entering it is animated by the
    // mode switch so the floating canvas doesn't snap underneath the panes.
    await tick();
    // Fit locally after the derived geometry reaches the DOM. Do not patch
    // shared geometry or PTY sizes: tiling is this browser's presentation mode.
    for (const windowState of displayWindows) {
      if (windowState.kind === "shell") termRefs[windowState.id]?.fitSize();
    }
  }

  function setWorkspaceMode(nextMode: "floating" | "tiled") {
    if (nextMode === workspaceMode) return;
    const wasFloating = workspaceMode === "floating";
    workspaceMode = nextMode;
    // Remember the toggle locally so a reload returns to the same view.
    localStorage.setItem("sshx-workspace-mode", nextMode);
    // Leaving tiled discards the tiled-only presentation state (floated-out
    // overlays) — the shared window geometry is intact.
    if (nextMode === "floating") {
      floatedTileIds = [];
      ctrlModifierHeld = false;
      // Floated panes return to the shared grid; publish so the room's layout
      // reflects that they are no longer floated out.
      persistTiledLayout();
    }
    layoutAnimating = true;
    window.clearTimeout(layoutAnimationTimer);
    layoutAnimationTimer = window.setTimeout(
      () => (layoutAnimating = false),
      480,
    );
    if (nextMode === "tiled") {
      // Remember the free-canvas camera exactly as the user left it. Tiled mode
      // is only a local presentation layer and must never discard this state.
      floatingCamera = { x: viewportX, y: viewportY, zoom };
      // The viewport and tree must exist before the first tiled render. Running
      // this synchronously prevents the first toggle from rendering floating
      // geometry and only laying out on the second click.
      if (fabricEl) {
        const rect = fabricEl.getBoundingClientRect();
        tiledViewport = { width: rect.width, height: rect.height };
      }
      syncTileTree(windows.map((windowState) => windowState.id));
      animateViewTo(0, 0, 1, viewportX, viewportY, zoom, 480);
      void applyTiledLayout();
    } else if (floatingCamera) {
      // Return to the exact pan and magnification the floating workspace had
      // before tiling, rather than forcing the user back to a 100% overview.
      animateViewTo(
        floatingCamera.x,
        floatingCamera.y,
        floatingCamera.zoom,
        viewportX,
        viewportY,
        zoom,
        480,
      );
    } else if (wasFloating) {
      layoutAnimating = false;
    }
  }

  function focusWindow(id: number) {
    focusedWindowID = id;
    if (workspaceMode === "tiled") {
      // Tiled focus is a grid focus: no z-order change, just track it. Mirrors
      // Hyprland's focusState — a single focus target steered by both keyboard
      // (movefocus) and mouse (click/pointer over).
      tiledFocusId = id;
      if (isFloatedTile(id)) focusFloatedOverlay(id, true);
      // When the focused pane isn't tiled (floated out), full-screen applies to it.
      return;
    }
    if (workspaceMode === "floating")
      send({ type: "patch", id, patch: { zIndex: ++topZ } });
  }

  function closeWindow(id: number) {
    if (focusedWindowID === id) focusedWindowID = -1;
    if (tiledFocusId === id) tiledFocusId = -1;
    floatedTileIds = floatedTileIds.filter((n) => n !== id);
    send({ type: "close", id });
    persistTiledLayout();
  }

  // ---------------------------------------------------------------------------
  // Tiled-mode window management (Hyprland-inspired focus + drag model)
  // ---------------------------------------------------------------------------

  function isFloatedTile(id: number) {
    return floatedTileIds.includes(id);
  }

  /** Focus an overlay window that was floated out of the grid. */
  function focusFloatedOverlay(id: number, raise = false) {
    if (!raise) return;
    // Floating overlays are stacked by zIndex on the local canvas.
    const win = windowById(id);
    if (win) send({ type: "patch", id, patch: { zIndex: ++topZ } });
  }

  /** Return the window id of the currently focused tiled pane. */
  function activeTiledPane(): number {
    return tiledFocusId;
  }

  function tiledActionTable(): ActionTable {
    return {
      focus_left: () => moveTiledFocus("left"),
      focus_right: () => moveTiledFocus("right"),
      focus_up: () => moveTiledFocus("up"),
      focus_down: () => moveTiledFocus("down"),
      swap_left: () => swapTiled("left"),
      swap_right: () => swapTiled("right"),
      swap_up: () => swapTiled("up"),
      swap_down: () => swapTiled("down"),
      toggle_split: () => toggleTiledSplit(),
      close: () => {
        const p = activeTiledPane();
        if (p >= 0) closeWindow(p);
      },
      cycle_focus_next: () => moveTiledFocus("right"),
    };
  }

  function moveTiledFocus(dir: "left" | "right" | "up" | "down") {
    const current = activeTiledPane();
    if (current < 0 || !tileTree) return;
    const id = neighborPane(tileTree, current, dir);
    if (id == null) return;
    tiledFocusId = id;
    focusedWindowID = id;
    if (isFloatedTile(id)) focusFloatedOverlay(id, true);
    // Defer focus so the DOM reflects the new focused state before we
    // hand the real input focus to the target terminal/editor element.
    requestAnimationFrame(() => {
      const paneEl = document.querySelector(
        `[data-tile-pane="${id}"]`,
      ) as HTMLElement | null;
      if (paneEl) {
        // Find the terminal textarea or the CodeMirror editor inside this pane
        // and give it real input focus so the cursor lands on the content.
        const textarea = paneEl.querySelector(
          "textarea",
        ) as HTMLTextAreaElement | null;
        if (textarea) {
          textarea.focus();
        } else {
          // Fallback: focus the pane itself so keyboard events reach it.
          paneEl.focus();
        }
      }
    });
  }

  function swapTiled(dir: "left" | "right" | "up" | "down") {
    if (!tileTree) return;
    const current = activeTiledPane();
    if (current < 0) return;
    const next = moveWindowDirection(tileTree, current, dir);
    if (next !== tileTree) {
      tileTree = next;
      persistTiledLayout();
    }
    tiledFocusId = current;
    focusedWindowID = current;
  }

  function toggleTiledSplit() {
    if (!tileTree) return;
    const current = activeTiledPane();
    if (current < 0) {
      tileTree = toggleSplitAxis(tileTree, leafOrder(tileTree)[0] ?? current);
      persistTiledLayout();
      return;
    }
    tileTree = toggleSplitAxis(tileTree, current);
    if (tileTree) syncTileTree(tileLeaves(tileTree));
    persistTiledLayout();
  }

  function startMove(id: number, event: PointerEvent) {
    // Window title bars initiate normal floating drags only. Tiled reordering is
    // deliberately Ctrl+drag from anywhere inside a pane (see document handlers).
    if (workspaceMode === "tiled") return;
    event.preventDefault();
    focusWindow(id);
    const win = windowById(id);
    if (!win) return;
    movingWindowID = id;
    movingStart = [event.clientX, event.clientY];
    movingOrigin = [win.x, win.y];
  }

  function beginTiledReorder(id: number, event: PointerEvent) {
    const pane = displayWindows.find((windowState) => windowState.id === id);
    if (!pane) return;
    tiledReorderDrag = {
      windowId: id,
      width: pane.width,
      height: pane.height,
      originX: pane.x,
      originY: pane.y,
      downX: event.clientX,
      downY: event.clientY,
      ghostX: pane.x,
      ghostY: pane.y,
      lastTarget: null,
      targets: displayWindows
        .filter((windowState) => windowState.id !== id)
        .map(({ id: windowId, x, y, width, height }) => ({
          windowId,
          x,
          y,
          width,
          height,
        })),
    };
    tiledReorderPotential = null;
  }

  function moveTiledReorder(event: PointerEvent) {
    if (tiledReorderPotential && !tiledReorderDrag) {
      const { downX, downY, windowId } = tiledReorderPotential;
      if (
        Math.hypot(event.clientX - downX, event.clientY - downY) >
        TILED_DRAG_THRESHOLD
      ) {
        beginTiledReorder(windowId, event);
      }
    }
    if (!tiledReorderDrag || !fabricEl) return;
    const drag = tiledReorderDrag;
    const ghostX = Math.round(
      drag.originX + (event.clientX - drag.downX) / zoom,
    );
    const ghostY = Math.round(
      drag.originY + (event.clientY - drag.downY) / zoom,
    );
    const rect = fabricEl.getBoundingClientRect();
    const worldX = (event.clientX - rect.left - viewportX) / zoom;
    const worldY = (event.clientY - rect.top - viewportY) / zoom;
    const target =
      drag.targets.find(
        ({ x, y, width, height }) =>
          worldX >= x &&
          worldX <= x + width &&
          worldY >= y &&
          worldY <= y + height,
      )?.windowId ?? null;

    // Swap immediately when crossing a pane. Updating the shared tree only on
    // drop avoids broadcasting an intermediate layout on every pointer frame.
    if (target && target !== drag.lastTarget && tileTree) {
      tileTree = swapLeaves(tileTree, drag.windowId, target);
    }
    tiledReorderDrag = { ...drag, ghostX, ghostY, lastTarget: target };
  }

  function finishTiledReorder() {
    if (!tiledReorderDrag) {
      tiledReorderPotential = null;
      return;
    }
    // Deliberately publish once, at pointer release. This keeps drag frames
    // local while guaranteeing every completed Ctrl-drag commits its final tree.
    tiledReorderDrag = null;
    tiledReorderPotential = null;
    persistTiledLayout();
    void applyTiledLayout();
    // The final tree and PTY grid updates are both committed only after the
    // pointer gesture has ended, so no server state broadcast can interrupt it.
    flushTiledPtySizes();
  }

  function handleTiledReorderPointerDown(event: PointerEvent) {
    if (
      workspaceMode !== "tiled" ||
      !ctrlModifierHeld ||
      event.button !== 0 ||
      tiledReorderDrag
    )
      return;
    const target = event.target as HTMLElement | null;
    const pane = target?.closest("[data-tile-pane]") as HTMLElement | null;
    if (!pane) return;
    const windowId = Number(pane.dataset.tilePane);
    if (!Number.isInteger(windowId)) return;
    event.preventDefault();
    event.stopPropagation();
    tiledFocusId = windowId;
    focusedWindowID = windowId;
    tiledReorderPotential = {
      windowId,
      downX: event.clientX,
      downY: event.clientY,
    };
  }

  function handleTiledReorderPointerMove(event: PointerEvent) {
    moveTiledReorder(event);
  }

  function handleTiledReorderPointerUp() {
    finishTiledReorder();
  }

  function startResize(
    id: number,
    event: PointerEvent,
    width: number,
    height: number,
  ) {
    if (workspaceMode === "tiled") return;
    event.preventDefault();
    focusWindow(id);
    resizingWindowID = id;
    resizingStart = [event.clientX, event.clientY];
    resizingOrigin = [width, height];
  }

  function pointerDist(a: [number, number], b: [number, number]) {
    return Math.max(1, Math.hypot(a[0] - b[0], a[1] - b[1]));
  }

  function pointerMid(
    a: [number, number],
    b: [number, number],
  ): [number, number] {
    return [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
  }

  function handlePointerDown(event: PointerEvent) {
    if (event.pointerType === "mouse" && event.button !== 0) return;
    if (drawingMode || workspaceMode === "tiled") return;
    const target = event.target as HTMLElement;
    if (target.closest("[data-no-pan]")) return;
    if (target.closest("[data-pan-surface]") === null) return;
    focusedWindowID = -1;
    surfacePointers.set(event.pointerId, [event.clientX, event.clientY]);
    if (surfacePointers.size === 1) {
      panning = true;
      panStart = [event.clientX, event.clientY];
      panOrigin = [viewportX, viewportY];
      tapCandidate = {
        id: event.pointerId,
        x: event.clientX,
        y: event.clientY,
        time: Date.now(),
        moved: false,
      };
    } else if (surfacePointers.size === 2) {
      // A second finger on the canvas promotes the pan to a pinch zoom.
      panning = false;
      tapCandidate = null;
      const [a, b] = [...surfacePointers.values()];
      pinch = {
        zoom0: zoom,
        viewport0: [viewportX, viewportY],
        mid0: pointerMid(a, b),
        dist0: pointerDist(a, b),
      };
    }
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
    if (event && target?.hasPointerCapture(event.pointerId))
      target.releasePointerCapture(event.pointerId);
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
    if (
      !drawingMode ||
      event.defaultPrevented ||
      event.metaKey ||
      event.ctrlKey ||
      event.altKey
    )
      return;
    if (event.key === "Escape") {
      drawingMode = false;
      return;
    }
    if (event.key === "[")
      drawingStrokeWidth = Math.max(2, drawingStrokeWidth - 1);
    if (event.key === "]")
      drawingStrokeWidth = Math.min(12, drawingStrokeWidth + 1);
  }

  function handleWheel(event: WheelEvent) {
    if (drawingMode || workspaceMode === "tiled") return;
    event.preventDefault();
    const rect = fabricEl.getBoundingClientRect();
    const mouseX = event.clientX - rect.left;
    const mouseY = event.clientY - rect.top;
    const beforeX = (mouseX - viewportX) / zoom;
    const beforeY = (mouseY - viewportY) / zoom;
    const nextZoom = clampZoom(zoom * Math.exp(-event.deltaY * 0.001));
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

  function handlePointerMove(event: PointerEvent) {
    // Only devices with a persistent cursor broadcast it — a finger has no
    // hover position, so touch moves would just teleport the remote cursor.
    if (event.pointerType !== "touch" && fabricEl) {
      const [x, y] = screenToWorld(event.clientX, event.clientY);
      const cursorStyle = detectCursorStyle(event.clientX, event.clientY);
      send({ type: "cursor", x, y, cursorStyle });
    }

    if (drawingMode) return;

    if (surfacePointers.has(event.pointerId)) {
      surfacePointers.set(event.pointerId, [event.clientX, event.clientY]);

      if (
        tapCandidate &&
        event.pointerId === tapCandidate.id &&
        !tapCandidate.moved
      ) {
        if (
          Math.hypot(
            event.clientX - tapCandidate.x,
            event.clientY - tapCandidate.y,
          ) > 10
        ) {
          tapCandidate.moved = true;
        }
      }

      if (pinch && surfacePointers.size >= 2) {
        const rect = fabricEl.getBoundingClientRect();
        const [a, b] = [...surfacePointers.values()];
        const mid = pointerMid(a, b);
        const nextZoom = clampZoom(
          pinch.zoom0 * (pointerDist(a, b) / pinch.dist0),
        );
        // Keep the world point under the initial pinch midpoint stationary
        // while the midpoint itself follows the fingers.
        const worldX =
          (pinch.mid0[0] - rect.left - pinch.viewport0[0]) / pinch.zoom0;
        const worldY =
          (pinch.mid0[1] - rect.top - pinch.viewport0[1]) / pinch.zoom0;
        zoom = nextZoom;
        viewportX = Math.round(mid[0] - rect.left - worldX * nextZoom);
        viewportY = Math.round(mid[1] - rect.top - worldY * nextZoom);
        return;
      }

      if (panning) {
        viewportX = Math.round(panOrigin[0] + event.clientX - panStart[0]);
        viewportY = Math.round(panOrigin[1] + event.clientY - panStart[1]);
        return;
      }
    }

    if (resizingWindowID !== -1) {
      // Keep windows reachable on phones: never require a minimum larger than
      // the viewport itself.
      const vw = fabricEl?.clientWidth ?? 1024;
      const vh = fabricEl?.clientHeight ?? 768;
      const minWidth = Math.min(
        isShell(resizingWindowID) ? 320 : 520,
        Math.max(240, vw - 48),
      );
      const minHeight = Math.min(
        isShell(resizingWindowID) ? 180 : 360,
        Math.max(200, vh - 48),
      );
      const width = Math.max(
        minWidth,
        Math.round(
          resizingOrigin[0] + (event.clientX - resizingStart[0]) / zoom,
        ),
      );
      const height = Math.max(
        minHeight,
        Math.round(
          resizingOrigin[1] + (event.clientY - resizingStart[1]) / zoom,
        ),
      );
      windows = windows.map((w) => {
        if (w.id !== resizingWindowID) return w;
        const next = { ...w, width, height };
        if (w.kind === "shell") {
          const fitted = termRefs[w.id]?.fitSize();
          if (fitted) {
            next.cols = fitted.cols;
            next.rows = fitted.rows;
          }
        }
        return next;
      });
      return;
    }

    if (movingWindowID !== -1) {
      const x = Math.round(
        movingOrigin[0] + (event.clientX - movingStart[0]) / zoom,
      );
      const y = Math.round(
        movingOrigin[1] + (event.clientY - movingStart[1]) / zoom,
      );
      windows = windows.map((w) =>
        w.id === movingWindowID ? { ...w, x, y } : w,
      );
      return;
    }
  }

  function handlePointerUp(event: PointerEvent) {
    const wasSurfacePointer = surfacePointers.delete(event.pointerId);

    if (pinch && surfacePointers.size < 2) {
      pinch = null;
      if (surfacePointers.size === 1) {
        // A pinch just ended: keep panning seamlessly with the remaining finger.
        const [remaining] = [...surfacePointers.values()];
        panStart = remaining;
        panOrigin = [viewportX, viewportY];
        panning = true;
      }
    }
    if (panning && surfacePointers.size === 0) {
      panning = false;
    }

    // Double-tap on empty canvas toggles between 100% and 200% zoom.
    if (
      wasSurfacePointer &&
      tapCandidate &&
      event.pointerId === tapCandidate.id
    ) {
      const quick = Date.now() - tapCandidate.time < 350;
      if (quick && !tapCandidate.moved) {
        const now = Date.now();
        const isDouble =
          now - lastTap.time < 350 &&
          Math.hypot(event.clientX - lastTap.x, event.clientY - lastTap.y) < 40;
        if (isDouble) {
          lastTap = { time: 0, x: 0, y: 0 };
          handleDoubleTap(event.clientX, event.clientY);
        } else {
          lastTap = { time: now, x: event.clientX, y: event.clientY };
        }
      }
      tapCandidate = null;
    }

    stopMove();
  }

  function handlePointerLeave(event: PointerEvent) {
    // A mouse leaving the window mid-gesture releases everything; touch
    // pointers are implicitly captured and always end with up/cancel instead.
    if (event.pointerType === "mouse") {
      panning = false;
      pinch = null;
      surfacePointers.clear();
      tapCandidate = null;
    }
    stopMove();
  }

  function stopMove() {
    // Note: `panning` is owned by the pointer handlers (a pinch can end with
    // one finger still down, seamlessly continuing the pan), so it is not
    // reset here.
    if (resizingWindowID !== -1) {
      const win = windowById(resizingWindowID);
      if (win) {
        const patch: WindowPatch = { width: win.width, height: win.height };
        if (win.kind === "shell") {
          const fitted = termRefs[win.id]?.fitSize();
          patch.cols = fitted?.cols ?? win.cols;
          patch.rows = fitted?.rows ?? win.rows;
        }
        send({ type: "patch", id: win.id, patch });
      }
      resizingWindowID = -1;
    }
    if (movingWindowID !== -1) {
      const win = windowById(movingWindowID);
      if (win)
        send({ type: "patch", id: win.id, patch: { x: win.x, y: win.y } });
      movingWindowID = -1;
    }
  }

  // iOS Safari fires proprietary gesture events for page pinch-zoom; the
  // workspace handles pinch itself, so suppress the browser's version.
  const preventPageGesture = (event: Event) => event.preventDefault();

  // -------------------------------------------------------------------------
  // Tiled keybind + Ctrl+drag wiring (Hyprland-style key/mouse coordination)
  // -------------------------------------------------------------------------

  /**
   * Capture-phase key handler. In tiled mode it primes the Ctrl modifier state
   * and consumes Chord+tile actions before they reach the focused terminal or
   * editor, mirroring Hyprland grabbing binds above applications.
   */
  function handleTilingKeydown(event: KeyboardEvent) {
    if (event.key === "Control") {
      ctrlModifierHeld = true;
      return;
    }
    if (!ctrlModifierHeld) return;

    // Handle Ctrl+Q in both floating and tiled modes to close focused window
    if (
      (event.key === "q" || event.key === "Q") &&
      workspaceMode === "floating"
    ) {
      if (focusedWindowID >= 0) {
        closeWindow(focusedWindowID);
        event.preventDefault();
        event.stopPropagation();
      }
      return;
    }

    if (workspaceMode !== "tiled") return;

    // Reserve Ctrl+Q / Ctrl+` / direction chords for window management even
    // when a terminal is focused.
    const consumed = handleTilingKey(
      { key: event.key, shift: event.shiftKey },
      true,
      tiledActionTable(),
    );
    if (consumed) {
      event.preventDefault();
      event.stopPropagation();
    }
  }

  function handleTilingKeyup(event: KeyboardEvent) {
    if (event.key === "Control") ctrlModifierHeld = false;
  }

  onMount(() => {
    refreshShapes();
    drawingShapes.observe(refreshShapes);
    // Restore the user's floating/tiled toggle from local storage.
    workspaceMode =
      localStorage.getItem("sshx-workspace-mode") === "tiled"
        ? "tiled"
        : "floating";
    // If the restored mode is tiled, lay out the tiles against the actual
    // viewport once the panel is mounted.
    if (workspaceMode === "tiled") {
      if (fabricEl) {
        const rect = fabricEl.getBoundingClientRect();
        tiledViewport = { width: rect.width, height: rect.height };
      }
      // Set the canvas transform to the tiled-mode defaults (origin at 0,0, zoom 1)
      // just like setWorkspaceMode("tiled") does.
      viewportX = 0;
      viewportY = 0;
      zoom = 1;
      void tick().then(() => void applyTiledLayout());
    }
    document.addEventListener("gesturestart", preventPageGesture);
    document.addEventListener("gesturechange", preventPageGesture);
    document.addEventListener("keydown", handleTilingKeydown, true);
    document.addEventListener("keyup", handleTilingKeyup, true);
    document.addEventListener(
      "pointerdown",
      handleTiledReorderPointerDown,
      true,
    );
    document.addEventListener("pointermove", handleTiledReorderPointerMove);
    document.addEventListener("pointerup", handleTiledReorderPointerUp);
    document.addEventListener("pointercancel", handleTiledReorderPointerUp);
    collabConn = new CollabConnection((status) => {
      collabStatus = status;
      if (status === "auth-failed") {
        makeToast({
          kind: "error",
          message: "Collaborative workspace authentication failed.",
        });
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
    window.clearTimeout(layoutAnimationTimer);
    window.clearTimeout(tiledPtyFlushTimer);
    window.cancelAnimationFrame(activeViewAnimation);
    window.clearInterval(pingTimer);
    window.clearTimeout(reconnectTimer);
    document.removeEventListener("gesturestart", preventPageGesture);
    document.removeEventListener("gesturechange", preventPageGesture);
    document.removeEventListener("keydown", handleTilingKeydown, true);
    document.removeEventListener("keyup", handleTilingKeyup, true);
    document.removeEventListener(
      "pointerdown",
      handleTiledReorderPointerDown,
      true,
    );
    document.removeEventListener("pointermove", handleTiledReorderPointerMove);
    document.removeEventListener("pointerup", handleTiledReorderPointerUp);
    document.removeEventListener("pointercancel", handleTiledReorderPointerUp);
    drawingShapes.unobserve(refreshShapes);
    collabConn?.destroy();
    socket?.close();
  });
</script>

<ToastContainer />
<svelte:window
  on:keydown={handleDrawingKeydown}
  on:resize={() => {
    if (workspaceMode === "tiled") void applyTiledLayout();
  }}
/>
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
  on:pointerdown={handlePointerDown}
  on:pointermove={handlePointerMove}
  on:pointerup={handlePointerUp}
  on:pointercancel={handlePointerUp}
  on:pointerleave={handlePointerLeave}
  on:wheel={handleWheel}
>
  <div class="absolute top-4 left-4 z-[12000]">
    <Toolbar
      {connected}
      hasWriteAccess={true}
      {drawingMode}
      onCreate={createShell}
      onCreateEditor={createEditorWindow}
      onToggleDrawing={() => (drawingMode = !drawingMode)}
      onSettings={() => (settingsOpen = true)}
      onNetworkInfo={() => (showNetworkInfo = !showNetworkInfo)}
    />
  </div>

  <div class="absolute top-4 right-4 z-[10000] flex items-start gap-3">
    <div class="relative">
      <WorkspaceMode
        mode={workspaceMode}
        onChange={(mode) => setWorkspaceMode(mode)}
      />
    </div>
    <div class="flex items-center gap-3 pt-1">
      <!-- The full name list crowds the toolbar on phones; avatars suffice. -->
      <div class="hidden sm:block">
        <NameList users={usersForUI} />
      </div>
      <div class="block sm:hidden">
        <Avatars users={usersForUI} />
      </div>
    </div>
  </div>

  {#if showNetworkInfo}
    <div
      class="absolute top-24 left-4 z-[10000] w-[360px] max-w-[calc(100vw-2rem)]"
    >
      <NetworkInfo
        status={connected ? "connected" : "no-server"}
        {serverLatency}
        {shellLatency}
      />
    </div>
  {/if}

  <div
    data-pan-surface
    class="hypr-surface absolute inset-0"
    class:cursor-crosshair={drawingMode}
    class:tile-layout-animating={layoutAnimating}
  >
    <!-- touch-action: none lets our pointer handlers own one-finger pans and
         two-finger pinches without the browser hijacking the gesture. Windows
         float above this layer, so terminal/editor touch scrolling is unaffected. -->
    <div
      class="absolute cursor-grab inset-0 opacity-20"
      style="touch-action: none; background-image: radial-gradient(circle, #71717a 1px, transparent 1px); background-size: {32 *
        zoom}px {32 *
        zoom}px; background-position: {viewportX}px {viewportY}px;"
    ></div>
    <button
      class="absolute left-4 bottom-4 z-[10000] rounded-full border border-white/10 bg-zinc-900/80 px-3 py-2 sm:py-1 text-xs text-zinc-300"
      data-no-pan
      title="Reset zoom to 100%"
      on:click={resetZoom}>zoom {Math.round(zoom * 100)}%</button
    >

    <div
      class="absolute left-0 top-0 origin-top-left"
      style="transform: translate({viewportX}px, {viewportY}px) scale({zoom}); width: 1px; height: 1px;"
    >
      {#each displayWindows as windowState (windowState.id)}
        {#if windowState.kind === "shell"}
          <WebTerm
            bind:this={termRefs[windowState.id]}
            shell={windowState}
            output={outputs[windowState.id] ?? ""}
            zIndex={windowState.zIndex ?? 1}
            focused={focusedWindowID === windowState.id}
            tiled={workspaceMode === "tiled"}
            {layoutAnimating}
            tilePaneId={workspaceMode === "tiled" ? windowState.id : null}
            onFocus={(id) => focusWindow(id)}
            onBlur={() => {
              if (focusedWindowID === windowState.id) focusedWindowID = -1;
            }}
            onStartMove={(id, event) => startMove(id, event)}
            onStartResize={(id, event, width, height) =>
              startResize(id, event, width, height)}
            onTiledResize={reportTiledPtyResize}
            onInput={(id, data) => send({ type: "input", id, data })}
            onResize={(id, cols, rows, width, height) => {
              windows = windows.map((w) =>
                w.id === id ? { ...w, cols, rows, width, height } : w,
              );
              send({ type: "patch", id, patch: { cols, rows, width, height } });
            }}
            onClose={(id) => closeWindow(id)}
          />
        {:else}
          <EditorWindow
            {windowState}
            zIndex={windowState.zIndex ?? 1}
            focused={focusedWindowID === windowState.id}
            tiled={workspaceMode === "tiled"}
            {layoutAnimating}
            tilePaneId={workspaceMode === "tiled" ? windowState.id : null}
            onFocus={(id) => focusWindow(id)}
            onStartMove={(id, event) => startMove(id, event)}
            onStartResize={(id, event, width, height) =>
              startResize(id, event, width, height)}
            onClose={(id) => closeWindow(id)}
          />
        {/if}
      {/each}

      {#if workspaceMode === "tiled"}
        <!-- Floated-out tiles render as real floating windows above the grid. -->
        {#each floatedOverlays as windowState (windowState.id)}
          {#if windowState.kind === "shell"}
            <WebTerm
              bind:this={termRefs[windowState.id]}
              shell={windowState}
              output={outputs[windowState.id] ?? ""}
              zIndex={windowState.zIndex ?? 1}
              focused={focusedWindowID === windowState.id}
              tiled={false}
              {layoutAnimating}
              tilePaneId={null}
              onFocus={(id) => focusWindow(id)}
              onBlur={() => {
                if (focusedWindowID === windowState.id) focusedWindowID = -1;
              }}
              onStartMove={(id, event) => startMove(id, event)}
              onStartResize={(id, event, width, height) =>
                startResize(id, event, width, height)}
              onInput={(id, data) => send({ type: "input", id, data })}
              onResize={(id, cols, rows, width, height) => {
                windows = windows.map((w) =>
                  w.id === id ? { ...w, cols, rows, width, height } : w,
                );
                send({
                  type: "patch",
                  id,
                  patch: { cols, rows, width, height },
                });
              }}
              onClose={(id) => closeWindow(id)}
            />
          {:else}
            <EditorWindow
              {windowState}
              zIndex={windowState.zIndex ?? 1}
              focused={focusedWindowID === windowState.id}
              tiled={false}
              {layoutAnimating}
              tilePaneId={null}
              onFocus={(id) => focusWindow(id)}
              onStartMove={(id, event) => startMove(id, event)}
              onStartResize={(id, event, width, height) =>
                startResize(id, event, width, height)}
              onClose={(id) => closeWindow(id)}
            />
          {/if}
        {/each}
      {/if}

      {#each otherUsersForUI as [id, user] (id)}
        {#if user.cursor}
          <div
            class="pointer-events-none absolute z-[9999]"
            style="transform: translate({user.cursor[0]}px, {user
              .cursor[1]}px);"
          >
            <LiveCursor {user} />
          </div>
        {/if}
      {/each}

      <svg
        class="pointer-events-none absolute left-0 top-0 z-[9998] h-px w-px overflow-visible"
        aria-label="Collaborative world drawings"
      >
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
    class={drawingMode
      ? "pointer-events-auto fixed inset-0 z-[11000] h-full w-full cursor-crosshair"
      : "pointer-events-none fixed inset-0 z-[11000] h-full w-full"}
    style="touch-action: none;"
    role="application"
    on:pointerdown={startCanvasDrawing}
    on:pointermove={continueCanvasDrawing}
    on:pointerup={finishCanvasDrawing}
    on:pointercancel={finishCanvasDrawing}
    on:pointerleave={finishCanvasDrawing}
    aria-label="Collaborative drawing capture layer"
  ></svg>

  {#if drawingMode}
    <section
      class="fixed bottom-4 left-1/2 z-[12000] w-[min(calc(100vw-2rem),620px)] -translate-x-1/2 rounded-2xl border border-indigo-300/25 bg-zinc-950/95 p-3 shadow-2xl backdrop-blur"
      data-no-pan
      aria-label="Drawing tools"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <span
            class="grid h-8 w-8 place-items-center rounded-xl bg-indigo-500/20 text-indigo-200"
            >✎</span
          >
          <div>
            <p class="text-sm font-semibold text-white">Draw</p>
            <p class="text-[11px] text-zinc-400">Esc returns to pointer</p>
          </div>
        </div>
        <button
          class="rounded-lg bg-zinc-800 px-3 py-2 text-xs font-medium text-zinc-200 transition hover:bg-zinc-700 focus:outline-none focus:ring-2 focus:ring-indigo-400"
          on:click={() => (drawingMode = false)}
          >Done <kbd class="ml-1 text-zinc-500">Esc</kbd></button
        >
      </div>

      <div
        class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-3 border-t border-white/10 pt-3"
      >
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
                ><span
                  class="h-5 w-5 rounded-full border border-black/20"
                  style={`background: ${color.value}`}
                ></span></button
              >
            {/each}
            <label
              class="relative grid h-7 w-7 cursor-pointer place-items-center overflow-hidden rounded-full border border-zinc-600 bg-zinc-800 text-base"
              title="Custom color"
            >
              <span aria-hidden="true">+</span>
              <input
                class="absolute inset-0 h-full w-full cursor-pointer opacity-0"
                type="color"
                bind:value={drawingColor}
                aria-label="Custom drawing color"
              />
            </label>
          </div>
        </fieldset>

        <fieldset class="flex items-center gap-2" aria-label="Stroke width">
          <legend class="sr-only">Stroke width</legend>
          <span class="text-xs font-medium text-zinc-400">Size</span>
          <div class="flex rounded-lg bg-zinc-900 p-1 ring-1 ring-white/10">
            {#each drawingWidths as width (width.value)}
              <button
                type="button"
                class="grid h-7 w-9 place-items-center rounded-md transition hover:bg-zinc-800 focus:outline-none focus:ring-2 focus:ring-indigo-400"
                class:bg-indigo-500={width.value}
                aria-label={`${width.name} stroke`}
                aria-pressed={drawingStrokeWidth === width.value}
                title={width.name}
                on:click={() => (drawingStrokeWidth = width.value)}
              >
                <span
                  class="rounded-full bg-white"
                  style={`width: ${Math.max(5, width.value * 2)}px; height: ${Math.max(5, width.value * 2)}px`}
                ></span>
              </button>
            {/each}
          </div>
        </fieldset>

        <div class="ml-auto flex items-center gap-2">
          <button
            class="rounded-lg px-2.5 py-2 text-xs text-zinc-300 transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!shapes.some((shape) => shape.createdBy === clientID)}
            on:click={undoLastDrawing}
            title="Undo your last stroke">Undo</button
          >
          <button
            class="rounded-lg px-2.5 py-2 text-xs text-zinc-300 transition hover:bg-red-500/15 hover:text-red-200 disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!shapes.some((shape) => shape.createdBy === clientID)}
            on:click={clearMyDrawings}
            title="Remove all of your strokes">Clear mine</button
          >
        </div>
      </div>
    </section>
  {/if}

  {#if authRequired && !authenticated}
    <div
      class="fixed inset-0 z-[20000] grid place-items-center bg-black/40 backdrop-blur-sm"
    >
      <form
        class="panel w-[min(92vw,420px)] p-6"
        on:submit|preventDefault={submitPassword}
      >
        <h2 class="mb-2 text-xl font-medium">Password Required</h2>
        <p class="mb-4 text-sm text-zinc-400">
          Enter the shared password to access this sshit session.
        </p>
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
        <button
          class="mt-2 rounded bg-pink-700 px-4 py-2 font-medium hover:bg-pink-600"
          type="submit"
        >
          Unlock
        </button>
      </form>
    </div>
  {/if}

  <Settings open={settingsOpen} on:close={() => (settingsOpen = false)} />
</main>
