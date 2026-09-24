import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { BoardFlowNode } from "../lib/boardFlow";
import { cardRoute, cardText } from "../lib/canvas";

/** One JSON Canvas node rendered as a card; file/text nodes open on click. */
export function BoardNodeView({ data }: NodeProps<BoardFlowNode>) {
  const { node, onOpen } = data;
  const route = cardRoute(node);
  const isGroup = node.type === "group";
  const text = cardText(node);
  return (
    <div className={`board-card board-card--${node.type}`} style={{ borderColor: node.color }}>
      <Handle
        type="target"
        position={Position.Left}
        isConnectable={false}
        className="board-card__handle"
      />
      {!isGroup && route && onOpen ? (
        <button type="button" className="board-card__button" onClick={() => onOpen(node)}>
          {text}
        </button>
      ) : (
        <span className="board-card__text">{text}</span>
      )}
      <Handle
        type="source"
        position={Position.Right}
        isConnectable={false}
        className="board-card__handle"
      />
    </div>
  );
}
