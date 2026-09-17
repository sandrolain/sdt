/**
 * Side panels of the documents workspace. They live in dockview **edge groups**
 * so `collapse()`/`expand()` actually work (plain grid panels are a no-op), and
 * the setup + recovery paths share one spec.
 */
export type EdgePosition = "left" | "right";

export interface SidePanelSpec {
  /** panel id */
  id: string;
  /** edge group id */
  groupId: string;
  position: EdgePosition;
  component: string;
  title: string;
  initialSize: number;
  minimumSize: number;
  maximumSize: number;
}

/** Thickness kept when an edge group is collapsed. */
export const COLLAPSED_SIZE = 34;

/** Panel id prefix for open documents. */
export const DOC_PANEL_PREFIX = "doc:";
/** Placeholder document panel shown while no document is open. */
export const COURTESY_PANEL_ID = "doc-courtesy";

/** Narrower defaults than round 6 (tree ~210, meta ~260). */
export const SIDE_PANELS: SidePanelSpec[] = [
  {
    id: "tree",
    groupId: "edge-tree",
    position: "left",
    component: "tree",
    title: "Documents",
    initialSize: 210,
    minimumSize: 150,
    maximumSize: 460,
  },
  {
    id: "meta",
    groupId: "edge-meta",
    position: "right",
    component: "meta",
    title: "Info",
    initialSize: 260,
    minimumSize: 180,
    maximumSize: 560,
  },
];

/** Minimal dockview API surface needed to (re)create the edge-group panels. */
export interface SidePanelApi {
  getPanel(id: string): unknown;
  getEdgeGroup(position: EdgePosition): unknown;
  addEdgeGroup(
    position: EdgePosition,
    options: {
      id: string;
      initialSize: number;
      minimumSize: number;
      maximumSize: number;
      collapsedSize: number;
    },
  ): unknown;
  addPanel(options: {
    id: string;
    component: string;
    title: string;
    position: { referenceGroup: string };
  }): unknown;
}

/** Minimal shape of a dockview group needed to track the centre group. */
export interface CenterGroup {
  id: string;
  api: { location: { type: string } };
}

/** Minimal dockview API needed to find or create the centre grid group. */
export interface CenterApi {
  panels: { id: string; group: CenterGroup }[];
  groups: CenterGroup[];
  addGroup(): CenterGroup;
}

/**
 * Return the centre grid group — the one hosting document tabs. With the side
 * panels living in edge groups, panels must reference this group explicitly;
 * referencing an edge-group panel would dock the document tabs into the side
 * panel's own tab bar. Uses the group that already holds a document (or the
 * placeholder) after a layout restore, otherwise any grid group, otherwise it
 * creates one in the centre.
 */
export function ensureCenterGroup(
  api: CenterApi,
  ref: { current: CenterGroup | null },
): CenterGroup {
  if (ref.current && api.groups.includes(ref.current)) return ref.current;
  const hosted = api.panels.find(
    (p) => p.id.startsWith(DOC_PANEL_PREFIX) || p.id === COURTESY_PANEL_ID,
  );
  const grid = api.groups.find((g) => g.api.location.type === "grid");
  const group = hosted?.group ?? grid ?? api.addGroup();
  ref.current = group;
  return group;
}

/**
 * Ensure each side panel exists inside its edge group. Called after restoring a
 * persisted layout so panels closed by an older build always come back, and so
 * the edge group is recreated when an older layout had none.
 */
export function addSidePanels(api: SidePanelApi): void {
  for (const spec of SIDE_PANELS) {
    if (!api.getEdgeGroup(spec.position)) {
      api.addEdgeGroup(spec.position, {
        id: spec.groupId,
        initialSize: spec.initialSize,
        minimumSize: spec.minimumSize,
        maximumSize: spec.maximumSize,
        collapsedSize: COLLAPSED_SIZE,
      });
    }
    if (api.getPanel(spec.id)) continue;
    api.addPanel({
      id: spec.id,
      component: spec.component,
      title: spec.title,
      position: { referenceGroup: spec.groupId },
    });
  }
}
