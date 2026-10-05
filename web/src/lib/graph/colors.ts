/**
 * Palette and colour-map assignment for the ported graph engine.
 *
 * Ported faithfully from `context/refs/react/graph-react/src/KnowledgeGraph.jsx`
 * (DEFAULT_PALETTE 53-56, NEUTRAL 57, DEFAULT_GROUP 58, DEFAULT_BG 59-60,
 * toRGB 98-104, buildColorMaps 111-143). `buildColorMaps` is pure and is the
 * validation entry point for the graph data.
 */
import * as THREE from "three";
import type { GraphLinkInput, GraphNodeInput } from "./types";

// Categorical identity (project rule): a palette entry names an identity, not a
// theme role, so it stays a raw value. The theming phase re-picks it from the
// Catppuccin palette.
export const DEFAULT_PALETTE = [
  "#cba6f7",
  "#89b4fa",
  "#a6e3a1",
  "#f9e2af",
  "#f5c2e7",
  "#fab387",
  "#94e2d5",
  "#89dceb",
  "#b4befe",
  "#f38ba8",
  "#74c7ec",
  "#eba0ac",
];

export const NEUTRAL = "#7f849c";
export const DEFAULT_GROUP = "__default__";
export const DEFAULT_BG =
  "radial-gradient(1200px 800px at 50% 38%, #101a2e 0%, #080d18 58%, #04070d 100%)";

const _col = new THREE.Color();

/** sRGB (0..1) components of a CSS colour, without linear conversion. */
export function toRGB(css: string): [number, number, number] {
  _col.set(css || NEUTRAL);
  const o = _col.getRGB({ r: 0, g: 0, b: 0 }, THREE.SRGBColorSpace);
  return [o.r, o.g, o.b];
}

export interface ColorMaps {
  /** [group, colour] in first-appearance order (for legends). */
  groups: [string, string][];
  /** [relation type, colour] in first-appearance order. */
  relations: [string, string][];
}

/**
 * Assign a colour to every group and relation type. Supplied colours win; the
 * rest come from the palette. Returns pairs in first-appearance order.
 */
export function buildColorMaps(
  nodes: GraphNodeInput[] = [],
  links: GraphLinkInput[] = [],
  groupColors: Record<string, string> = {},
  relationColors: Record<string, string> = {},
  palette: string[] = DEFAULT_PALETTE,
): ColorMaps {
  if (!Array.isArray(nodes) || !Array.isArray(links)) {
    throw new TypeError("nodes e links devono essere array");
  }
  if (!Array.isArray(palette) || palette.length === 0) {
    throw new TypeError("palette deve contenere almeno un colore");
  }
  const groups = new Map<string, string>();
  const relations = new Map<string, string>();
  for (const n of nodes) {
    const g = String(n.group ?? DEFAULT_GROUP);
    if (!groups.has(g)) {
      groups.set(g, groupColors[g] ?? palette[groups.size % palette.length]);
    }
  }
  for (const l of links) {
    if (l.type === undefined || l.type === null) continue;
    const t = String(l.type);
    if (!relations.has(t)) {
      relations.set(
        t,
        relationColors[t] ??
          palette[(palette.length - 1 - relations.size + palette.length * 4) % palette.length],
      );
    }
  }
  return { groups: [...groups], relations: [...relations] };
}
