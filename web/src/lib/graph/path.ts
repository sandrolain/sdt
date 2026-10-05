/**
 * BFS shortest path over the graph.
 *
 * Ported from the reference demo `main.jsx` `findShortestPath`
 * (context/refs/react/graph-react/src/main.jsx 6-37), narrowed to the string
 * endpoints the SDT adapter emits. Pure; unit-tested.
 */
import type { GraphLinkInput, GraphNodeInput } from "./types";

export interface ShortestPath {
  nodeIds: string[];
  links: GraphLinkInput[];
}

/** Undirected BFS path between two node ids, or null when disconnected. */
export function findShortestPath(
  startId: string,
  endIdTarget: string,
  nodes: GraphNodeInput[],
  links: GraphLinkInput[],
): ShortestPath | null {
  const adjacency = new Map<string, { id: string; link: GraphLinkInput }[]>(
    nodes.map((node) => [node.id, []]),
  );
  for (const link of links) {
    adjacency.get(link.source)?.push({ id: link.target, link });
    adjacency.get(link.target)?.push({ id: link.source, link });
  }

  const previous = new Map<string, { from: string; link: GraphLinkInput } | null>([
    [startId, null],
  ]);
  const queue = [startId];
  for (let cursor = 0; cursor < queue.length && !previous.has(endIdTarget); cursor += 1) {
    const current = queue[cursor];
    for (const neighbor of adjacency.get(current) ?? []) {
      if (previous.has(neighbor.id)) continue;
      previous.set(neighbor.id, { from: current, link: neighbor.link });
      queue.push(neighbor.id);
    }
  }
  if (!previous.has(endIdTarget)) return null;

  const nodeIds = [endIdTarget];
  const linksOut: GraphLinkInput[] = [];
  let current = endIdTarget;
  while (current !== startId) {
    const step = previous.get(current);
    if (!step) return null;
    linksOut.push(step.link);
    current = step.from;
    nodeIds.push(current);
  }
  return { nodeIds: nodeIds.reverse(), links: linksOut.reverse() };
}
