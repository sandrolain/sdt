/**
 * Pure geometry and numeric helpers for the ported graph engine.
 *
 * `bounds`/`fitDistance` are ported from `KnowledgeGraph.jsx` `_bounds`/`_fitDist`
 * (1063-1086); `clamp`/`smooth`/`escapeSvg` from the reference utilities
 * (79-96). No three.js / DOM dependency, so they unit-test in a plain node env.
 */

export const clamp = (v: number, a: number, b: number): number => Math.min(b, Math.max(a, v));

export function smooth(a: number, b: number, x: number): number {
  const t = clamp((x - a) / (b - a), 0, 1);
  return t * t * (3 - 2 * t);
}

/** Escape the five XML special characters for an SVG attribute/text node. */
export function escapeSvg(value: unknown): string {
  return String(value).replace(
    /[&<>"']/g,
    (char) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&apos;",
      })[char] as string,
  );
}

export interface Bounds {
  cx: number;
  cy: number;
  cz: number;
  r: number;
}

/** Centroid and fit radius of the node positions (reference `_bounds`). */
export function bounds(pos: Float32Array, n: number): Bounds {
  if (n === 0) return { cx: 0, cy: 0, cz: 0, r: 40 };
  let cx = 0;
  let cy = 0;
  let cz = 0;
  for (let i = 0; i < n; i += 1) {
    cx += pos[i * 3];
    cy += pos[i * 3 + 1];
    cz += pos[i * 3 + 2];
  }
  cx /= n;
  cy /= n;
  cz /= n;
  let max2 = 0;
  let sum = 0;
  for (let i = 0; i < n; i += 1) {
    const dx = pos[i * 3] - cx;
    const dy = pos[i * 3 + 1] - cy;
    const dz = pos[i * 3 + 2] - cz;
    const d2 = dx * dx + dy * dy + dz * dz;
    if (d2 > max2) max2 = d2;
    sum += Math.sqrt(d2);
  }
  const mean = sum / n;
  const r = Math.min(Math.sqrt(max2), Math.max(mean * 3.2, 40)) + 12;
  return { cx, cy, cz, r };
}

/** Camera distance that fits radius `r`, blending the 2D/3D fov (reference `_fitDist`). */
export function fitDistance(r: number, fovDeg: number, aspect: number, flat: number): number {
  const half = (fovDeg / 2) * (Math.PI / 180);
  const asp = Math.max(0.2, aspect);
  const d3 = r / Math.sin(half);
  const d2 = r / Math.tan(half);
  return Math.max(60, ((d3 + (d2 - d3) * flat) * 1.08) / Math.min(1, asp));
}
