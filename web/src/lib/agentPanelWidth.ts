/**
 * The agent panel's width, as the reader left it.
 *
 * The panel is a pane a reader works in, not a fixed overlay, so its width is the one
 * preference the panel has. It lives in `localStorage` beside the theme rather than in
 * `.sdt.yaml`: the viewer never writes project files.
 *
 * The stored number is in CSS pixels, which is what the drag measures. The stylesheet still
 * bounds what is painted — the same floor and ceiling, and a share of the viewport on top —
 * so a width kept from a wide window stays a pane on a narrow one.
 */

export const PANEL_WIDTH_KEY = "sdt-agent-panel-width";

/** 24rem, 40rem and 64rem at the app's 15px root — the numbers the stylesheet clamps to. */
export const MIN_PANEL_WIDTH = 360;
export const DEFAULT_PANEL_WIDTH = 600;
export const MAX_PANEL_WIDTH = 960;

/** clampPanelWidth holds a width between the pane's floor and its ceiling. */
export function clampPanelWidth(width: number): number {
  if (!Number.isFinite(width)) return DEFAULT_PANEL_WIDTH;
  return Math.min(MAX_PANEL_WIDTH, Math.max(MIN_PANEL_WIDTH, Math.round(width)));
}

/** readPanelWidth reads the remembered width; anything unusable is the default, not an error. */
export function readPanelWidth(): number {
  const raw = typeof localStorage !== "undefined" ? localStorage.getItem(PANEL_WIDTH_KEY) : null;
  if (raw === null) return DEFAULT_PANEL_WIDTH;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? clampPanelWidth(parsed) : DEFAULT_PANEL_WIDTH;
}

/** writePanelWidth remembers the width, clamped so a corrupt value can never be stored. */
export function writePanelWidth(width: number): void {
  localStorage.setItem(PANEL_WIDTH_KEY, String(clampPanelWidth(width)));
}
