/**
 * Side panels of the documents workspace. They live in dockview **edge groups**
 * so `collapse()`/`expand()` actually work (plain grid panels are a no-op), and
 * the setup + recovery paths share one spec.
 */
import { kindColor, kindLabel, type EntryFilterKind } from "./kinds";

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
  /** Locked panels cannot be dragged/reordered or used as a drop target. */
  locked?: boolean;
}

/**
 * Thickness kept when an edge group is collapsed. Matches the
 * `themeCatppuccinMochaSpaced` tab strip height (44 px, set through the theme
 * object), so the collapsed rail and the tab strip read as one height.
 */
export const COLLAPSED_SIZE = 44;

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
    id: "meta-info",
    groupId: "edge-meta",
    position: "right",
    component: "meta-info",
    title: "Info",
    initialSize: 260,
    minimumSize: 180,
    maximumSize: 560,
    locked: true,
  },
  {
    id: "meta-sections",
    groupId: "edge-meta",
    position: "right",
    component: "meta-sections",
    title: "Sections",
    initialSize: 260,
    minimumSize: 180,
    maximumSize: 560,
    locked: true,
  },
  {
    id: "meta-links",
    groupId: "edge-meta",
    position: "right",
    component: "meta-links",
    title: "Links",
    initialSize: 260,
    minimumSize: 180,
    maximumSize: 560,
    locked: true,
  },
  {
    id: "meta-related",
    groupId: "edge-meta",
    position: "right",
    component: "meta-related",
    title: "Related",
    initialSize: 260,
    minimumSize: 180,
    maximumSize: 560,
    locked: true,
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
    locked?: boolean;
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
  // A fresh workspace: the Info panel is the default active tab in the right
  // group. Adding panels activates the last one, so re-activate Info — but only
  // when it is new, so a restored layout keeps the user's active tab.
  const fresh = api.getPanel("meta-info") === undefined;
  addPanels(api, SIDE_PANELS);
  if (fresh) {
    const info = api.getPanel("meta-info") as { api?: { setActive?: () => void } } | undefined;
    info?.api?.setActive?.();
  }
}

/**
 * Ensure every panel of a spec list exists inside its edge group. Shared by the
 * documents workspace and the `/wiki` surfaces (graph, board), which mount their
 * own `DockviewReact` and pass their own panel spec.
 */
export function addPanels(api: SidePanelApi, specs: SidePanelSpec[]): void {
  for (const spec of specs) {
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
      locked: spec.locked,
    });
  }
}

/** Minimal dockview tab-group API needed to cluster document tabs by kind. */
export interface TabGroupApi {
  getTabGroupForPanel(options: { groupId: string; panelId: string }): { id: string } | undefined;
  getTabGroups(options: { groupId: string }): readonly { id: string; label: string }[];
  createTabGroup(options: { groupId: string; label?: string; color?: string }): { id: string };
  addPanelToTabGroup(options: { groupId: string; tabGroupId: string; panelId: string }): void;
}

/**
 * Ensure a document panel sits in the tab group labelled for its kind, inside
 * the panel's own group. Idempotent: a panel already in a tab group is left
 * alone, and an existing kind group is reused so one kind shares one chip.
 * Colour comes from `kindColor`, the label from `kindLabel`; the courtesy
 * placeholder is never passed here.
 */
export function ensureKindTabGroup(
  api: TabGroupApi,
  panel: { id: string; group: { id: string } },
  kind: EntryFilterKind,
): void {
  const groupId = panel.group.id;
  if (api.getTabGroupForPanel({ groupId, panelId: panel.id })) return;
  const label = kindLabel(kind);
  const existing = api.getTabGroups({ groupId }).find((g) => g.label === label);
  const tabGroup = existing ?? api.createTabGroup({ groupId, label, color: kindColor(kind) });
  api.addPanelToTabGroup({ groupId, tabGroupId: tabGroup.id, panelId: panel.id });
}

/** Minimal dockview API needed to prune tab groups left without panels. */
export interface TabGroupPruneApi {
  groups: readonly { id: string }[];
  getTabGroups(options: { groupId: string }): readonly { id: string; isEmpty: boolean }[];
  dissolveTabGroup(options: { groupId: string; tabGroupId: string }): void;
}

/**
 * Dissolve every tab group whose last panel has closed, so a closed kind never
 * leaves an orphaned chip behind (dockview keeps an emptied group until it is
 * dissolved explicitly).
 */
export function pruneEmptyTabGroups(api: TabGroupPruneApi): void {
  for (const group of api.groups) {
    for (const tabGroup of api.getTabGroups({ groupId: group.id })) {
      if (tabGroup.isEmpty) api.dissolveTabGroup({ groupId: group.id, tabGroupId: tabGroup.id });
    }
  }
}
