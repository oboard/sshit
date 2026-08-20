/**
 * Map CSS cursor keywords to real macOS cursor PNGs in /cursors.
 */

const cursorFiles: Record<string, string> = {
  default: "normal.png",
  text: "text.png",
  "vertical-text": "text.png",
  pointer: "link.png",
  crosshair: "precision.png",
  move: "move.png",
  grab: "pan.png",
  grabbing: "closehand.png",
  "not-allowed": "unavailable.png",
  "no-drop": "unavailable.png",
  copy: "alternate.png",
  alias: "alternate.png",
  help: "help.png",
  "all-scroll": "move.png",
  "zoom-in": "zoom-in.png",
  "zoom-out": "zoom-out.png",
  cell: "precision.png",
  pencil: "handwriting.png",
  "ew-resize": "horizontal-resize.png",
  "col-resize": "horizontal-resize.png",
  "ns-resize": "vertical-resize.png",
  "row-resize": "vertical-resize.png",
  "nwse-resize": "diagonal-resize-1.png",
  "nesw-resize": "diagonal-resize-2.png",
  "n-resize": "vertical-resize.png",
  "s-resize": "vertical-resize.png",
  "e-resize": "horizontal-resize.png",
  "w-resize": "horizontal-resize.png",
  "nw-resize": "diagonal-resize-1.png",
  "se-resize": "diagonal-resize-1.png",
  "ne-resize": "diagonal-resize-2.png",
  "sw-resize": "diagonal-resize-2.png",
  person: "person.png",
  pin: "pin.png",
};

/** Map a CSS cursor keyword to one of our cursor asset names. */
export function cursorAssetName(style: string | undefined): string {
  if (!style || style === "auto") return "default";
  if (cursorFiles[style]) return style;
  if (style.includes("resize")) return "move";
  if (style.startsWith("zoom")) return style;
  return "default";
}

/** Return the /cursors URL for a cursor shape name. */
export function cursorUrl(shape: string): string {
  return `/cursors/${cursorFiles[shape] ?? cursorFiles.default}`;
}
