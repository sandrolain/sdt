import { useMemo } from "react";
import {
  Background,
  Controls,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type ReactFlowProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { toFlowEdges, toFlowNodes } from "../lib/boardFlow";
import type { BoardModel, BoardNode } from "../lib/canvas";
import { BoardNodeView } from "./BoardNode";

/** Stable node-type registry (module scope: React Flow warns on identity churn). */
const boardNodeTypes = {
  text: BoardNodeView,
  file: BoardNodeView,
  link: BoardNodeView,
  group: BoardNodeView,
};

interface BoardViewProps {
  model: BoardModel;
  onOpen?: (node: BoardNode) => void;
}

/** Zoom controls wired to the React Flow viewport (must sit inside the provider). */
function BoardZoom() {
  const { zoomIn, zoomOut, fitView } = useReactFlow();
  return (
    <div className="board__controls" role="group" aria-label="Board zoom">
      <button type="button" className="graph-tools__button" onClick={() => zoomIn()}>
        Zoom in
      </button>
      <button type="button" className="graph-tools__button" onClick={() => zoomOut()}>
        Zoom out
      </button>
      <button type="button" className="graph-tools__button" onClick={() => fitView()}>
        Fit
      </button>
    </div>
  );
}

const FLOW_PROPS: Partial<ReactFlowProps> = {
  minZoom: 0.1,
  maxZoom: 4,
  nodesDraggable: false,
  nodesConnectable: false,
  elementsSelectable: true,
  panOnDrag: true,
  zoomOnScroll: true,
  fitView: true,
  proOptions: { hideAttribution: true },
};

/** Read-only JSON Canvas board rendered with React Flow. */
export function BoardView({ model, onOpen }: BoardViewProps) {
  const nodes = useMemo(() => toFlowNodes(model, onOpen), [model, onOpen]);
  const edges = useMemo(() => toFlowEdges(model), [model]);
  return (
    <div className="board">
      <ReactFlowProvider>
        <BoardZoom />
        <div className="board__viewport" role="application" aria-label="Canvas board (read-only)">
          <ReactFlow nodes={nodes} edges={edges} nodeTypes={boardNodeTypes} {...FLOW_PROPS}>
            <Background />
            <Controls showInteractive={false} />
          </ReactFlow>
        </div>
      </ReactFlowProvider>
    </div>
  );
}
