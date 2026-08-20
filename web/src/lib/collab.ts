import * as Y from "yjs";

import { ydoc } from "$lib/yjsStore";

export type CollabStatus = "disconnected" | "connecting" | "connected" | "auth-failed";

type StatusListener = (status: CollabStatus) => void;

function collabURL() {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/collab`;
}

export class CollabConnection {
  private socket: WebSocket | null = null;
  private reconnectTimer: number | undefined;
  private manualClose = false;
  private password = "";
  private statusListener: StatusListener;
  private updateHandler: (update: Uint8Array, origin: unknown) => void;

  constructor(statusListener: StatusListener) {
    this.statusListener = statusListener;
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
          const message = JSON.parse(event.data) as { type?: string; id?: number };
          if (message.type === "ready") {
            if (!message.id && this.socket?.readyState === WebSocket.OPEN) {
              this.socket.send(Y.encodeStateAsUpdate(ydoc));
            }
            this.statusListener("connected");
          } else if (message.type === "authFailed") {
            this.statusListener("auth-failed");
            socket.close();
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

  destroy() {
    this.manualClose = true;
    window.clearTimeout(this.reconnectTimer);
    ydoc.off("update", this.updateHandler);
    this.socket?.close();
    this.socket = null;
  }
}
