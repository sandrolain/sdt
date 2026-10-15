/**
 * The docs map's inline guide, as data: one hotspot per sidebar section.
 *
 * A hotspot is anchored to the section it explains rather than to a canvas
 * coordinate, so a control that moves takes its hotspot with it — the coordinate
 * variant ages the moment the sidebar changes, and this surface's sidebar is exactly
 * what the wave before this one reorganised (analysis 20261013-063009, O6, D4).
 */
export type GuideAnchor = "surface" | "filters" | "explore" | "actions";

export interface GuideHotspot {
  anchor: GuideAnchor;
  /** The one line a hover shows; the extended guide is the corpus page below. */
  line: string;
}

/** The guide lives in the corpus, not in the bundle: `fetchDoc` reads it through the
 * viewer's own document endpoint (which takes the corpus-relative path, `context/`
 * included) and the markdown pipeline renders it. */
export const GUIDE_DOC_PATH = "context/wiki/viewer-docs-map-guide.md";

export const GUIDE_HOTSPOTS: GuideHotspot[] = [
  { anchor: "surface", line: "What the map draws, and how it is laid out" },
  { anchor: "filters", line: "What is drawn, and what is dimmed" },
  { anchor: "explore", line: "Follow one document to its semantic neighbours" },
  { anchor: "actions", line: "Fit, clear and export the picture" },
];

/** The line for an anchor, or null when the anchor carries no hotspot. */
export function guideLineFor(anchor: GuideAnchor): string | null {
  return GUIDE_HOTSPOTS.find((h) => h.anchor === anchor)?.line ?? null;
}
