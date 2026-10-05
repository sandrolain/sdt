import type { BoundaryRect } from "../lib/boundaries";
import type { GroupHull } from "../lib/groupHull";
import { METRICS } from "../lib/mapMetrics";

/** The XMindMark boundary accent, one colour for every boundary rectangle. */
const BOUNDARY_COLOR = "#fab387"; // categorical identity (phase 9), same as the hull labels
const SUMMARY_COLOR = "#94e2d5";

export interface MapOverlayProps {
  boundaries: BoundaryRect[];
  summaries: BoundaryRect[];
  hulls: GroupHull[];
}

/**
 * The wrap shapes, drawn behind the nodes in layout coordinates. The overlay is
 * mounted inside the shared canvas world (`renderOverlay`), which already applies
 * the pan/zoom transform, so no viewport is read here — one overlay for
 * boundaries, summaries and group hulls, so the geometry the viewer draws and the
 * geometry the exporter serializes are the same numbers.
 */
export function MapOverlay({ boundaries, summaries, hulls }: MapOverlayProps) {
  return (
    <svg className="mindmap__overlay" aria-hidden="true">
      {hulls.map((hull) => (
        <path
          key={`hull-${hull.id}`}
          className="mindmap__hull"
          d={hull.path}
          stroke={hull.color}
          fill={hull.color}
        >
          <text x={hull.labelX} y={hull.labelY} fill={hull.color} fontSize={12}>
            {hull.id}
          </text>
        </path>
      ))}
      {boundaries.map((rect) => (
        <g key={`b-${rect.id}`}>
          <rect
            className="mindmap__boundary"
            x={rect.x}
            y={rect.y}
            width={rect.width}
            height={rect.height}
            rx={10}
            ry={10}
            stroke={BOUNDARY_COLOR}
          />
          {rect.title && (
            <text
              className="mindmap__shape-label"
              x={rect.x + 8}
              y={rect.y + 15}
              fill={BOUNDARY_COLOR}
              fontSize={12}
            >
              {rect.title}
            </text>
          )}
        </g>
      ))}
      {summaries.map((rect) => (
        <g key={`s-${rect.id}`}>
          <rect
            className="mindmap__summary"
            x={rect.x}
            y={rect.y}
            width={rect.width}
            height={rect.height}
            rx={METRICS.shapePadding}
            ry={METRICS.shapePadding}
            stroke={SUMMARY_COLOR}
          />
          {rect.title && (
            <text
              className="mindmap__shape-label"
              x={rect.x + 8}
              y={rect.y + 15}
              fill={SUMMARY_COLOR}
              fontSize={12}
            >
              {rect.title}
            </text>
          )}
        </g>
      ))}
    </svg>
  );
}
