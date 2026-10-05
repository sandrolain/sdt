/**
 * Edge-panel spec shared by the `/wiki` surfaces.
 *
 * The documents workspace keeps its panels in `SIDE_PANELS`; the graph and the
 * board mount their own `DockviewReact` and reuse the same edge-group mechanics
 * through `addPanels` (analysis D4). The two layouts share the helper, never
 * their state.
 */
import { addPanels, type EdgePosition, type SidePanelSpec } from "./workspaceLayout";

export type { EdgePosition };

/** Centre panel id on the graph surface. */
export const GRAPH_CENTER_PANEL_ID = "wiki-graph";
/** Right edge-panel id hosting the graph controls. */
export const GRAPH_CONTROLS_PANEL_ID = "wiki-graph-controls";
/** Centre panel id on the board surface. */
export const BOARD_CENTER_PANEL_ID = "wiki-board";
/** Right edge-panel id hosting the board controls (sibling wave). */
export const BOARD_CONTROLS_PANEL_ID = "wiki-board-controls";

/** Right-docked controls panel for the graph surface. */
export const WIKI_GRAPH_PANELS: SidePanelSpec[] = [
  {
    id: GRAPH_CONTROLS_PANEL_ID,
    groupId: "edge-wiki-graph-controls",
    position: "right",
    component: "graph-controls",
    title: "Graph",
    initialSize: 288,
    minimumSize: 200,
    maximumSize: 520,
  },
];

/** Right-docked controls panel for the board surface. */
export const WIKI_BOARD_PANELS: SidePanelSpec[] = [
  {
    id: BOARD_CONTROLS_PANEL_ID,
    groupId: "edge-wiki-board-controls",
    position: "right",
    component: "board-controls",
    title: "Board",
    initialSize: 288,
    minimumSize: 200,
    maximumSize: 520,
  },
];

/** Ensure the graph controls panel lives in its right edge group. */
export function addGraphPanels(api: Parameters<typeof addPanels>[0]): void {
  addPanels(api, WIKI_GRAPH_PANELS);
}

/** Ensure the board controls panel lives in its right edge group. */
export function addBoardPanels(api: Parameters<typeof addPanels>[0]): void {
  addPanels(api, WIKI_BOARD_PANELS);
}
