/** Built-in tab context-menu entries used by the workspace. */
export type TabMenuItem = "close" | "closeOthers" | "closeAll";

/** Panels that must never be closable (the workspace has no way to recreate them). */
const NON_CLOSABLE = new Set(["tree", "meta", "doc-courtesy"]);

/** True when the panel tab should show a close button. */
export function isTabClosable(panelId: string): boolean {
  return !NON_CLOSABLE.has(panelId);
}

/** Tab context-menu items: no close at all for the non-closable panels. */
export function tabContextMenuItems(panelId: string): TabMenuItem[] {
  return isTabClosable(panelId) ? ["close", "closeOthers", "closeAll"] : [];
}
