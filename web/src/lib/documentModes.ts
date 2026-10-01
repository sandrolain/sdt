/** Document view modes: raw source, rendered HTML, conceptual outline, mermaid, slides. */
export type DocumentMode = "code" | "render" | "map" | "mermaid" | "slides";

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
  { id: "slides", label: "Slides", icon: "slideshow" },
];

/** Material Symbols glyph marking a `.map.md` document across the UI. */
export const MAP_ICON = "account_tree";

/** Material Symbols glyph marking a `.mmd` mermaid document across the UI. */
export const MERMAID_ICON = "polyline";

/** Material Symbols glyph marking a `.slide.md` deck across the UI. */
export const SLIDES_ICON = "slideshow";

/** Per-document flags selecting the mode set. Readable at the call site. */
export interface DocumentShape {
  isMap?: boolean;
  isMermaid?: boolean;
  isSlide?: boolean;
}

/**
 * Modes offered for a document. Mermaid mode exists only for `.mmd` documents
 * (Code + Mermaid), Map mode only for `.map.md` semantic maps, Slides mode only
 * for `.slide.md` decks; every other document gets Code/Render.
 */
export function modesFor(shape: DocumentShape = {}): DocumentModeInfo[] {
  if (shape.isMermaid) return DOCUMENT_MODES.filter((m) => m.id === "code" || m.id === "mermaid");
  if (shape.isSlide) {
    return DOCUMENT_MODES.filter((m) => m.id === "code" || m.id === "render" || m.id === "slides");
  }
  return shape.isMap
    ? DOCUMENT_MODES.filter((m) => m.id !== "mermaid" && m.id !== "slides")
    : DOCUMENT_MODES.filter((m) => m.id !== "map" && m.id !== "mermaid" && m.id !== "slides");
}

export function isDocumentMode(value: string | null | undefined): value is DocumentMode {
  return (
    value === "code" ||
    value === "render" ||
    value === "map" ||
    value === "mermaid" ||
    value === "slides"
  );
}

/** Map documents default to Map mode, mermaid to Mermaid, decks to Slides; otherwise Render. */
export function defaultMode(shape: DocumentShape = {}): DocumentMode {
  if (shape.isMermaid) return "mermaid";
  if (shape.isSlide) return "slides";
  return shape.isMap ? "map" : "render";
}

/** True for `.map.md` paths (the filename convention mirrors server metadata). */
export function isMapPath(path: string): boolean {
  return path.endsWith(".map.md");
}

/** True for `.mmd` standalone mermaid paths (mirrors the server extension). */
export function isMermaidPath(path: string): boolean {
  return path.endsWith(".mmd");
}

/** True for `.slide.md` deck paths (the suffix modifier mirrors the corpus model). */
export function isSlidePath(path: string): boolean {
  return path.endsWith(".slide.md");
}
