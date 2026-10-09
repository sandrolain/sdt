import type { ReactNode } from "react";
import { Icon } from "../lib/icon";
import type { CanvasNode } from "../lib/jsoncanvas/document";

/** The opaque placeholder card shared by every nested-canvas body. */
export function nestedPlaceholder(icon: string, title: string, hint: string): ReactNode {
  return (
    <div className="nested-canvas nested-canvas--placeholder">
      <Icon name={icon} />
      <span className="nested-canvas__title">{title}</span>
      <span className="nested-canvas__hint">{hint}</span>
    </div>
  );
}

/** The "zoom in" affordance that focuses the node before drilling into it. */
export function nestedZoomButton(
  node: CanvasNode,
  onEnter?: (node: CanvasNode) => void,
): ReactNode {
  if (!onEnter) return null;
  const label = String(node.title ?? node.file ?? node.url ?? "canvas");
  return (
    <button
      type="button"
      className="nested-canvas__zoom"
      title="Zoom into canvas"
      aria-label={`Zoom into ${label}`}
      onClick={(e) => {
        e.stopPropagation();
        onEnter(node);
      }}
    >
      <Icon name="zoom_in" />
    </button>
  );
}
