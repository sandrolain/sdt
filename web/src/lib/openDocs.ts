/** Session-scoped stack of open documents (oldest → newest) for the tab rail. */

export const OPEN_DOCS_CAP = 8;

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
  | { type: "close"; path: string };

export const initialOpenDocs: OpenDocsState = { docs: [], active: null, seen: [] };

/** Open state reducer: route sync, append with cap/LRU eviction, activate, close. */
export function openDocsReducer(state: OpenDocsState, action: OpenDocsAction): OpenDocsState {
  switch (action.type) {
    case "route":
      return state.docs.includes(action.path) ? activate(state, action.path) : replace(action.path);
    case "open":
      return openDoc(state, action.path);
    case "activate":
      return activate(state, action.path);
    case "close":
      return closeDoc(state, action.path);
  }
}

function withSeen(state: OpenDocsState, path: string): string[] {
  return state.seen.includes(path) ? state.seen : [...state.seen, path];
}

function replace(path: string): OpenDocsState {
  return { docs: [path], active: path, seen: [path] };
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
