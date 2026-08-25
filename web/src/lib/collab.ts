import * as Y from "yjs";

import { ydoc } from "$lib/yjsStore";

export type CollabStatus = "disconnected" | "connecting" | "connected" | "auth-failed";

type AwarenessState = {
  anchor?: { kind: "docCursor"; docId: string };
  cursor?: number;
  selection?: [number, number];
};

type RemoteAwarenessState = AwarenessState & {
  name: string;
  color: string;
};

type StatusListener = (status: CollabStatus) => void;
type AwarenessListener = (states: AwarenessState[]) => void;

function collabURL() {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/collab`;
}

// We used a module-level variable to share a single collab connection.
// For now we keep the class, but also expose a getter for awareness data.
let activeInstance: CollabConnection | null = null;

export function getActiveCollab() {
  return activeInstance;
}

export class CollabConnection {
  private socket: WebSocket | null = null;
  private reconnectTimer: number | undefined;
  private reconnectAttempt = 0;
  private manualClose = false;
  private password = "";
  private name = "";
  private userID = 0;
  private clientID = "";
  private statusListener: StatusListener;
  private updateHandler: (update: Uint8Array, origin: unknown) => void;
  private awarenessMap = new Map<string, RemoteAwarenessState>();
  private awarenessListeners = new Set<AwarenessListener>();

  constructor(statusListener: StatusListener) {
    this.statusListener = statusListener;
    activeInstance = this;
    this.updateHandler = (update, origin) => {
      if (origin === this || this.socket?.readyState !== WebSocket.OPEN) return;
      this.socket.send(update);
    };
    ydoc.on("update", this.updateHandler);
  }

  connect(password = "", name = "", userID = 0) {
    this.password = password || this.password;
    this.name = name || this.name;
    this.userID = userID || this.userID;
    this.manualClose = false;
    window.clearTimeout(this.reconnectTimer);
    this.reconnectTimer = undefined;
    this.statusListener("connecting");
    this.socket?.close();

    const socket = new WebSocket(collabURL());
    socket.binaryType = "arraybuffer";
    this.socket = socket;

    socket.onopen = () => {
      socket.send(JSON.stringify({ type: "auth", password: this.password, name: this.name, userId: this.userID }));
    };

    socket.onmessage = (event) => {
      if (typeof event.data === "string") {
        try {
          const message = JSON.parse(event.data) as {
            type?: string;
            id?: number;
            clientId?: string;
            name?: string;
            color?: string;
            awareness?: AwarenessState | null;
          };
          if (message.type === "ready") {
            this.clientID = message.clientId ?? "";
            this.reconnectAttempt = 0;
            window.clearTimeout(this.reconnectTimer);
            this.reconnectTimer = undefined;
            this.statusListener("connected");
          } else if (message.type === "authFailed") {
            this.statusListener("auth-failed");
            socket.close();
          } else if (message.type === "awareness" && message.clientId && message.clientId !== this.clientID) {
            if (message.awareness) {
              this.awarenessMap.set(message.clientId, {
                ...message.awareness,
                name: message.name || "anonymous",
                color: message.color || "#818cf8",
              });
            } else {
              this.awarenessMap.delete(message.clientId);
            }
            this.notifyAwareness();
          }
        } catch (error) {
          console.warn("invalid collab control message", error);
        }
        return;
      }

      const update = new Uint8Array(event.data as ArrayBuffer);
      Y.applyUpdate(ydoc, update, this);
    };

    socket.onclose = (event) => {
      // `connect()` intentionally closes the previous socket. Ignore that
      // socket's close event: only the active socket may schedule a reconnect.
      if (this.socket !== socket) return;
      this.socket = null;
      if (this.manualClose || event.code === 1008) return;

      this.statusListener("disconnected");
      window.clearTimeout(this.reconnectTimer);
      // Exponential backoff stops a failing server connection from hammering
      // `/collab`; a successful `ready` resets this counter.
      const delay = Math.min(10_000, 1_000 * 2 ** this.reconnectAttempt++);
      console.warn(`collab socket closed (code ${event.code}${event.reason ? `: ${event.reason}` : ""}); retrying in ${delay}ms`);
      this.reconnectTimer = window.setTimeout(() => this.connect(), delay);
    };

    socket.onerror = () => {
      this.statusListener("disconnected");
    };
  }

  setAwareness(state: AwarenessState) {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "awareness", awareness: state }));
    }
  }

  getDocCursors(docId: string): { clientId: string; position: number; color: string; name: string; selection?: [number, number] }[] {
    const cursors: { clientId: string; position: number; color: string; name: string; selection?: [number, number] }[] = [];
    for (const [clientId, state] of this.awarenessMap) {
      if (state.anchor?.kind === "docCursor" && state.anchor.docId === docId && state.cursor !== undefined) {
        cursors.push({ clientId, position: state.cursor, color: state.color || "#818cf8", name: state.name || "anonymous", selection: state.selection });
      }
    }
    return cursors.sort((a, b) => a.position - b.position);
  }

  clearDocCursor(_docId: string) {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "awareness", awareness: null }));
    }
  }

  onAwareness(fn: AwarenessListener) {
    this.awarenessListeners.add(fn);
    return () => this.awarenessListeners.delete(fn);
  }

  destroy() {
    activeInstance = null;
    this.manualClose = true;
    window.clearTimeout(this.reconnectTimer);
    ydoc.off("update", this.updateHandler);
    this.socket?.close();
    this.socket = null;
  }

  private notifyAwareness() {
    const states = Array.from(this.awarenessMap.values());
    for (const listener of this.awarenessListeners) {
      try {
        listener(states);
      } catch {}
    }
  }
}
