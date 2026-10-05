/**
 * Node neighbourhood grouping for the graph detail panel (B10).
 *
 * Pure: derives a selected node's incident edges from the adapted engine graph,
 * grouped by verb with in/out direction, so the panel's only logic is
 * unit-testable without the engine.
 */
import type { AdaptedEngineGraph } from "./adapter";

export interface GraphNeighbour {
  id: string;
  label: string;
  direction: "in" | "out";
}

export interface GraphNeighbourGroup {
  verb: string;
  items: GraphNeighbour[];
}

export function graphNeighbours(
  adapted: AdaptedEngineGraph,
  id: string | null,
): GraphNeighbourGroup[] {
  if (!id) return [];
  const labels = new Map(adapted.nodes.map((n) => [n.id, String(n.label ?? n.id)]));
  const byVerb = new Map<string, GraphNeighbour[]>();
  for (const link of adapted.links) {
    const out = link.source === id;
    const inc = link.target === id;
    if (!out && !inc) continue;
    const other = out ? link.target : link.source;
    const verb = String(link.type ?? "—");
    const list = byVerb.get(verb) ?? [];
    list.push({ id: other, label: labels.get(other) ?? other, direction: out ? "out" : "in" });
    byVerb.set(verb, list);
  }
  return [...byVerb.entries()]
    .map(([verb, items]) => ({
      verb,
      items: items.sort((a, b) => a.label.localeCompare(b.label)),
    }))
    .sort((a, b) => a.verb.localeCompare(b.verb));
}
