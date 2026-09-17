/** Stack of open documents (oldest → newest) for the tab rail, persisted. */

import { clearLayout, loadLayout, saveLayout } from "./layoutStore";

export const OPEN_DOCS_CAP = 8;

/** localStorage key suffix for the persisted open-docs state. */
const STORE_ID = "open-docs";

export interface OpenDocsState {
  /** open document paths, oldest first */
  docs: string[];
  /** currently focused document path */
  active: string | null;
  /** paths ever opened, so slides mount lazily but stay mounted */
  seen: string[];
}

export type OpenDocsAction =
  | { type: "route"; path: string }
  | { type: "open"; path: string }
  | { type: "activate"; path: string }
  | { type: "close"; path: string }
  | { type: "closeAll" }
  | { type: "prune"; paths: string[] };

export const initialOpenDocs: OpenDocsState = { docs: [], active: null, seen: [] };

/** Open state reducer: route sync, append with cap/LRU eviction, activate, close. */
export function openDocsReducer(state: OpenDocsState, action: OpenDocsAction): OpenDocsState {
  switch (action.type) {
    case "route":
      // every document navigation appends (or activates an open tab)
      return openDoc(state, action.path);
    case "open":
      return openDoc(state, action.path);
    case "activate":
      return activate(state, action.path);
    case "close":
      return closeDoc(state, action.path);
    case "closeAll":
      return state.docs.length === 0 && state.active === null
        ? state
        : { ...state, docs: [], active: null };
    case "prune":
      return pruneDocs(state, action.paths);
  }
}

/** Drop open docs that no longer exist in the corpus; keep `active` coherent. */
function pruneDocs(state: OpenDocsState, paths: string[]): OpenDocsState {
  if (state.docs.length === 0) return state;
  const known = new Set(paths);
  const docs = state.docs.filter((p) => known.has(p));
  if (docs.length === state.docs.length) return state;
  const active = state.active && docs.includes(state.active) ? state.active : (docs.at(-1) ?? null);
  return { ...state, docs, active, seen: state.seen.filter((p) => known.has(p)) };
}

function withSeen(state: OpenDocsState, path: string): string[] {
  return state.seen.includes(path) ? state.seen : [...state.seen, path];
}

function activate(state: OpenDocsState, path: string): OpenDocsState {
  if (!state.docs.includes(path)) return state;
  return { ...state, active: path, seen: withSeen(state, path) };
}

function openDoc(state: OpenDocsState, path: string): OpenDocsState {
  if (state.docs.includes(path)) return activate(state, path);
  const docs = [...state.docs, path];
  while (docs.length > OPEN_DOCS_CAP) {
    const evict = docs.findIndex((p) => p !== path);
    if (evict < 0) break;
    docs.splice(evict, 1);
  }
  return { docs, active: path, seen: withSeen(state, path) };
}

function closeDoc(state: OpenDocsState, path: string): OpenDocsState {
  const index = state.docs.indexOf(path);
  if (index < 0) return state;
  const docs = state.docs.filter((p) => p !== path);
  if (state.active !== path) return { ...state, docs };
  const next = docs[index - 1] ?? docs[index] ?? null;
  return { ...state, docs, active: next };
}

/** Restore persisted open docs (path/active only; `seen` is rebuilt lazily). */
export function loadOpenDocs(): OpenDocsState | null {
  const stored = loadLayout(STORE_ID) as { docs?: unknown; active?: unknown } | null;
  if (!stored || !Array.isArray(stored.docs)) return null;
  const docs = stored.docs.filter((p): p is string => typeof p === "string");
  if (docs.length === 0) return null;
  const active =
    typeof stored.active === "string" && docs.includes(stored.active)
      ? stored.active
      : (docs.at(-1) ?? null);
  return { docs, active, seen: [...docs] };
}

/** Persist the open docs; an empty stack clears the stored entry. */
export function saveOpenDocs(state: OpenDocsState): void {
  if (state.docs.length === 0) {
    clearLayout(STORE_ID);
    return;
  }
  saveLayout(STORE_ID, { docs: state.docs, active: state.active });
}
