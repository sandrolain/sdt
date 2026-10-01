import { useEffect, useMemo, useState } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { Icon } from "../lib/icon";
import { loadCorpusIndex, type CorpusIndex } from "../lib/corpusIndex";
import { relationFor, siblingsOf } from "../lib/docRelations";

interface BreadcrumbsProps {
  /** corpus-relative document path, e.g. context/wiki/backend/auth.md */
  path: string;
}

/**
 * Corpus path bar: the `context/…` path, an ancestor link for the typed
 * relation (a task file names its plan, a plan its analysis), a sibling jump and
 * the copy-to-clipboard affordance. A segment with no document behind it stays
 * plain text — there is no folder listing route, so a link would dead-end.
 */
export function Breadcrumbs({ path }: BreadcrumbsProps) {
  const [copied, setCopied] = useState(false);
  const [index, setIndex] = useState<CorpusIndex | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    let alive = true;
    loadCorpusIndex()
      .then((ix) => {
        if (alive) setIndex(ix);
      })
      .catch(() => {
        // the path bar stays usable without the index (path + copy only)
      });
    return () => {
      alive = false;
    };
  }, []);

  const relation = useMemo(() => relationFor(path, index), [path, index]);
  const siblings = useMemo(() => siblingsOf(path, index), [path, index]);

  const copy = () => {
    if (!navigator.clipboard) return;
    void navigator.clipboard.writeText(path).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    });
  };

  return (
    <nav className="doc-path" aria-label="Document path">
      <NavLink
        className="doc-path__home"
        to="/docs"
        end
        aria-label="Corpus root"
        title="Corpus root"
      >
        <Icon name="home" />
      </NavLink>
      <div className="doc-path__row">
        <span className="doc-path__value" title={path}>
          {path}
        </span>
        <button
          type="button"
          className="doc-path__copy"
          aria-label={copied ? "Path copied" : "Copy path"}
          title={copied ? "Copied" : "Copy path"}
          onClick={copy}
        >
          <Icon name={copied ? "check" : "content_copy"} />
        </button>
      </div>
      {relation && (
        <NavLink
          className="doc-path__relation"
          to={`/docs/${relation.path}`}
          title={`${relation.label}: ${relation.title}`}
        >
          <Icon name="subdirectory_arrow_right" className="doc-path__relation-icon" />
          <span className="doc-path__relation-label">{relation.label}</span>
          <span className="doc-path__relation-title">{relation.title}</span>
        </NavLink>
      )}
      {siblings.length > 0 && (
        <select
          className="doc-path__siblings"
          aria-label="Sibling document"
          title="Sibling document"
          value=""
          onChange={(e) => {
            const target = e.target.value;
            if (target) navigate(`/docs/${target}`);
          }}
        >
          <option value="">Siblings…</option>
          {siblings.map((sibling) => (
            <option key={sibling.path} value={sibling.path}>
              {sibling.title}
            </option>
          ))}
        </select>
      )}
    </nav>
  );
}
