/**
 * JSON Canvas 1.0 document model, shared by the board and mind-map lanes.
 *
 * Derived from `context/refs/react/jc-vite` (MIT) and the JSON Canvas 1.0 spec.
 * Unknown fields on nodes, edges and the document root are preserved (the
 * guide's `x-*` convention), so a third-party property survives a read-only
 * render. `x-layer`/`x-layers` are the two non-standard properties the view
 * reads and renders.
 */

export type CanvasSide = "top" | "right" | "bottom" | "left";
export type CanvasEnd = "arrow" | "none";

/** A JSON Canvas node with the SDT/Charkoal extensions preserved. */
export interface CanvasNode {
  id: string;
  type: string;
  x: number;
  y: number;
  width: number;
  height: number;
  color?: string;
  text?: string;
  file?: string;
  subpath?: string;
  url?: string;
  label?: string;
  /** Non-standard: layer index for the 3D view (may be negative). */
  "x-layer"?: number;
  /** Charkoal `nested-canvas`: the embedded sub-canvas (inline, one file). */
  canvas?: CanvasDocument;
  /** Charkoal `nested-canvas` label. */
  title?: string;
  [key: string]: unknown;
}

/** A JSON Canvas edge with the SDT extensions preserved. */
export interface CanvasEdge {
  id: string;
  fromNode: string;
  toNode: string;
  fromSide?: CanvasSide;
  toSide?: CanvasSide;
  fromEnd?: CanvasEnd;
  toEnd?: CanvasEnd;
  color?: string;
  label?: string;
  [key: string]: unknown;
}

/** A named 3D layer (`doc["x-layers"]`). */
export interface CanvasLayer {
  id: number | string;
  name: string;
}

/** A JSON Canvas 1.0 document (root fields preserved). */
export interface CanvasDocument {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  /** Non-standard: labels for the `x-layer` levels. */
  "x-layers"?: CanvasLayer[];
  [key: string]: unknown;
}

const DEFAULT_WIDTH = 220;
const DEFAULT_HEIGHT = 110;

function num(v: unknown, fallback: number): number {
  return typeof v === "number" && Number.isFinite(v) ? v : fallback;
}

function obj(v: unknown): Record<string, unknown> {
  return typeof v === "object" && v !== null ? (v as Record<string, unknown>) : {};
}

function readLayers(v: unknown): CanvasLayer[] | undefined {
  if (!Array.isArray(v)) return undefined;
  return v.map((l, i) => {
    const raw = obj(l);
    return {
      id: typeof raw.id === "number" || typeof raw.id === "string" ? raw.id : i,
      name: String(raw.name ?? `Level ${i}`),
    };
  });
}

/**
 * Normalize an API board response (`{nodes,edges}`) or a canvas response
 * (`{path,canvas:{…}}`) into a `CanvasDocument`, preserving unknown fields.
 */
export function normalizeCanvas(input: unknown): CanvasDocument {
  const root = obj(input);
  const source = "canvas" in root ? obj(root.canvas) : root;
  const rawNodes = Array.isArray(source.nodes) ? source.nodes : [];
  const rawEdges = Array.isArray(source.edges) ? source.edges : [];

  const nodes: CanvasNode[] = rawNodes.map((n, i) => {
    const node = obj(n);
    const rawLayer = node["x-layer"];
    const layer =
      typeof rawLayer === "number" && Number.isFinite(rawLayer) ? { "x-layer": rawLayer } : {};
    const type = String(node.type ?? "text");
    // A `nested-canvas` embeds a whole sub-canvas inline; normalize it recursively
    // (missing `canvas` → an empty child, rendered as an empty placeholder).
    const nested = type === "nested-canvas" ? { canvas: normalizeCanvas(node.canvas) } : {};
    return {
      ...node,
      id: String(node.id ?? i),
      type,
      x: num(node.x, 0),
      y: num(node.y, 0),
      width: num(node.width, DEFAULT_WIDTH),
      height: num(node.height, DEFAULT_HEIGHT),
      color: typeof node.color === "string" ? node.color : undefined,
      text: typeof node.text === "string" ? node.text : undefined,
      file: typeof node.file === "string" ? node.file : undefined,
      subpath: typeof node.subpath === "string" ? node.subpath : undefined,
      url: typeof node.url === "string" ? node.url : undefined,
      label: typeof node.label === "string" ? node.label : undefined,
      title: typeof node.title === "string" ? node.title : undefined,
      ...layer,
      ...nested,
    } as CanvasNode;
  });

  const edges: CanvasEdge[] = rawEdges.map((e, i) => {
    const edge = obj(e);
    return {
      ...edge,
      id: String(edge.id ?? `e${i}`),
      fromNode: String(edge.fromNode ?? ""),
      toNode: String(edge.toNode ?? ""),
      fromSide: typeof edge.fromSide === "string" ? (edge.fromSide as CanvasSide) : undefined,
      toSide: typeof edge.toSide === "string" ? (edge.toSide as CanvasSide) : undefined,
      fromEnd: edge.fromEnd === "arrow" || edge.fromEnd === "none" ? edge.fromEnd : undefined,
      toEnd: edge.toEnd === "arrow" || edge.toEnd === "none" ? edge.toEnd : undefined,
      color: typeof edge.color === "string" ? edge.color : undefined,
      label: typeof edge.label === "string" ? edge.label : undefined,
    } as CanvasEdge;
  });

  const layers = readLayers(source["x-layers"]);
  return { nodes, edges, ...(layers ? { "x-layers": layers } : {}) } as CanvasDocument;
}

/** Layer index of a node (`x-layer`, default 0). */
export function nodeLayer(n: CanvasNode): number {
  const v = n["x-layer"];
  return typeof v === "number" && Number.isFinite(v) ? v : 0;
}

/** Named layers of a document: the distinct `x-layer` values, `x-layers` names. */
export interface CanvasLayerInfo {
  id: number;
  name: string;
}

export function canvasLayers(doc: CanvasDocument): CanvasLayerInfo[] {
  const ids = [...new Set(doc.nodes.map(nodeLayer))].sort((a, b) => a - b);
  const names = Object.fromEntries((doc["x-layers"] ?? []).map((l) => [String(l.id), l.name]));
  return ids.map((id) => ({ id, name: names[String(id)] ?? `Level ${id}` }));
}

/** The embedded sub-canvas of a `nested-canvas` node, or null. */
export function nestedCanvas(node: CanvasNode): CanvasDocument | null {
  return node.type === "nested-canvas" ? (node.canvas ?? null) : null;
}

/**
 * The nested-canvas node ids to enter to reach a deep node id, or null when the
 * id is not in the tree. An id on a nested node itself is reached by entering
 * that node, so its path is the ids *above* it (the caller then shows that
 * level); a node in the root document resolves to `[]`.
 */
export function findNodeRecursively(doc: CanvasDocument, id: string): string[] | null {
  for (const node of doc.nodes) {
    if (node.id === id) return [];
    const child = nestedCanvas(node);
    if (child) {
      const sub = findNodeRecursively(child, id);
      if (sub) return [node.id, ...sub];
    }
  }
  return null;
}
