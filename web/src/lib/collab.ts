import * as Y from "yjs";

import { ydoc } from "$lib/yjsStore";

export type CollabStatus = "disconnected" | "connecting" | "connected" | "auth-failed";

type AwarenessState = {
  anchor?: { kind: "docCursor"; docId: string };
  cursor?: number;
  selection?: [number, number];
  name?: string;
  color?: string;
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
  private manualClose = false;
  private password = "";
  private statusListener: StatusListener;
  private updateHandler: (update: Uint8Array, origin: unknown) => void;
  private awarenessMap = new Map<string, AwarenessState>();
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

  connect(password = "") {
    this.password = password || this.password;
    this.manualClose = false;
    this.statusListener("connecting");
    this.socket?.close();

    const socket = new WebSocket(collabURL());
    socket.binaryType = "arraybuffer";
    this.socket = socket;

    socket.onopen = () => {
      socket.send(JSON.stringify({ type: "auth", password: this.password }));
    };

    socket.onmessage = (event) => {
      if (typeof event.data === "string") {
        try {
          const message = JSON.parse(event.data) as { type?: string; id?: number; clientId?: string; awareness?: AwarenessState };
          if (message.type === "ready") {
            if (!message.id && this.socket?.readyState === WebSocket.OPEN) {
              this.socket.send(Y.encodeStateAsUpdate(ydoc));
            }
            this.statusListener("connected");
          } else if (message.type === "authFailed") {
            this.statusListener("auth-failed");
            socket.close();
          } else if (message.type === "awareness" && message.clientId) {
            this.awarenessMap.set(message.clientId, message.awareness ?? {});
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

    socket.onclose = () => {
      if (this.socket === socket) this.socket = null;
      if (!this.manualClose) {
        this.statusListener("disconnected");
        window.clearTimeout(this.reconnectTimer);
        this.reconnectTimer = window.setTimeout(() => this.connect(), 1000);
      }
    };

    socket.onerror = () => {
      this.statusListener("disconnected");
    };
  }

  setAwareness(state: AwarenessState) {
    const selfKey = "local";
    this.awarenessMap.set(selfKey, state);
    this.notifyAwareness();
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "awareness", awareness: state }));
    }
  }

  getDocCursors(docId: string): { position: number; color: string; name: string; selection?: [number, number] }[] {
    const cursors: { position: number; color: string; name: string; selection?: [number, number] }[] = [];
    for (const [, state] of this.awarenessMap) {
      if (state.anchor?.kind === "docCursor" && state.anchor.docId === docId && state.cursor !== undefined) {
        cursors.push({ position: state.cursor, color: state.color || "#818cf8", name: state.name || "anonymous", selection: state.selection });
      }
    }
    return cursors.sort((a, b) => a.position - b.position);
  }

  clearDocCursor(docId: string) {
    const selfKey = "local";
    const current = this.awarenessMap.get(selfKey);
    if (current?.anchor?.docId === docId) {
      this.awarenessMap.delete(selfKey);
      this.notifyAwareness();
      if (this.socket?.readyState === WebSocket.OPEN) {
        this.socket.send(JSON.stringify({ type: "awareness", awareness: null }));
      }
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

export let collabConnection: CollabConnection | null = null;