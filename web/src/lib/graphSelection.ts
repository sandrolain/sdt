import { adjacency, DIM_NODE_COLOR, linkKey, type GLink } from "./graphModel";

export interface SelectionState {
  selected: string | null;
  hovered: string | null;
}

export const initialSelection: SelectionState = { selected: null, hovered: null };

export type SelectionAction =
  | { type: "select"; id: string | null }
  | { type: "hover"; id: string | null }
  | { type: "clear" };

export function selectionReducer(state: SelectionState, action: SelectionAction): SelectionState {
  switch (action.type) {
    case "select":
      return { ...state, selected: action.id };
    case "hover":
      return { ...state, hovered: action.id };
    case "clear":
      return initialSelection;
  }
}

export interface Highlight {
  /** focus node id (selected, else hovered), null when idle */
  focus: string | null;
  active: boolean;
  selected: Set<string>;
  neighbors: Set<string>;
  /** link keys touching the focus node */
  edges: Set<string>;
  /** nodes within blastDepth hops of the focus */
  blast: Set<string>;
}

/**
 * GitNexus-style priority: selected → direct neighbors → connected edges →
 * blast radius → the rest dimmed. Pure sets so the adapter stays testable.
 */
export function computeHighlight(links: GLink[], sel: SelectionState, blastDepth = 2): Highlight {
  const focus = sel.selected ?? sel.hovered;
  const empty: Highlight = {
    focus,
    active: false,
    selected: new Set(),
    neighbors: new Set(),
    edges: new Set(),
    blast: new Set(),
  };
  if (!focus) return empty;

  const adj = adjacency(links);
  const neighbors = new Set(adj.get(focus) ?? []);
  const blast = new Set<string>();
  let frontier = new Set(neighbors);
  for (let depth = 2; depth <= blastDepth; depth += 1) {
    const next = new Set<string>();
    for (const id of frontier) {
      for (const n of adj.get(id) ?? []) {
        if (n !== focus && !neighbors.has(n) && !blast.has(n)) next.add(n);
      }
    }
    for (const id of next) blast.add(id);
    frontier = next;
  }
  const edges = new Set<string>();
  for (const l of links) {
    const s = endpointId(l.source);
    const t = endpointId(l.target);
    if (s === focus || t === focus)
      edges.add(linkKey(l as { source: unknown; target: unknown; verb: string }));
  }
  const selected = new Set<string>([focus]);
  if (sel.hovered && sel.selected) selected.add(sel.hovered);
  return { focus, active: true, selected, neighbors, edges, blast };
}

export interface NodeVisual {
  alpha: number;
  emphasis: boolean;
}

/** Visual weight for a node under the active highlight. */
export function nodeVisual(id: string, h: Highlight): NodeVisual {
  if (!h.active) return { alpha: 1, emphasis: false };
  if (h.selected.has(id)) return { alpha: 1, emphasis: true };
  if (h.neighbors.has(id)) return { alpha: 0.9, emphasis: true };
  if (h.blast.has(id)) return { alpha: 0.55, emphasis: false };
  return { alpha: 0.18, emphasis: false };
}

/** Visual weight for a link under the active highlight. */
export function linkVisual(
  link: { source: unknown; target: unknown; verb: string },
  h: Highlight,
): { alpha: number } {
  if (!h.active) return { alpha: 0.6 };
  const key = linkKey(link);
  if (h.edges.has(key)) return { alpha: 1 };
  const s = endpointId(link.source);
  const t = endpointId(link.target);
  if (h.selected.has(s ?? "") || h.selected.has(t ?? "")) return { alpha: 1 };
  if (h.neighbors.has(s ?? "") || h.neighbors.has(t ?? "")) return { alpha: 0.5 };
  return { alpha: 0.08 };
}

/** Link stroke width: emphasis links are heavier, dimmed ones thinner. */
export function linkWidthFor(
  link: { source: unknown; target: unknown; verb: string },
  h: Highlight,
): number {
  if (!h.active) return 1;
  return linkVisual(link, h).alpha > 0.5 ? 2.5 : 0.75;
}

export function dimmedColor(): string {
  return DIM_NODE_COLOR;
}

function endpointId(endpoint: unknown): string | null {
  if (typeof endpoint === "string") return endpoint;
  if (typeof endpoint === "object" && endpoint !== null && "id" in endpoint) {
    return String((endpoint as { id: unknown }).id);
  }
  return null;
}
