import type { ClusterKey } from "./graphModel";
import type { GraphLayout, GraphMode } from "./graph/types";

export type GraphToolMode = GraphMode;

export interface GraphToolsState {
  mode: GraphMode;
  layout: GraphLayout;
  clusterKey: ClusterKey;
  /** verbs to show; null = all visible */
  hiddenVerbs: string[];
  /** edge kinds to show; null = all visible */
  hiddenKinds: string[];
  showLabels: boolean;
  /** node focused via the panel's Focus action */
  focusId: string | null;
  /** highlight the most connected nodes (engine centrality) */
  centrality: boolean;
  /** show only the selected node and its direct neighbours */
  neighborsOnly: boolean;
}

export const initialGraphTools: GraphToolsState = {
  mode: "2d",
  layout: "force",
  clusterKey: "type",
  hiddenVerbs: [],
  hiddenKinds: [],
  showLabels: true,
  focusId: null,
  centrality: false,
  neighborsOnly: false,
};

export type GraphToolsAction =
  | { type: "mode"; value: GraphMode }
  | { type: "layout"; value: GraphLayout }
  | { type: "clusterKey"; value: ClusterKey }
  | { type: "toggleVerb"; value: string }
  | { type: "toggleKind"; value: string }
  | { type: "setHiddenVerbs"; value: string[] }
  | { type: "setHiddenKinds"; value: string[] }
  | { type: "labels"; value: boolean }
  | { type: "focus"; value: string | null }
  | { type: "centrality"; value: boolean }
  | { type: "neighbors"; value: boolean }
  | { type: "reset" };

export function graphToolsReducer(
  state: GraphToolsState,
  action: GraphToolsAction,
): GraphToolsState {
  switch (action.type) {
    case "mode":
      return { ...state, mode: action.value };
    case "layout":
      return { ...state, layout: action.value };
    case "clusterKey":
      return { ...state, clusterKey: action.value };
    case "toggleVerb":
      return { ...state, hiddenVerbs: toggle(state.hiddenVerbs, action.value) };
    case "toggleKind":
      return { ...state, hiddenKinds: toggle(state.hiddenKinds, action.value) };
    case "setHiddenVerbs":
      return { ...state, hiddenVerbs: action.value };
    case "setHiddenKinds":
      return { ...state, hiddenKinds: action.value };
    case "labels":
      return { ...state, showLabels: action.value };
    case "focus":
      return { ...state, focusId: action.value };
    case "centrality":
      return { ...state, centrality: action.value };
    case "neighbors":
      return { ...state, neighborsOnly: action.value };
    case "reset":
      return { ...initialGraphTools };
  }
}

function toggle(list: string[], value: string): string[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

/** Visible verbs given the hidden set, or undefined when nothing is hidden. */
export function visibleSet(all: string[], hidden: string[]): Set<string> | undefined {
  if (hidden.length === 0) return undefined;
  return new Set(all.filter((v) => !hidden.includes(v)));
}
