/**
 * Edge-hover tooltip model for the graph surface (B6).
 *
 * Pure: resolves a hovered link's verb and both endpoint labels from the
 * adapted engine graph, so the panel's only logic is unit-testable.
 */
import type { GraphLinkInput } from "./types";

export interface EdgeHoverInfo {
  verb: string;
  fromLabel: string;
  toLabel: string;
  label: string | null;
}

export function edgeHoverInfo(
  link: GraphLinkInput,
  labelOf: (id: string) => string,
): EdgeHoverInfo {
  return {
    verb: String(link.type ?? "—"),
    fromLabel: labelOf(link.source),
    toLabel: labelOf(link.target),
    label: link.label === undefined || link.label === null ? null : String(link.label),
  };
}
