/** Document view modes: raw source, rendered HTML, conceptual outline. */
export type DocumentMode = "code" | "render" | "map";

export interface DocumentModeInfo {
  id: DocumentMode;
  label: string;
  /** Material Symbols glyph name for the mode button */
  icon: string;
}

/** Display order of the mode switch. */
export const DOCUMENT_MODES: DocumentModeInfo[] = [
  { id: "code", label: "Code", icon: "code" },
  { id: "render", label: "Render", icon: "article" },
  { id: "map", label: "Map", icon: "account_tree" },
];

/** Material Symbols glyph marking a `.map.md` document across the UI. */
export const MAP_ICON = "account_tree";

/**
 * Modes offered for a document: Map mode exists only for `.map.md` semantic
 * maps; every other document gets Code/Render.
 */
export function modesFor(isMap: boolean): DocumentModeInfo[] {
  return isMap ? DOCUMENT_MODES : DOCUMENT_MODES.filter((m) => m.id !== "map");
}

export function isDocumentMode(value: string | null | undefined): value is DocumentMode {
  return value === "code" || value === "render" || value === "map";
}

/** Map documents default to Map mode; ordinary markdown defaults to Render. */
export function defaultMode(isMap: boolean): DocumentMode {
  return isMap ? "map" : "render";
}

/** True for `.map.md` paths (the filename convention mirrors server metadata). */
export function isMapPath(path: string): boolean {
  return path.endsWith(".map.md");
}
