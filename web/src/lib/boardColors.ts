/**
 * Board colour decoration (O3): resolve cluster → accent and edge kind → tone
 * client-side from the shared palette owner (`graphModel.ts`), so the Go emitter
 * only carries identity strings (`x-cluster`, `x-kind`) and the categorical
 * identity rule (phase 9) stays in one place.
 */
import type { BoardModel, BoardNode, BoardEdge } from "./canvas";
import { clusterColorMap, DEFAULT_NODE_COLOR } from "./graphModel";

// Edge-kind tones: categorical identity (phase 9), raw Catppuccin accents —
// a frontmatter relation vs a typed body link.
const KIND_COLORS: Record<string, string> = {
  relation: "#74c7ec", // sapphire
  link: "#a6e3a1", // green
};

// A cluster group box keeps a muted identity tone, not a theme role.
const GROUP_COLOR = DEFAULT_NODE_COLOR;

function clusterId(node: BoardNode): string {
  return String(node["x-cluster"] ?? "").trim();
}

/** Clone the board (recursively) adding a resolved `color` to nodes and edges. */
export function decorateBoardColours(board: BoardModel): BoardModel {
  const palette = clusterColorMap(board.nodes.map(clusterId).filter(Boolean));
  const nodes: BoardNode[] = board.nodes.map((n) => {
    const out: BoardNode = { ...n };
    const c = clusterId(n);
    if (n.type === "group") {
      out.color = GROUP_COLOR;
    } else if (c && palette.has(c)) {
      out.color = palette.get(c);
    }
    if (n.type === "nested-canvas" && n.canvas) {
      out.canvas = decorateBoardColours(n.canvas);
    }
    return out;
  });
  const edges: BoardEdge[] = board.edges.map((e) => {
    const tone = KIND_COLORS[String(e["x-kind"] ?? "")];
    return tone ? { ...e, color: tone } : e;
  });
  return { ...board, nodes, edges };
}
