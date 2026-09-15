import { useEffect, useMemo, useRef, useState, type PointerEvent, type WheelEvent } from "react";
import type { BoardModel, BoardNode } from "../lib/canvas";
import { boardBounds, cardRoute, nodeCenter } from "../lib/canvas";
import {
  fitTransform,
  IDENTITY,
  panBy,
  transformStyle,
  zoomAt,
  type Transform,
  type Viewport,
} from "../lib/boardView";
import { displayTitle } from "../lib/titles";

interface BoardViewProps {
  model: BoardModel;
  onOpen?: (node: BoardNode) => void;
}

const PAD = 120;

/** Card label: explicit canvas text first, else a formatted file title / id. */
function cardText(node: BoardNode): string {
  if (node.label) return node.label;
  if (node.text) return node.text;
  if (node.file) return displayTitle({ path: node.file });
  return node.id;
}

/** Read-only pan/zoom JSON Canvas board with relation edges. */
export function BoardView({ model, onOpen }: BoardViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState<Viewport>({ width: 0, height: 0 });
  const [userT, setUserT] = useState<Transform | null>(null);
  const drag = useRef<{ x: number; y: number } | null>(null);

  useEffect(() => {
    const el = containerRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() =>
      setSize({ width: el.clientWidth, height: el.clientHeight }),
    );
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const bounds = useMemo(() => boardBounds(model), [model]);
  const byId = useMemo(() => new Map(model.nodes.map((n) => [n.id, n])), [model]);
  const view = userT ?? (size.width > 0 ? fitTransform(bounds, size) : IDENTITY);

  const fit = () => setUserT(null);

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.target instanceof Element && e.target.closest(".board-card")) return;
    drag.current = { x: e.clientX, y: e.clientY };
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return;
    const dx = e.clientX - drag.current.x;
    const dy = e.clientY - drag.current.y;
    drag.current = { x: e.clientX, y: e.clientY };
    setUserT(panBy(view, dx, dy));
  };

  const onPointerUp = () => {
    drag.current = null;
  };

  const onWheel = (e: WheelEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1;
    setUserT(zoomAt(view, factor, e.clientX - rect.left, e.clientY - rect.top));
  };

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "0") setUserT(null);
    };
    el.addEventListener("keydown", onKey);
    return () => el.removeEventListener("keydown", onKey);
  }, []);

  const svgX = bounds.minX - PAD;
  const svgY = bounds.minY - PAD;

  return (
    <div className="board">
      <div className="board__controls" role="group" aria-label="Board zoom">
        <button
          type="button"
          className="graph-tools__button"
          onClick={() => setUserT(zoomAt(view, 1.2, size.width / 2, size.height / 2))}
        >
          Zoom in
        </button>
        <button
          type="button"
          className="graph-tools__button"
          onClick={() => setUserT(zoomAt(view, 1 / 1.2, size.width / 2, size.height / 2))}
        >
          Zoom out
        </button>
        <button type="button" className="graph-tools__button" onClick={fit}>
          Fit
        </button>
      </div>
      <div
        ref={containerRef}
        className="board__viewport"
        role="application"
        aria-label="Canvas board (read-only)"
        tabIndex={0}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onWheel={onWheel}
      >
        <div
          className="board__layer"
          style={{ transform: transformStyle(view), transformOrigin: "0 0" }}
        >
          <svg
            className="board__edges"
            style={{ left: svgX, top: svgY }}
            width={bounds.width + PAD * 2}
            height={bounds.height + PAD * 2}
            aria-hidden="true"
          >
            <defs>
              <marker
                id="board-arrow"
                viewBox="0 0 10 10"
                refX="9"
                refY="5"
                markerWidth="6"
                markerHeight="6"
                orient="auto-start-reverse"
              >
                <path d="M 0 0 L 10 5 L 0 10 z" fill="#6c7086" />
              </marker>
            </defs>
            {model.edges.map((edge) => {
              const from = byId.get(edge.fromNode);
              const to = byId.get(edge.toNode);
              if (!from || !to) return null;
              const a = nodeCenter(from);
              const b = nodeCenter(to);
              return (
                <g key={edge.id}>
                  <line
                    x1={a.x - svgX}
                    y1={a.y - svgY}
                    x2={b.x - svgX}
                    y2={b.y - svgY}
                    stroke="#6c7086"
                    strokeWidth={1.5}
                    markerEnd="url(#board-arrow)"
                  />
                  {edge.label && (
                    <text
                      x={(a.x + b.x) / 2 - svgX}
                      y={(a.y + b.y) / 2 - svgY}
                      fill="#9399b2"
                      fontSize={11}
                    >
                      {edge.label}
                    </text>
                  )}
                </g>
              );
            })}
          </svg>
          {model.nodes.map((node) => {
            const route = cardRoute(node);
            const isGroup = node.type === "group";
            return (
              <div
                key={node.id}
                className={`board-card board-card--${node.type}`}
                style={{
                  left: node.x,
                  top: node.y,
                  width: node.width,
                  height: node.height,
                  borderColor: node.color,
                }}
              >
                {!isGroup && route && onOpen ? (
                  <button type="button" className="board-card__button" onClick={() => onOpen(node)}>
                    {cardText(node)}
                  </button>
                ) : (
                  <span className="board-card__text">
                    {cardText(node)}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
