import type { ClusterKey } from "./graphModel";
import type { GraphLayout, GraphMode } from "./graph/types";

export type GraphToolMode = GraphMode;

export interface GraphToolsState {
  mode: GraphMode;
  layout: GraphLayout;
  clusterKey: ClusterKey;
  /** cluster groups to dim; the engine keeps the model complete (B4/B5) */
  hiddenGroups: string[];
  /** relation verbs to dim */
  hiddenRelations: string[];
  /** edge kinds to dim */
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
  hiddenGroups: [],
  hiddenRelations: [],
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
  | { type: "toggleGroup"; value: string }
  | { type: "toggleRelation"; value: string }
  | { type: "toggleKind"; value: string }
  | { type: "clearFilters" }
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
    case "toggleGroup":
      return { ...state, hiddenGroups: toggle(state.hiddenGroups, action.value) };
    case "toggleRelation":
      return { ...state, hiddenRelations: toggle(state.hiddenRelations, action.value) };
    case "toggleKind":
      return { ...state, hiddenKinds: toggle(state.hiddenKinds, action.value) };
    case "clearFilters":
      return { ...state, hiddenGroups: [], hiddenRelations: [], hiddenKinds: [] };
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
