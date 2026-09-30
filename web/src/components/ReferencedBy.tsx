import { useEffect, useState } from "react";
import { NavLink } from "react-router-dom";
import { fetchBacklinks, type Backlink } from "../lib/api";
import { KIND_ORDER, kindIcon, kindLabel, kindFromPath, type EntryFilterKind } from "../lib/kinds";
import { displayTitle } from "../lib/titles";
import { useReloadToken } from "../lib/useReloadToken";

interface ReferencedByProps {
  /** corpus path of the document being read */
  path: string;
  /** heading rendered above the list; omit it when the host has its own title */
  heading?: string;
  /** compact list for the hover preview, full rows for the metadata panel */
  compact?: boolean;
}

/** Whether a frontmatter kind is one the corpus vocabulary offers. */
function kindIncludes(kind: string | undefined): boolean {
  return !!kind && (KIND_ORDER as string[]).includes(kind);
}

/** Human label for the frontmatter list that produced the edge. */
function viaLabel(via: string): string {
  if (via === "sources") return "source";
  if (via === "relation") return "derives from";
  return "links to";
}

/**
 * The inbound references of a document (analysis N1/Q3): which documents point
 * at this one, served by /api/backlinks from the index built at startup. No
 * client-side derivation — the corpus graph has one owner.
 */
export function ReferencedBy({ path, heading, compact = false }: ReferencedByProps) {
  // keyed by path so a new document shows the loading state without an effect
  const [state, setState] = useState<{
    path: string;
    referrers: Backlink[] | null;
    error: string | null;
  }>({ path, referrers: null, error: null });
  const reloadToken = useReloadToken();

  useEffect(() => {
    if (!path) return;
    let alive = true;
    fetchBacklinks(path)
      .then((res) => {
        if (alive) setState({ path, referrers: res.referrers ?? [], error: null });
      })
      .catch((err: unknown) => {
        if (alive)
          setState({
            path,
            referrers: null,
            error: err instanceof Error ? err.message : String(err),
          });
      });
    return () => {
      alive = false;
    };
  }, [path, reloadToken]);

  const referrers = state.path === path ? state.referrers : null;
  const error = state.path === path ? state.error : null;

  if (error) return <p className="content__empty">backlinks error: {error}</p>;
  if (referrers === null) return <p className="content__empty">Loading referenced by…</p>;
  if (referrers.length === 0) {
    return <p className="content__empty">No document references this one yet.</p>;
  }

  return (
    <div className="referrers">
      {heading && (
        <h3 className="referrers__heading">
          {heading} <span className="referrers__count">{referrers.length}</span>
        </h3>
      )}
      <ul
        className={compact ? "referrers__list referrers__list--compact" : "referrers__list"}
        role="list"
      >
        {referrers.map((ref) => {
          const kind: EntryFilterKind = kindIncludes(ref.kind)
            ? (ref.kind as EntryFilterKind)
            : kindFromPath(ref.path);
          return (
            <li key={ref.path}>
              <NavLink
                className="referrers__item"
                to={`/docs/${ref.path}`}
                title={ref.summary || ref.path}
                end
              >
                <span className="referrers__text">
                  <span className="referrers__title">
                    {displayTitle({ title: ref.title, path: ref.path })}
                  </span>
                  {!compact && <span className="referrers__via">{viaLabel(ref.via)}</span>}
                </span>
                <span className="referrers__kind" title={kindLabel(kind)}>
                  <span className="referrers__kind-icon" aria-hidden="true">
                    {kindIcon(kind)}
                  </span>
                  {kindLabel(kind)}
                </span>
              </NavLink>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
