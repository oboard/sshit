/**
 * Keybind dispatch for tiled mode, inspired by Hyprland's CBind /
 * CKeybindManager funnel and its drag_threshold CLICK/DRAG disambiguation.
 *
 * Keyboard and mouse/pointer events are funnelled through this one registry so
 * both input paths settle on the same focused pane as their action target.
 * The registry is a plain key ↔ action table; the modifier gate and the
 * click-vs-drag decision are supplied by the caller (App.svelte), mirroring how
 * Hyprland gates binds by modifiers and flag binds CLICK/DRAG.
 */

import type { TilingKeyAction, TilingGesture, ActionTable } from "./tiling";

function isLeftStreet(k: string): boolean {
  return k === "h" || k === "l" || k === "k" || k === "j";
}

/**
 * Resolve a keyboard event against the action table.
 *
 * @param hit   the DOM event key + whether SHIFT is held
 * @param ctrl  whether the Ctrl/tile modifier is currently held
 * @param binds the action table (missing → no-op)
 * @returns     true when a chord matched and was consumed (caller should not
 *              forward the keystroke to the terminal/editor)
 */
export function handleTilingKey(
  hit: { key: string; shift: boolean },
  ctrl: boolean,
  binds: ActionTable,
): boolean {
  if (!ctrl) return false;

  const key = hit.key;
  const arrowOrHjk =
    key === "ArrowLeft" || key === "h"
      ? "left"
      : key === "ArrowRight" || key === "l"
        ? "right"
        : key === "ArrowUp" || key === "k"
          ? "up"
          : key === "ArrowDown" || key === "j"
            ? "down"
            : null;

  // Directional focus/swap under Ctrl.
  if (arrowOrHjk) {
    if (hit.shift) {
      const action = { left: "swap_left", right: "swap_right", up: "swap_up", down: "swap_down" }[arrowOrHjk];
      if (action && binds[action as keyof ActionTable]) {
        binds[action as keyof ActionTable]?.();
        return true;
      }
      return false;
    }
    const action = { left: "focus_left", right: "focus_right", up: "focus_up", down: "focus_down" }[arrowOrHjk];
    if (action && binds[action as keyof ActionTable]) {
      binds[action as keyof ActionTable]?.();
      return true;
    }
    return false;
  }

  // Non-directional chord binds (SHIFT-insensitive).
  const fixed =
    key === "q" || key === "Q" ? "close"
    : key === "f" || key === "F" ? "fullscreen"
    : key.toLowerCase() === "t" ? "toggle_split"
    : key === "`" ? "cycle_focus_next"
    : null;

  if (fixed && binds[fixed]) {
    binds[fixed]?.();
    return true;
  }
  return false;
}

/** Invoke a mouse gesture action (click vs drag). */
export function dispatchMouseGesture(
  gesture: TilingGesture,
  binds: { click?: () => void; drag?: () => void },
) {
  if (gesture === "click") binds.click?.();
  else binds.drag?.();
}