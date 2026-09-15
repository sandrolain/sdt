import type { ClusterKey } from "./graphModel";
import type { LayoutKind } from "./graphLayout";

export type GraphMode = "2d" | "3d";

export interface GraphToolsState {
  mode: GraphMode;
  layout: LayoutKind;
  clusterKey: ClusterKey;
  /** verbs to show; null = all visible */
  hiddenVerbs: string[];
  /** edge kinds to show; null = all visible */
  hiddenKinds: string[];
  showLabels: boolean;
  /** node focused via the panel's Focus action */
  focusId: string | null;
}

export const initialGraphTools: GraphToolsState = {
  mode: "2d",
  layout: "force",
  clusterKey: "type",
  hiddenVerbs: [],
  hiddenKinds: [],
  showLabels: true,
  focusId: null,
};

export type GraphToolsAction =
  | { type: "mode"; value: GraphMode }
  | { type: "layout"; value: LayoutKind }
  | { type: "clusterKey"; value: ClusterKey }
  | { type: "toggleVerb"; value: string }
  | { type: "toggleKind"; value: string }
  | { type: "labels"; value: boolean }
  | { type: "focus"; value: string | null }
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
    case "labels":
      return { ...state, showLabels: action.value };
    case "focus":
      return { ...state, focusId: action.value };
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
