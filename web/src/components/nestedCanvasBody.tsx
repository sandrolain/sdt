import type { ReactNode } from "react";
import { Icon } from "../lib/icon";
import { nestedCanvas, type CanvasNode } from "../lib/jsoncanvas/document";
import { JsonCanvas } from "./JsonCanvas";

/** Default placeholder threshold (ported from Charkoal's 0.9). */
export const PLACEHOLDER_SCALE = 0.9;

/**
 * A `nested-canvas` body for the board (analysis 20261003-224537): below the
 * placeholder scale (or an empty/missing child) an opaque card with the title,
 * otherwise a nested read-only JsonCanvas miniature scaled to the node rect. The
 * miniature is non-interactive; the outer node keeps the double-click that enters
 * the level. Returns `undefined` for any other node type, so the shared view
 * falls through to its own body.
 */
export function renderNestedBody(node: CanvasNode, boardZoom: number): ReactNode {
  if (node.type !== "nested-canvas") return undefined;
  const title = String(node.title ?? "Nested canvas");
  const child = nestedCanvas(node);
  const empty = !child || child.nodes.length === 0;
  if (empty || boardZoom < PLACEHOLDER_SCALE) {
    return (
      <div className="nested-canvas nested-canvas--placeholder">
        <Icon name="package_2" />
        <span className="nested-canvas__title">{title}</span>
        <span className="nested-canvas__hint">
          {empty ? "Empty canvas" : "Double-click to expand"}
        </span>
      </div>
    );
  }
  return (
    <div className="nested-canvas nested-canvas--mini" aria-hidden="true">
      <JsonCanvas data={child} showMinimap={false} />
    </div>
  );
}
