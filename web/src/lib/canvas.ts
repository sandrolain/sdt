import { displayTitle } from "./titles";
import {
  normalizeCanvas,
  type CanvasDocument,
  type CanvasEdge,
  type CanvasNode,
} from "./jsoncanvas/document";

/**
 * Board model = the shared JSON Canvas document. `x-layer`/`x-layers` and any
 * unknown field are preserved by `normalizeCanvas` (decision 0025); the board
 * keeps its card helpers below.
 */
export type CanvasNodeType = "text" | "file" | "link" | "group";
export type BoardNode = CanvasNode;
export type BoardEdge = CanvasEdge;
export type BoardModel = CanvasDocument;

export interface BoardBounds {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
  width: number;
  height: number;
}

/**
 * Normalize either an API board response (`{nodes,edges}`) or a canvas
 * response (`{path,canvas:{…}}`) into a board model. Unknown fields survive.
 */
export function normalizeBoard(input: unknown): BoardModel {
  return normalizeCanvas(input);
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

/** Card label: explicit canvas text first, else a formatted file title / id. */
export function cardText(node: BoardNode): string {
  if (node.label) return node.label;
  if (node.text) return node.text;
  if (node.file) return displayTitle({ path: node.file });
  return node.id;
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
