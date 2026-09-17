/** Versioned localStorage persistence for dockview layouts. */

const VERSION = 3;
const PREFIX = "sdt-layout:";

interface StoredLayout {
  version: number;
  layout: unknown;
}

function keyFor(id: string): string {
  return `${PREFIX}${id}`;
}

/** Load a stored layout; returns null when absent, stale or corrupt. */
export function loadLayout(id: string): unknown | null {
  try {
    const raw = localStorage.getItem(keyFor(id));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as StoredLayout;
    if (parsed?.version !== VERSION) {
      localStorage.removeItem(keyFor(id));
      return null;
    }
    return parsed.layout;
  } catch {
    localStorage.removeItem(keyFor(id));
    return null;
  }
}

/** Persist a layout shape (best effort; storage failures are ignored). */
export function saveLayout(id: string, layout: unknown): void {
  try {
    localStorage.setItem(keyFor(id), JSON.stringify({ version: VERSION, layout }));
  } catch {
    // storage full/disabled: layout persistence is non-essential
  }
}

/** Remove a stored layout. */
export function clearLayout(id: string): void {
  try {
    localStorage.removeItem(keyFor(id));
  } catch {
    // ignore
  }
}

/**
 * Discard the stored layout and reload so the workspace rebuilds its default
 * panels. The reload is injectable for tests.
 */
export function resetLayout(id: string, reload: () => void = () => window.location.reload()): void {
  clearLayout(id);
  reload();
}
