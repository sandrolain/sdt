/**
 * Default side panels of the documents workspace. Kept in one place so the
 * dockview setup and the recovery path agree on ids, widths and placement.
 */
export interface SidePanelSpec {
  id: string;
  component: string;
  title: string;
  initialWidth: number;
  minimumWidth: number;
  maximumWidth: number;
  position?: { direction: "left" | "right" };
}

/** Narrower defaults than round 6 (tree ~210, meta ~260). */
export const SIDE_PANELS: SidePanelSpec[] = [
  {
    id: "tree",
    component: "tree",
    title: "Tree",
    initialWidth: 210,
    minimumWidth: 150,
    maximumWidth: 460,
  },
  {
    id: "meta",
    component: "meta",
    title: "Metadata",
    initialWidth: 260,
    minimumWidth: 180,
    maximumWidth: 560,
    position: { direction: "right" },
  },
];

/** Minimal dockview API surface needed to (re)add the side panels. */
export interface SidePanelApi {
  getPanel(id: string): unknown;
  addPanel(options: SidePanelSpec): unknown;
}

/**
 * Add any missing side panel. Called after restoring a persisted layout so a
 * tree/meta panel closed by an older build always comes back.
 */
export function addSidePanels(api: SidePanelApi): void {
  for (const spec of SIDE_PANELS) {
    if (api.getPanel(spec.id)) continue;
    api.addPanel(spec);
  }
}
