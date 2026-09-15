import { useEffect, useMemo, useReducer, useRef, type MouseEvent } from "react";
import { useNavigate } from "react-router-dom";
import { initialOpenDocs, openDocsReducer } from "../lib/openDocs";
import { OpenDocsContext } from "../lib/openDocsContext";
import { displayTitle } from "../lib/titles";
import { useDoc } from "../lib/useDoc";
import { DocDetail } from "./DocDetail";
import { Icon } from "../lib/icon";

interface OpenDocsRailProps {
  /** active document path from the route, e.g. context/wiki/foo.md */
  path?: string;
}

/** Side-by-side open-document rail: session stack, tabs and snap slides. */
export function OpenDocsRail({ path }: OpenDocsRailProps) {
  const [state, dispatch] = useReducer(openDocsReducer, initialOpenDocs);
  const navigate = useNavigate();

  const api = useMemo(() => ({ state, dispatch }), [state]);

  useEffect(() => {
    if (path) dispatch({ type: "route", path });
  }, [path]);

  const railRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const index = state.active ? state.docs.indexOf(state.active) : -1;
    if (index < 0) return;
    const slide = railRef.current?.children[index];
    if (slide instanceof HTMLElement) slide.scrollIntoView({ inline: "start", block: "nearest" });
  }, [state.active, state.docs]);

  const selectTab = (target: string) => {
    dispatch({ type: "activate", path: target });
    navigate(`/docs/${target}`);
  };

  const closeTab = (target: string) => {
    const index = state.docs.indexOf(target);
    const remaining = state.docs.filter((p) => p !== target);
    const wasActive = state.active === target;
    const next = wasActive ? (remaining[index - 1] ?? remaining[index] ?? null) : state.active;
    dispatch({ type: "close", path: target });
    if (wasActive) navigate(next ? `/docs/${next}` : "/docs");
  };

  const onRailClick = (event: MouseEvent<HTMLDivElement>) => {
    const anchor = (event.target as HTMLElement).closest("a");
    const href = anchor?.getAttribute("href") ?? "";
    if (!anchor || !href.startsWith("#/docs/")) return;
    event.preventDefault();
    const target = href.slice("#/docs/".length);
    dispatch({ type: "open", path: target });
    navigate(`/docs/${target}`);
  };

  return (
    <OpenDocsContext.Provider value={api}>
      <div className="open-docs">
        <div className="open-docs__tabs" role="tablist" aria-label="Open documents">
          {state.docs.map((docPath) => {
            const active = docPath === state.active;
            return (
              <span key={docPath} className={`open-docs__tab${active ? " is-active" : ""}`}>
                <button
                  type="button"
                  role="tab"
                  aria-selected={active}
                  className="open-docs__tab-title"
                  onClick={() => selectTab(docPath)}
                >
                  {tabTitle(docPath)}
                </button>
                <button
                  type="button"
                  className="open-docs__close"
                  aria-label={`Close ${tabTitle(docPath)}`}
                  onClick={() => closeTab(docPath)}
                >
                  <Icon name="close" />
                </button>
              </span>
            );
          })}
        </div>
        <div className="open-docs__rail" ref={railRef} onClick={onRailClick}>
          {state.docs.length === 0 ? (
            <p className="content__empty">Select a document from the tree.</p>
          ) : (
            state.docs.map((docPath) => (
              <section
                key={docPath}
                className={`open-docs__slide${docPath === state.active ? " is-active" : ""}`}
                data-path={docPath}
              >
                {state.seen.includes(docPath) ? (
                  <DocSlide path={docPath} />
                ) : (
                  <p className="content__empty">…</p>
                )}
              </section>
            ))
          )}
        </div>
      </div>
    </OpenDocsContext.Provider>
  );
}

function DocSlide({ path }: { path: string }) {
  const { doc, error, loading } = useDoc(path);
  return <DocDetail path={path} doc={doc} error={error} loading={loading} />;
}

function tabTitle(path: string): string {
  return displayTitle({ path });
}
