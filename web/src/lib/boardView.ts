import type { BoardBounds } from "./canvas";

export interface Transform {
  x: number;
  y: number;
  scale: number;
}

export const IDENTITY: Transform = { x: 0, y: 0, scale: 1 };

export const MIN_SCALE = 0.1;
export const MAX_SCALE = 4;

export interface Viewport {
  width: number;
  height: number;
}

export function clampScale(scale: number): number {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale));
}

/** Fit the board bounds into the viewport with padding, centered. */
export function fitTransform(bounds: BoardBounds, viewport: Viewport, padding = 40): Transform {
  const availW = Math.max(1, viewport.width - padding * 2);
  const availH = Math.max(1, viewport.height - padding * 2);
  const scale = clampScale(
    Math.min(
      bounds.width > 0 ? availW / bounds.width : 1,
      bounds.height > 0 ? availH / bounds.height : 1,
      1,
    ),
  );
  const x = padding + (availW - bounds.width * scale) / 2 - bounds.minX * scale;
  const y = padding + (availH - bounds.height * scale) / 2 - bounds.minY * scale;
  return { x, y, scale };
}

/** Zoom around a point (screen coords) keeping it stationary. */
export function zoomAt(t: Transform, factor: number, px: number, py: number): Transform {
  const scale = clampScale(t.scale * factor);
  const ratio = scale / t.scale;
  return { scale, x: px - (px - t.x) * ratio, y: py - (py - t.y) * ratio };
}

/** Pan by a screen-space delta. */
export function panBy(t: Transform, dx: number, dy: number): Transform {
  return { ...t, x: t.x + dx, y: t.y + dy };
}

/** World (board) coords → screen coords. */
export function worldToScreen(t: Transform, x: number, y: number): { x: number; y: number } {
  return { x: x * t.scale + t.x, y: y * t.scale + t.y };
}

/** Screen coords → world (board) coords. */
export function screenToWorld(t: Transform, x: number, y: number): { x: number; y: number } {
  return { x: (x - t.x) / t.scale, y: (y - t.y) / t.scale };
}

/** CSS transform string for the board layer. */
export function transformStyle(t: Transform): string {
  return `translate(${t.x}px, ${t.y}px) scale(${t.scale})`;
}
