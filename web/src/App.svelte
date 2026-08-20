<script lang="ts">
  import "@fontsource-variable/inter";

  import { onDestroy, onMount, tick } from "svelte";

  import Avatars from "$lib/ui/Avatars.svelte";
  import ChooseName from "$lib/ui/ChooseName.svelte";
  import LiveCursor from "$lib/ui/LiveCursor.svelte";
  import NameList from "$lib/ui/NameList.svelte";
  import NetworkInfo from "$lib/ui/NetworkInfo.svelte";
  import Settings from "$lib/ui/Settings.svelte";
  import ToastContainer from "$lib/ui/ToastContainer.svelte";
  import Toolbar from "$lib/ui/Toolbar.svelte";
  import { makeToast } from "$lib/toast";
  import { settings } from "$lib/settings";
  import type { WsUser } from "$lib/protocol";

  import { arrangeNewTerminal } from "./arrange";
  import WebTerm, { type Shell } from "./WebTerm.svelte";

  type ServerUser = {
    id: number;
    name: string;
    x: number;
    y: number;
    cursor: boolean;
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

  let movingShellID = -1;
  let movingStart = [0, 0];
  let movingOrigin = [0, 0];
  let resizingShellID = -1;
  let resizingStart = [0, 0];
  let resizingOrigin = [0, 0];

  $: usersForUI = users.map((user) => [
    user.id,
    {
      name: user.name,
      cursor: user.cursor ? [user.x, user.y] : null,
      focus: null,
      canWrite: true,
    } satisfies WsUser,
  ]) as [number, WsUser][];

  $: otherUsersForUI = usersForUI.filter(([id]) => id !== clientID);
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
    if (message.users) users = message.users;
    if (message.shells) {
      const previousShellCount = shells.length;
      const localShells = new Map(shells.map((shell) => [shell.id, shell]));
      shells = message.shells.map((incoming) => {
        const local = localShells.get(incoming.id);
        if (local && incoming.id === movingShellID) {
          return { ...incoming, x: local.x, y: local.y };
        }
        if (local && incoming.id === resizingShellID) {
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
      outputs = nextOutputs;
      zOrder = zOrder;
      if (previousShellCount === 0 && shells.length === 1) {
        const shell = shells[0];
        requestAnimationFrame(() => moveCanvasTo(shell.x, shell.y, 1));
      }
    }
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
        password = "";
        makeToast({ kind: "success", message: "Connected to sshit." });
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

  function createShell() {
    const existing = shells.map((shell) => ({
      x: shell.x,
      y: shell.y,
      width: shell.width || 760,
      height: shell.height || 420,
    }));
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

  function focusShell(id: number) {
    zOrder[id] = ++topZ;
    zOrder = zOrder;
  }

  function startMove(id: number, event: MouseEvent) {
    event.preventDefault();
    focusShell(id);
    const shell = shells.find((shell) => shell.id === id);
    if (!shell) return;
    movingShellID = id;
    movingStart = [event.clientX, event.clientY];
    movingOrigin = [shell.x, shell.y];
  }

  function startResize(id: number, event: MouseEvent, width: number, height: number) {
    event.preventDefault();
    focusShell(id);
    resizingShellID = id;
    resizingStart = [event.clientX, event.clientY];
    resizingOrigin = [width, height];
  }

  function startPan(event: MouseEvent) {
    if (event.button !== 0) return;
    const target = event.target as HTMLElement;
    if (target.closest("[data-no-pan]")) return;
    if (target.closest("[data-pan-surface]") === null) return;
    panning = true;
    panStart = [event.clientX, event.clientY];
    panOrigin = [viewportX, viewportY];
  }

  function handleWheel(event: WheelEvent) {
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

  function handleMouseMove(event: MouseEvent) {
    if (fabricEl) {
      const [x, y] = screenToWorld(event.clientX, event.clientY);
      send({ type: "cursor", x, y });
    }

    if (panning) {
      viewportX = Math.round(panOrigin[0] + event.clientX - panStart[0]);
      viewportY = Math.round(panOrigin[1] + event.clientY - panStart[1]);
      return;
    }

    if (resizingShellID !== -1) {
      const width = Math.max(320, Math.round(resizingOrigin[0] + (event.clientX - resizingStart[0]) / zoom));
      const height = Math.max(180, Math.round(resizingOrigin[1] + (event.clientY - resizingStart[1]) / zoom));
      shells = shells.map((shell) => {
        if (shell.id !== resizingShellID) return shell;
        const next = { ...shell, width, height };
        const fitted = termRefs[shell.id]?.fitSize();
        if (fitted) {
          next.cols = fitted.cols;
          next.rows = fitted.rows;
        }
        return next;
      });
      return;
    }

    if (movingShellID !== -1) {
      const x = Math.round(movingOrigin[0] + (event.clientX - movingStart[0]) / zoom);
      const y = Math.round(movingOrigin[1] + (event.clientY - movingStart[1]) / zoom);
      shells = shells.map((shell) =>
        shell.id === movingShellID ? { ...shell, x, y } : shell,
      );
    }
  }

  function stopMove() {
    panning = false;
    if (resizingShellID !== -1) {
      const shell = shells.find((shell) => shell.id === resizingShellID);
      if (shell) {
        const fitted = termRefs[shell.id]?.fitSize();
        const cols = fitted?.cols ?? shell.cols;
        const rows = fitted?.rows ?? shell.rows;
        send({ type: "resize", id: shell.id, width: shell.width, height: shell.height, cols, rows });
      }
      resizingShellID = -1;
    }
    if (movingShellID !== -1) {
      const shell = shells.find((shell) => shell.id === movingShellID);
      if (shell) send({ type: "move", id: shell.id, x: shell.x, y: shell.y });
      movingShellID = -1;
    }
  }

  onMount(() => {
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
    socket?.close();
  });
</script>

<ToastContainer />
{#if authenticated}
  <ChooseName />
{/if}

<main
  class="relative h-full w-full overflow-hidden text-zinc-100"
  bind:this={fabricEl}
  class:cursor-grabbing={panning}
  on:mousedown={startPan}
  on:mousemove={handleMouseMove}
  on:mouseup={stopMove}
  on:mouseleave={stopMove}
  on:wheel={handleWheel}
>
  <div class="absolute top-4 left-4 z-[10000]">
    <Toolbar
      {connected}
      hasWriteAccess={true}
      newMessages={false}
      on:create={createShell}
      on:chat={() => makeToast({ kind: "info", message: "Chat is not enabled in sshit yet." })}
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

  <div data-pan-surface class="absolute inset-0 cursor-grab bg-[radial-gradient(circle_at_30%_0%,rgba(192,38,211,0.22),transparent_32rem),radial-gradient(circle_at_80%_20%,rgba(37,99,235,0.2),transparent_28rem),#111111]">
    <div class="absolute inset-0 opacity-20" style="background-image: radial-gradient(circle, #71717a 1px, transparent 1px); background-size: {32 * zoom}px {32 * zoom}px; background-position: {viewportX}px {viewportY}px;" />
    <div class="absolute left-4 bottom-4 z-[10000] rounded-full border border-white/10 bg-zinc-900/80 px-3 py-1 text-xs text-zinc-300">zoom {Math.round(zoom * 100)}%</div>

    <div class="absolute left-0 top-0 origin-top-left" style="transform: translate({viewportX}px, {viewportY}px) scale({zoom}); width: 1px; height: 1px;">
    {#each shells as shell (shell.id)}
      <WebTerm
        bind:this={termRefs[shell.id]}
        {shell}
        output={outputs[shell.id] ?? ""}
        zIndex={zOrder[shell.id] ?? 1}
        on:focus={(event) => focusShell(event.detail.id)}
        on:startMove={(event) => startMove(event.detail.id, event.detail.event)}
        on:startResize={(event) => startResize(event.detail.id, event.detail.event, event.detail.width, event.detail.height)}
        on:input={(event) => send({ type: "input", id: event.detail.id, data: event.detail.data })}
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
    </div>
  </div>

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
