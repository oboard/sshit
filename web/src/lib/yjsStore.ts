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

const initialMarkdown = `# 协作文档\n\n这是一个独立的 Markdown 窗口文档。每个编辑器窗口都有自己的内容，并通过 Yjs 与所有协作者同步。\n\n- 移动或缩放窗口会同步\n- 在绘画模式中画到窗口上的线条会跟着窗口移动\n`;

const initializedDocs = new Set<string>();

export function markdownTextForDoc(docId: string) {
  return ydoc.getText(`markdown:${docId}`);
}

export function ensureDefaultDocument(docId: string) {
  if (initializedDocs.has(docId)) return;
  initializedDocs.add(docId);
  const text = markdownTextForDoc(docId);
  if (text.length === 0) {
    text.insert(0, initialMarkdown);
  }
}
