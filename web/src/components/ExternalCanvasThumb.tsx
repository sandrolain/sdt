import { useEffect, useState, type ReactNode } from "react";
import type { CanvasDocument, CanvasNode } from "../lib/jsoncanvas/document";
import { loadCanvas } from "../lib/jsoncanvas/canvasLoader";
import { JsonCanvas } from "./JsonCanvas";
import { nestedPlaceholder, nestedZoomButton } from "./nestedCanvasParts";

/** Placeholder threshold (ported from Charkoal's 0.9). */
const PLACEHOLDER_SCALE = 0.9;

/**
 * An external `.canvas` file/link node: fetch the referenced canvas and render a
 * non-interactive miniature, exactly like an inline `nested-canvas`. Only the
 * current level fetches (lazy); a placeholder while loading / when missing.
 */
export function ExternalCanvasThumb({
  node,
  path,
  boardZoom,
  onEnter,
}: {
  node: CanvasNode;
  path: string;
  boardZoom: number;
  onEnter?: (node: CanvasNode) => void;
}): ReactNode {
  const [child, setChild] = useState<CanvasDocument | null | undefined>(undefined);
  useEffect(() => {
    if (boardZoom < PLACEHOLDER_SCALE) return;
    let alive = true;
    loadCanvas(path).then((doc) => {
      if (alive) setChild(doc && doc.nodes.length > 0 ? doc : null);
    });
    return () => {
      alive = false;
    };
  }, [path, boardZoom]);

  const title = String(node.title ?? path.split("/").pop() ?? "Canvas");
  if (boardZoom < PLACEHOLDER_SCALE)
    return nestedPlaceholder("package_2", title, "Double-click to expand");
  if (child === null) return nestedPlaceholder("error", title, "Missing canvas");
  if (!child) return nestedPlaceholder("hourglass_empty", title, "Loading…");
  return (
    <div className="nested-canvas nested-canvas--mini">
      <div className="nested-canvas__canvas" aria-hidden="true">
        <JsonCanvas data={child} showMinimap={false} />
      </div>
      {nestedZoomButton(node, onEnter)}
    </div>
  );
}
