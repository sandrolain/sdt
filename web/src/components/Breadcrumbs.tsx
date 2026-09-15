import { NavLink } from "react-router-dom";
import { Icon } from "../lib/icon";
import { displayTitle } from "../lib/titles";

interface BreadcrumbsProps {
  /** corpus-relative document path, e.g. context/wiki/backend/auth.md */
  path: string;
  /** already-resolved document title; falls back to the title cascade */
  title?: string;
}

/** Project root → folders → document title trail for the document header. */
export function Breadcrumbs({ path, title }: BreadcrumbsProps) {
  const segments = path.split("/").filter(Boolean);
  // drop the "context" corpus root and the filename; keep intermediate folders
  const folders = segments.slice(1, -1);
  const leaf = title || displayTitle({ path });

  return (
    <nav className="breadcrumbs" aria-label="Breadcrumb">
      <NavLink className="breadcrumbs__root" to="/docs" end>
        <Icon name="home" />
        Corpus
      </NavLink>
      {folders.map((folder) => (
        <span key={folder} className="breadcrumbs__seg">
          <Icon name="chevron_right" className="breadcrumbs__sep" />
          {folder}
        </span>
      ))}
      <span className="breadcrumbs__seg breadcrumbs__seg--current">
        <Icon name="chevron_right" className="breadcrumbs__sep" />
        {leaf}
      </span>
    </nav>
  );
}
