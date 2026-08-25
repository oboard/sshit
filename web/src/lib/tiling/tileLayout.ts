import * as Y from "yjs";
import { ydoc } from "$lib/yjsStore";
import type { TileNode } from "$lib/tiling/tiling";

/**
 * Shared, multi-user tiled layout.
 *
 * Floating windows are already synced to every user over the WebSocket (their
 * x/y/size/z-index travel as `patch`/`move` messages). The tiled arrangement —
 * which windows are tiled, how they are split, their ratios, and any panes
 * floated out of the grid — used to be a purely per-browser presentation layer.
 * That is what this module makes room-wide: the whole room sees the same layout
 * so one person arranging panes updates the layout for everyone.
 *
 * Concurrency is handled by Yjs (the same CRDT used for canvas drawings and
 * shared Markdown documents), and because it rides the existing collab channel,
 * the tiled layout is also persisted server-side alongside collab.json.
 *
 * The tree is stored as a single JSON-serialised value ("tree"), plus the list
 * of floated window ids ("floated"). Every local mutation publishes a complete
 * new tree, so scalar last-writer-wins is enough; there is no need to track
 * fine-grained sub-trees.
 */

const tileLayout = ydoc.getMap<{ tree?: string; floated?: string }>("tile-layout");

/** Marker origin used for local writes so we can ignore our own echo. */
const LOCAL_ORIGIN = "tile-local";

export function getSharedTileTree(): TileNode | null {
  const raw = tileLayout.get("tree");
  if (!raw) return null;
  try {
    return JSON.parse(raw) as TileNode | null;
  } catch {
    return null;
  }
}

export function getSharedFloated(): number[] {
  const raw = tileLayout.get("floated");
  if (!raw) return [];
  try {
    return JSON.parse(raw) as number[];
  } catch {
    return [];
  }
}

/**
 * Store the current tiled-layout snapshot into the shared Yjs document. Tagged
 * with a local origin so observers can ignore their own echo.
 */
export function publishTileLayout(tree: TileNode | null, floated: number[]) {
  Y.transact(
    ydoc,
    () => {
      tileLayout.set("tree", tree ? JSON.stringify(tree) : "");
      tileLayout.set("floated", JSON.stringify(floated));
    },
    LOCAL_ORIGIN,
  );
}

/**
 * Observe remote (and local) changes to the shared tiled layout. The callback
 * is invoked with the latest tree and floated list whenever anything changes
 * that did NOT originate from this client (local writes are already applied
 * synchronously by publishTileLayout).
 */
export function observeTileLayout(cb: (tree: TileNode | null, floated: number[]) => void) {
  return tileLayout.observe((event) => {
    if (event.transaction.origin === LOCAL_ORIGIN) return;
    cb(getSharedTileTree(), getSharedFloated());
  });
}