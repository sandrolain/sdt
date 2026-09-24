/** Document view modes: raw source, rendered HTML, conceptual outline, mermaid. */
export type DocumentMode = "code" | "render" | "map" | "mermaid";

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
  { id: "mermaid", label: "Mermaid", icon: "polyline" },
];

/** Material Symbols glyph marking a `.map.md` document across the UI. */
export const MAP_ICON = "account_tree";

/** Material Symbols glyph marking a `.mmd` mermaid document across the UI. */
export const MERMAID_ICON = "polyline";

/**
 * Modes offered for a document: Mermaid mode exists only for `.mmd` mermaid
 * documents (Code + Mermaid), Map mode only for `.map.md` semantic maps; every
 * other document gets Code/Render.
 */
export function modesFor(isMap: boolean, isMermaid = false): DocumentModeInfo[] {
  if (isMermaid) return DOCUMENT_MODES.filter((m) => m.id === "code" || m.id === "mermaid");
  return isMap
    ? DOCUMENT_MODES.filter((m) => m.id !== "mermaid")
    : DOCUMENT_MODES.filter((m) => m.id !== "map" && m.id !== "mermaid");
}

export function isDocumentMode(value: string | null | undefined): value is DocumentMode {
  return value === "code" || value === "render" || value === "map" || value === "mermaid";
}

/** Map documents default to Map mode, mermaid documents to Mermaid; otherwise Render. */
export function defaultMode(isMap: boolean, isMermaid = false): DocumentMode {
  if (isMermaid) return "mermaid";
  return isMap ? "map" : "render";
}

/** True for `.map.md` paths (the filename convention mirrors server metadata). */
export function isMapPath(path: string): boolean {
  return path.endsWith(".map.md");
}

/** True for `.mmd` standalone mermaid paths (mirrors the server extension). */
export function isMermaidPath(path: string): boolean {
  return path.endsWith(".mmd");
}
