import * as Y from "yjs";

export type DrawingAnchor =
  | { kind: "world" }
  | { kind: "collabWindow"; id: number }
  | { kind: "shell"; id: number };

export type DrawingShape = {
  id: string;
  type: "path";
  anchor: DrawingAnchor;
  x: number;
  y: number;
  color: string;
  strokeWidth: number;
  points: [number, number][];
  createdBy?: number;
};

export type CollabWindowState = {
  id: number;
  docId: string;
  kind: "editor";
  x: number;
  y: number;
  width: number;
  height: number;
  zIndex: number;
};

export const ydoc = new Y.Doc();
export const drawingShapes = ydoc.getMap<DrawingShape>("drawing-shapes");
export const collabWindowMap = ydoc.getMap<CollabWindowState>("collab-windows");

const initializedDocs = new Set<string>();

export function markdownTextForDoc(docId: string) {
  return ydoc.getText(`markdown:${docId}`);
}

export function ensureDefaultDocument(docId: string) {
  if (initializedDocs.has(docId)) return;
  initializedDocs.add(docId);
  const text = markdownTextForDoc(docId);
  if (text.length === 0) {
    text.insert(0, "");
  }
}
