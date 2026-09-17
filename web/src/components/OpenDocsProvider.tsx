import { useCallback, useEffect, useMemo, useReducer, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { fetchTree } from "../lib/api";
import { initialOpenDocs, loadOpenDocs, openDocsReducer, saveOpenDocs } from "../lib/openDocs";
import { OpenDocsContext, type OpenDocsApi } from "../lib/openDocsContext";

/** Route prefix for the documents section. */
const DOCS_PREFIX = "/docs/";

/**
 * App-level open-documents stack: survives wiki ↔ documents navigation so the
 * wiki graph/board can open a document as a documents tab, and persists across
 * reloads through versioned localStorage.
 */
export function OpenDocsProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(openDocsReducer, undefined, initOpenDocs);
  const navigate = useNavigate();
  const location = useLocation();

  // route → state: an unknown docs path replaces the stack, a known one activates
  useEffect(() => {
    if (!location.pathname.startsWith(DOCS_PREFIX)) return;
    const path = location.pathname.slice(DOCS_PREFIX.length);
    if (path) dispatch({ type: "route", path });
  }, [location.pathname]);

  // state → storage
  useEffect(() => {
    saveOpenDocs(state);
  }, [state]);

  // drop restored tabs whose files disappeared from the corpus
  useEffect(() => {
    let alive = true;
    fetchTree()
      .then((res) => {
        const paths = (res.entries ?? []).map((e) => e.path);
        // an empty listing means "no corpus / tree unavailable", not "delete all"
        if (alive && paths.length > 0) dispatch({ type: "prune", paths });
      })
      .catch(() => {
        // tree unavailable: keep the restored stack as-is
      });
    return () => {
      alive = false;
    };
  }, []);

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

  const closeAll = useCallback(() => {
    dispatch({ type: "closeAll" });
    navigate("/docs");
  }, [navigate]);

  // Cmd/Ctrl+W closes the active document unless focus is in a form control
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== "w") return;
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName?.toLowerCase();
      if (tag === "input" || tag === "textarea" || tag === "select" || target?.isContentEditable) {
        return;
      }
      if (!state.active) return;
      e.preventDefault();
      close(state.active);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [state.active, close]);

  const api = useMemo<OpenDocsApi>(
    () => ({ state, open, activate, close, closeAll }),
    [state, open, activate, close, closeAll],
  );

  return <OpenDocsContext.Provider value={api}>{children}</OpenDocsContext.Provider>;
}

/** Hydrate the reducer from persisted tabs, falling back to the empty state. */
function initOpenDocs(): typeof initialOpenDocs {
  return loadOpenDocs() ?? initialOpenDocs;
}
