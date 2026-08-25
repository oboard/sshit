type Sid = number; // u32
type Uid = number; // u32

/** A workspace window. Shell and editor windows share one list, told apart by kind. */
export type WindowState = {
  id: number;
  kind: "shell" | "editor";
  x: number;
  y: number;
  width: number;
  height: number;
  zIndex: number;
  /** Shell-only: terminal grid size. */
  cols?: number;
  rows?: number;
  /** Shell-only: replay scrollback sent on hello/state. */
  buffer?: string;
  /** Shell-only: current working directory. */
  cwd?: string;
  /** Shell-only: user home directory for path shortening. */
  home?: string;
  /** Editor-only: collaborative document id. */
  docId?: string;
};

/** Patch applied to a window; absent fields are unchanged. */
export type WindowPatch = {
  x?: number;
  y?: number;
  width?: number;
  height?: number;
  zIndex?: number;
  cols?: number;
  rows?: number;
};

/** Position and size of a window, see the Rust version. */
export type WsWinsize = {
  x: number;
  y: number;
  rows: number;
  cols: number;
};

/** Information about a user, see the Rust version */
export type WsUser = {
  name: string;
  cursor: [number, number] | null;
  focus: number | null;
  canWrite: boolean;
};

/** Server message type, see the Rust version. */
export type WsServer = {
  hello?: [Uid, string];
  invalidAuth?: [];
  users?: [Uid, WsUser][];
  userDiff?: [Uid, WsUser | null];
  shells?: [Sid, WsWinsize][];
  chunks?: [Sid, number, Uint8Array[]];
  hear?: [Uid, string, string];
  shellLatency?: number | bigint;
  pong?: number | bigint;
  error?: string;
};

/** Client message type, see the Rust version. */
export type WsClient = {
  authenticate?: [Uint8Array, Uint8Array | null];
  setName?: string;
  setCursor?: [number, number] | null;
  setFocus?: number | null;
  create?: [number, number];
  close?: Sid;
  move?: [Sid, WsWinsize | null];
  data?: [Sid, Uint8Array, bigint];
  subscribe?: [Sid, number];
  ping?: bigint;
};
