/**
 * 3D depth (O8, `x-z`): a node's depth is the finite `x-z` when present, else
 * the integer `x-layer` ladder × the layer gap (unchanged default). For the
 * layer-plane controls, a node is keyed by its integer `x-layer` when present,
 * otherwise by the nearest lower `x-layer` plane implied by `x-z`; no plane is
 * created for a lone `x-z`.
 */
import { nodeLayer, type CanvasNode } from "./document";

export function nodeDepth(n: CanvasNode, gap: number): number {
  const z = n["x-z"];
  if (typeof z === "number" && Number.isFinite(z)) return z;
  return nodeLayer(n) * gap;
}

export function planeLayer(n: CanvasNode, gap: number): number {
  const z = n["x-z"];
  if (typeof z !== "number" || !Number.isFinite(z)) return nodeLayer(n);
  const layer = n["x-layer"];
  if (typeof layer === "number" && Number.isFinite(layer)) return layer;
  return Math.floor(z / gap);
}
