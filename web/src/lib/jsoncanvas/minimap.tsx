/**
 * SVG minimap for the shared JSON Canvas view.
 *
 * Adapted from `context/refs/react/jc-vite/src/JsonCanvas.jsx` (MIT): an overview
 * of every node with the current viewport rectangle; a click jumps the view. Pure
 * presentational component — the host owns the view state.
 */
import type { PointerEvent as ReactPointerEvent } from "react";
import type { CanvasNode } from "./document";
import { resolveColor, type CanvasTheme } from "./theme";

export interface CanvasViewState {
  x: number;
  y: number;
  k: number;
}

export interface CanvasSize {
  w: number;
  h: number;
}

interface MinimapProps {
  nodes: CanvasNode[];
  view: CanvasViewState;
  size: CanvasSize;
  theme: CanvasTheme;
  presets?: Record<string, string>;
  onGo: (x: number, y: number) => void;
}

export function Minimap({ nodes, view, size, theme, presets, onGo }: MinimapProps) {
  const vw = { x: -view.x / view.k, y: -view.y / view.k, w: size.w / view.k, h: size.h / view.k };
  const xs = [vw.x, vw.x + vw.w, ...nodes.flatMap((n) => [n.x, n.x + n.width])];
  const ys = [vw.y, vw.y + vw.h, ...nodes.flatMap((n) => [n.y, n.y + n.height])];
  const x0 = Math.min(...xs) - 80;
  const y0 = Math.min(...ys) - 80;
  const W = Math.max(...xs) + 80 - x0;
  const H = Math.max(...ys) + 80 - y0;
  const go = (e: ReactPointerEvent<SVGSVGElement>) => {
    e.stopPropagation();
    const ctm = e.currentTarget.getScreenCTM();
    if (!ctm) return;
    const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(ctm.inverse());
    onGo(p.x, p.y);
  };
  return (
    <svg
      className="jc-mm"
      viewBox={`${x0} ${y0} ${W} ${H}`}
      onPointerDown={go}
      role="presentation"
      aria-hidden="true"
    >
      {nodes.map((n) => (
        <rect
          key={n.id}
          x={n.x}
          y={n.y}
          width={n.width}
          height={n.height}
          rx="8"
          fill={n.type === "group" ? "none" : resolveColor(n.color, presets) || theme.border}
          stroke={n.type === "group" ? resolveColor(n.color, presets) || theme.border : "none"}
          strokeWidth="6"
          opacity={n.type === "group" ? 0.8 : 0.9}
        />
      ))}
      <rect
        x={vw.x}
        y={vw.y}
        width={vw.w}
        height={vw.h}
        fill={theme.accent}
        fillOpacity=".12"
        stroke={theme.accent}
        strokeWidth="2"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
