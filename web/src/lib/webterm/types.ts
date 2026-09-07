/**
 * Types copied from Gaurav-Gosain/webterm (MIT), limited to the Kitty overlay
 * embedded by sshit. See ./LICENSE for the upstream license.
 */

export interface KittyOptions {
  /** Anchor images to their scrollback row (default) or to the visible viewport. */
  anchor?: "scrollback" | "viewport";
  /** Decoded images to retain before least-recently-used eviction. Default: 128. */
  storageLimit?: number;
  /** z-index for the image overlay. Default: 5. */
  zIndex?: number;
}
