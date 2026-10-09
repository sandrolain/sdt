import type { ReactNode } from "react";
import { nestedCanvas, type CanvasNode } from "../lib/jsoncanvas/document";
import { ExternalCanvasThumb } from "./ExternalCanvasThumb";
import { JsonCanvas } from "./JsonCanvas";
import { nestedPlaceholder, nestedZoomButton } from "./nestedCanvasParts";

/** Default placeholder threshold (ported from Charkoal's 0.9). */
export const PLACEHOLDER_SCALE = 0.9;

export interface NestedBodyOptions {
  /** Drill into the node (used by the "zoom in" affordance). */
  onEnter?: (node: CanvasNode) => void;
}

/** The `context/`-prefixed path of an external `.canvas` file/link node, or null. */
export function externalCanvasPath(node: CanvasNode): string | null {
  if (node.type !== "file" && node.type !== "link") return null;
  const raw = node.file ?? node.url ?? "";
  if (!raw.endsWith(".canvas")) return null;
  return raw.startsWith("context/") ? raw : `context/${raw}`;
}

/**
 * A `nested-canvas` body for the board (analysis 20261003-224537): below the
 * placeholder scale (or an empty/missing child) an opaque card with the title,
 * otherwise a nested read-only JsonCanvas miniature scaled to the node rect. An
 * external `.canvas` file/link node gets the same miniature, fetched lazily.
 * Returns `undefined` for any other node type, so the shared view falls through
 * to its own body.
 */
export function renderNestedBody(
  node: CanvasNode,
  boardZoom: number,
  opts: NestedBodyOptions = {},
): ReactNode {
  if (node.type === "nested-canvas") {
    const title = String(node.title ?? "Nested canvas");
    const child = nestedCanvas(node);
    const empty = !child || child.nodes.length === 0;
    if (empty || boardZoom < PLACEHOLDER_SCALE) {
      return nestedPlaceholder(
        "package_2",
        title,
        empty ? "Empty canvas" : "Double-click to expand",
      );
    }
    return (
      <div className="nested-canvas nested-canvas--mini">
        <div className="nested-canvas__canvas" aria-hidden="true">
          <JsonCanvas data={child} showMinimap={false} />
        </div>
        {nestedZoomButton(node, opts.onEnter)}
      </div>
    );
  }
  const path = externalCanvasPath(node);
  if (path) {
    return (
      <ExternalCanvasThumb node={node} path={path} boardZoom={boardZoom} onEnter={opts.onEnter} />
    );
  }
  return undefined;
}
