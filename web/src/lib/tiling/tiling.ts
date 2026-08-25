/**
 * Tiled-mode tree operations, modeled after Hyprland's layout abstractions.
 *
 * Operations are pure functions over the reactive `TileNode` tree passed in by
 * App.svelte, so they compose with Svelte's reactivity (a new tree triggers a
 * re-layout of panes) without duplicating shared state.
 */

export type TileAxis = "horizontal" | "vertical";

export type TileNode =
  | { id: string; windowId: number }
  | { id: string; axis: TileAxis; ratio: number; first: TileNode; second: TileNode };

export function isLeaf(n: TileNode): n is Extract<TileNode, { windowId: number }> {
  return "windowId" in n;
}

/** Find the leaf wrapping `windowId`. */
export function findPane(tree: TileNode | null, windowId: number): TileNode | null {
  if (!tree) return null;
  if (isLeaf(tree)) return tree.windowId === windowId ? tree : null;
  return findPane(tree.first, windowId) ?? findPane(tree.second, windowId);
}

/** Ordered list of leaf window ids (pre-order). */
export function leafOrder(tree: TileNode | null): number[] {
  if (!tree) return [];
  if (isLeaf(tree)) return [tree.windowId];
  return [...leafOrder(tree.first), ...leafOrder(tree.second)];
}

/**
 * Id of the neighboring leaf in `dir`, by flattened index, wrapping at edges —
 * a web flavor of Hyprland's `movefocus`.
 */
export function neighborPane(
  tree: TileNode | null,
  windowId: number,
  dir: "left" | "right" | "up" | "down",
): number | null {
  const order = leafOrder(tree);
  const idx = order.indexOf(windowId);
  if (idx === -1) return null;
  const n = order.length;
  if (dir === "right" || dir === "down") return order[(idx + 1) % n] ?? null;
  return order[(idx - 1 + n) % n] ?? null;
}

function remapLeaf(tree: TileNode, f: (id: number) => number): TileNode {
  if (isLeaf(tree)) return { ...tree, windowId: f(tree.windowId) };
  return { ...tree, first: remapLeaf(tree.first, f), second: remapLeaf(tree.second, f) };
}

/** Swap the two leaves (by value-swapping their window ids — shape is kept). */
export function swapLeaves(tree: TileNode, aId: number, bId: number): TileNode {
  if (!findPane(tree, aId) || !findPane(tree, bId)) return tree;
  return remapLeaf(tree, id => (id === aId ? bId : id === bId ? aId : id));
}

/** Move focused window in a direction by swapping with its neighbor. */
export function moveWindowDirection(
  tree: TileNode,
  windowId: number,
  dir: "left" | "right" | "up" | "down",
): TileNode {
  const nb = neighborPane(tree, windowId, dir);
  if (nb == null || nb === windowId) return tree;
  return swapLeaves(tree, windowId, nb);
}

/**
 * Flip the split axis of the direct parent split of the pane `windowId`. If the
 * pane sits directly under the root we flip the root. If `windowId` is a split
 * (no pane anchor) it falls back to flipping the root.
 */
export function toggleSplitAxis(tree: TileNode, windowId: number): TileNode {
  const flip = (n: TileNode): TileNode =>
    isLeaf(n) ? n : { ...n, axis: n.axis === "vertical" ? "horizontal" : "vertical" };

  function go(n: TileNode, parent: TileNode | null): TileNode {
    if (isLeaf(n)) {
      if (n.windowId === windowId && parent) return flip(parent);
      return n;
    }
    const first = go(n.first, n);
    const second = go(n.second, n);
    return { ...n, first, second };
  }
  return go(tree, null);
}

/** Replace the window id held by a given pane (leaf by id). */
export function setWindowId(tree: TileNode, leaf: TileNode, windowId: number): TileNode {
  if (isLeaf(leaf)) return remapLeaf(tree, id => (id === leaf.windowId ? windowId : id));
  return tree;
}

/** Rebuild the leaf for a specific window with a new geometry hint, used when
 *  a pane is floated out (we detach it from the tiled tree). */
export function removeLeaf(tree: TileNode, windowId: number): TileNode | null {
  function go(n: TileNode): TileNode | null {
    if (isLeaf(n)) return n.windowId === windowId ? null : n;
    const first = go(n.first);
    const second = go(n.second);
    if (!first) return second;
    if (!second) return first;
    if (first === n.first && second === n.second) return n;
    return { ...n, first, second };
  }
  return go(tree);
}