import { useState } from "react";
import { NavLink } from "react-router-dom";
import { Icon } from "../lib/icon";

interface BreadcrumbsProps {
  /** corpus-relative document path, e.g. context/wiki/backend/auth.md */
  path: string;
}

/**
 * Corpus path bar for the right column: shows the full `context/…` path with a
 * copy-to-clipboard affordance. The document title lives in the main header, so
 * the path is the only breadcrumb surface.
 */
export function Breadcrumbs({ path }: BreadcrumbsProps) {
  const [copied, setCopied] = useState(false);

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
    </nav>
  );
}
