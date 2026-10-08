import {
  normalizeCanvas,
  type CanvasDocument,
  type CanvasEdge,
  type CanvasNode,
} from "./jsoncanvas/document";

/**
 * Board model = the shared JSON Canvas document. `x-layer`/`x-layers` and any
 * unknown field are preserved by `normalizeCanvas` (decision 0025).
 */
export type BoardNode = CanvasNode;
export type BoardEdge = CanvasEdge;
export type BoardModel = CanvasDocument;

/**
 * Normalize either an API board response (`{nodes,edges}`) or a canvas
 * response (`{path,canvas:{…}}`) into a board model. Unknown fields survive.
 */
export function normalizeBoard(input: unknown): BoardModel {
  return normalizeCanvas(input);
}
