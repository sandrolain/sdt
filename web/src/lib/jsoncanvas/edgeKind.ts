/**
 * Edge-kind styles (O7, `x-kind`): a closed set of stroke styles for a semantic
 * edge kind, applied on top of the colour resolved by `boardColors`. Unknown
 * kinds degrade to the neutral (unstyled) edge, and an explicit `x-dash` always
 * wins over the kind's dash.
 */
export interface EdgeKindStyle {
  dash?: string;
  width?: number;
}

// Designed extension (no reference implementation): a frontmatter relation is a
// solid, slightly heavier line; a typed body link is dashed.
const EDGE_KIND_STYLES: Record<string, EdgeKindStyle> = {
  relation: { width: 1.6 },
  link: { dash: "5 4" },
};

export function edgeKindStyle(kind: unknown): EdgeKindStyle {
  return EDGE_KIND_STYLES[String(kind ?? "")] ?? {};
}
