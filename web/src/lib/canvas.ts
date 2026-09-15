export type CanvasNodeType = "text" | "file" | "link" | "group";

export interface BoardNode {
  id: string;
  type: CanvasNodeType;
  x: number;
  y: number;
  width: number;
  height: number;
  color?: string;
  text?: string;
  file?: string;
  url?: string;
  label?: string;
}

export interface BoardEdge {
  id: string;
  fromNode: string;
  fromSide?: string;
  toNode: string;
  toSide?: string;
  color?: string;
  label?: string;
}

export interface BoardModel {
  nodes: BoardNode[];
  edges: BoardEdge[];
}

export interface BoardBounds {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
  width: number;
  height: number;
}

const DEFAULT_WIDTH = 220;
const DEFAULT_HEIGHT = 110;

function num(v: unknown, fallback: number): number {
  return typeof v === "number" && Number.isFinite(v) ? v : fallback;
}

function obj(v: unknown): Record<string, unknown> {
  return typeof v === "object" && v !== null ? (v as Record<string, unknown>) : {};
}

/**
 * Normalize either an API board response (`{nodes,edges}`) or a canvas
 * response (`{path,canvas:{…}}`) into a board model with numeric geometry.
 */
export function normalizeBoard(input: unknown): BoardModel {
  const root = obj(input);
  const source = "canvas" in root ? obj(root.canvas) : root;
  const rawNodes = Array.isArray(source.nodes) ? source.nodes : [];
  const rawEdges = Array.isArray(source.edges) ? source.edges : [];

  const nodes: BoardNode[] = rawNodes.map((n, i) => {
    const node = obj(n);
    const type = ["text", "file", "link", "group"].includes(String(node.type))
      ? (node.type as CanvasNodeType)
      : "text";
    return {
      id: String(node.id ?? i),
      type,
      x: num(node.x, 0),
      y: num(node.y, 0),
      width: num(node.width, DEFAULT_WIDTH),
      height: num(node.height, DEFAULT_HEIGHT),
      color: typeof node.color === "string" ? node.color : undefined,
      text: typeof node.text === "string" ? node.text : undefined,
      file: typeof node.file === "string" ? node.file : undefined,
      url: typeof node.url === "string" ? node.url : undefined,
      label: typeof node.label === "string" ? node.label : undefined,
    };
  });

  const edges: BoardEdge[] = rawEdges.map((e, i) => {
    const edge = obj(e);
    return {
      id: String(edge.id ?? `e${i}`),
      fromNode: String(edge.fromNode ?? ""),
      fromSide: typeof edge.fromSide === "string" ? edge.fromSide : undefined,
      toNode: String(edge.toNode ?? ""),
      toSide: typeof edge.toSide === "string" ? edge.toSide : undefined,
      color: typeof edge.color === "string" ? edge.color : undefined,
      label: typeof edge.label === "string" ? edge.label : undefined,
    };
  });

  return { nodes, edges };
}

/** Bounding box of the board nodes (zero-size when empty). */
export function boardBounds(model: BoardModel): BoardBounds {
  if (model.nodes.length === 0) {
    return { minX: 0, minY: 0, maxX: 0, maxY: 0, width: 0, height: 0 };
  }
  const xs = model.nodes.flatMap((n) => [n.x, n.x + n.width]);
  const ys = model.nodes.flatMap((n) => [n.y, n.y + n.height]);
  const minX = Math.min(...xs);
  const minY = Math.min(...ys);
  const maxX = Math.max(...xs);
  const maxY = Math.max(...ys);
  return { minX, minY, maxX, maxY, width: maxX - minX, height: maxY - minY };
}

/** Center point of a node, for edge routing. */
export function nodeCenter(node: BoardNode): { x: number; y: number } {
  return { x: node.x + node.width / 2, y: node.y + node.height / 2 };
}

/** Resolve the navigation target for a board card. */
export function cardRoute(node: BoardNode): string | null {
  if (node.file) {
    const path = node.file.startsWith("context/") ? node.file : `context/${node.file}`;
    return `/docs/${path}`;
  }
  if (node.type === "text" && node.id) return `/wiki/${node.id}`;
  if (node.url) return node.url;
  return null;
}
