export type EditorWindowState = {
  id: number;
  docId: string;
  kind: "editor";
  x: number;
  y: number;
  width: number;
  height: number;
  zIndex: number;
};
