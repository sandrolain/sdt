import { useCallback, useEffect, useMemo, useReducer, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { initialOpenDocs, openDocsReducer } from "../lib/openDocs";
import { OpenDocsContext, type OpenDocsApi } from "../lib/openDocsContext";

/** Route prefix for the documents section. */
const DOCS_PREFIX = "/docs/";

/**
 * App-level open-documents stack: survives wiki ↔ documents navigation so the
 * wiki graph/board can open a document as a documents tab.
 */
export function OpenDocsProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(openDocsReducer, initialOpenDocs);
  const navigate = useNavigate();
  const location = useLocation();

  // route → state: an unknown docs path replaces the stack, a known one activates
  useEffect(() => {
    if (!location.pathname.startsWith(DOCS_PREFIX)) return;
    const path = location.pathname.slice(DOCS_PREFIX.length);
    if (path) dispatch({ type: "route", path });
  }, [location.pathname]);

  const open = useCallback(
    (path: string) => {
      dispatch({ type: "open", path });
      navigate(`${DOCS_PREFIX}${path}`);
    },
    [navigate],
  );

  const activate = useCallback(
    (path: string) => {
      dispatch({ type: "activate", path });
      navigate(`${DOCS_PREFIX}${path}`);
    },
    [navigate],
  );

  const close = useCallback(
    (path: string) => {
      const index = state.docs.indexOf(path);
      const remaining = state.docs.filter((p) => p !== path);
      const wasActive = state.active === path;
      const next = wasActive ? (remaining[index - 1] ?? remaining[index] ?? null) : state.active;
      dispatch({ type: "close", path });
      if (wasActive) navigate(next ? `${DOCS_PREFIX}${next}` : "/docs");
    },
    [state, navigate],
  );

  const api = useMemo<OpenDocsApi>(
    () => ({ state, open, activate, close }),
    [state, open, activate, close],
  );

  return <OpenDocsContext.Provider value={api}>{children}</OpenDocsContext.Provider>;
}
