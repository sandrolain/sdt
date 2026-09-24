import { MarkerType, type Edge, type Node } from "@xyflow/react";
import type { BoardModel, BoardNode } from "./canvas";

/** Data carried by every board node in the React Flow graph. */
export interface BoardNodeData extends Record<string, unknown> {
  node: BoardNode;
  onOpen?: (node: BoardNode) => void;
}

export type BoardFlowNode = Node<BoardNodeData>;
export type BoardFlowEdge = Edge;

const EDGE_COLOR = "#6c7086";
const EDGE_LABEL_COLOR = "#9399b2";

/** Map the normalized board model to read-only React Flow nodes. */
export function toFlowNodes(
  model: BoardModel,
  onOpen?: (node: BoardNode) => void,
): BoardFlowNode[] {
  return model.nodes.map((node) => ({
    id: node.id,
    type: node.type,
    position: { x: node.x, y: node.y },
    data: { node, onOpen },
    initialWidth: node.width,
    initialHeight: node.height,
    style: { width: node.width, height: node.height },
    draggable: false,
    connectable: false,
    selectable: true,
    zIndex: node.type === "group" ? 0 : 1,
  }));
}

/** Map the normalized board model to read-only React Flow edges. */
export function toFlowEdges(model: BoardModel): BoardFlowEdge[] {
  return model.edges.map((edge) => ({
    id: edge.id,
    source: edge.fromNode,
    target: edge.toNode,
    type: "default",
    label: edge.label,
    labelStyle: { fill: EDGE_LABEL_COLOR, fontSize: 11 },
    style: { stroke: EDGE_COLOR, strokeWidth: 1.5 },
    markerEnd: { type: MarkerType.ArrowClosed, color: EDGE_COLOR },
  }));
}
