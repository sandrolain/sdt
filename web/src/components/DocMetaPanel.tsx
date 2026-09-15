import { useEffect, useMemo, useState, type ReactNode } from "react";
import { isCanvas, type CanvasResponse, type DocResponse } from "../lib/api";
import {
  booleanValue,
  formatFieldDate,
  parseFrontmatter,
  type FrontmatterField,
} from "../lib/frontmatter";
import { collectBodyLinks, collectMetaLinks, type DocLink } from "../lib/docLinks";
import { parseOutline, type OutlineItem } from "../lib/outline";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { Icon } from "../lib/icon";
import { RelatedPanel } from "./RelatedPanel";

interface DocMetaPanelProps {
  doc?: DocResponse | CanvasResponse | null;
  /** wiki page id; enables the relations card instead of the links-out card */
  relatedId?: string;
}

const DATE_KEYS = new Set(["created", "updated"]);
const CHIP_KEYS = new Set(["tags", "type"]);
const LINK_KEYS = new Set(["links", "sources", "relations"]);

/** Right-column metadata panel: frontmatter rows, heading sections, relations. */
export function DocMetaPanel({ doc, relatedId }: DocMetaPanelProps) {
  const [index, setIndex] = useState<WikiIndex | undefined>(undefined);
  const markdownDoc = doc && !isCanvas(doc) ? doc : null;

  useEffect(() => {
    if (relatedId || !markdownDoc) return;
    let alive = true;
    loadWikiIndex()
      .then((ix) => {
        if (alive) setIndex(ix);
      })
      .catch(() => {
        // link resolution degrades to name-only; rows still render
      });
    return () => {
      alive = false;
    };
  }, [relatedId, markdownDoc]);

  const fields = useMemo(
    () => (markdownDoc ? parseFrontmatter(markdownDoc.frontmatter) : []),
    [markdownDoc],
  );
  const headings = useMemo(
    () => (markdownDoc ? headingToc(parseOutline(markdownDoc.markdown)) : []),
    [markdownDoc],
  );
  const outLinks = useMemo(() => {
    if (!markdownDoc) return [];
    return [
      ...collectMetaLinks(fields, markdownDoc.path, index),
      ...collectBodyLinks(markdownDoc.markdown, markdownDoc.path, index),
    ];
  }, [markdownDoc, fields, index]);

  if (!doc) return null;

  return (
    <aside className="panel panel--meta" aria-label="Document metadata">
      <MetaCard title="Metadata" icon="info">
        {isCanvas(doc) ? (
          <CanvasMeta canvas={doc.canvas} />
        ) : fields.length === 0 ? (
          <p className="content__empty">No frontmatter.</p>
        ) : (
          <dl className="meta-rows" role="list">
            {fields.map((field) => (
              <MetaRow key={field.key} field={field} basePath={doc.path} index={index} />
            ))}
          </dl>
        )}
      </MetaCard>

      {headings.length > 0 && (
        <MetaCard title="Sections" icon="toc">
          <ul className="meta-toc" role="list">
            {headings.map((h, i) => (
              <li key={`${h.text}-${i}`} style={{ paddingLeft: `${(h.level - 1) * 0.6}rem` }}>
                <button type="button" className="meta-toc__link" onClick={() => scrollToHeading(i)}>
                  {h.text}
                </button>
              </li>
            ))}
          </ul>
        </MetaCard>
      )}

      {relatedId ? (
        <MetaCard title="Related" icon="hub">
          <RelatedPanel id={relatedId} />
        </MetaCard>
      ) : (
        <MetaCard title="Links out" icon="link">
          {outLinks.length === 0 ? (
            <p className="content__empty">No linked documents.</p>
          ) : (
            <ul className="meta-links" role="list">
              {dedupe(outLinks).map((link, i) => (
                <li key={`${link.label}-${link.href ?? ""}-${i}`}>
                  <MetaLink link={link} />
                </li>
              ))}
            </ul>
          )}
        </MetaCard>
      )}
    </aside>
  );
}

interface MetaCardProps {
  title: string;
  icon: string;
  children: ReactNode;
}

function MetaCard({ title, icon, children }: MetaCardProps) {
  return (
    <details className="meta-card" open>
      <summary className="meta-card__title">
        <Icon name={icon} />
        {title}
      </summary>
      <div className="meta-card__body">{children}</div>
    </details>
  );
}

function MetaRow({
  field,
  basePath,
  index,
}: {
  field: FrontmatterField;
  basePath: string;
  index?: WikiIndex;
}) {
  return (
    <div className="meta-row" role="listitem">
      <dt className="meta-row__label">{field.label}</dt>
      <dd className="meta-row__value">
        {field.values.length === 0 ? (
          <span className="meta-row__muted">—</span>
        ) : (
          field.values.map((value, i) => (
            <MetaValue
              key={`${value}-${i}`}
              field={field}
              value={value}
              basePath={basePath}
              index={index}
            />
          ))
        )}
      </dd>
    </div>
  );
}

function MetaValue({
  field,
  value,
  basePath,
  index,
}: {
  field: FrontmatterField;
  value: string;
  basePath: string;
  index?: WikiIndex;
}) {
  if (LINK_KEYS.has(field.key)) {
    const link = collectMetaLinks([{ key: field.key, values: [value] }], basePath, index)[0];
    return <MetaLink link={link} />;
  }
  if (DATE_KEYS.has(field.key)) {
    return <span className="meta-row__text">{formatFieldDate(value)}</span>;
  }
  if (CHIP_KEYS.has(field.key)) {
    return <span className="meta-chip">{value}</span>;
  }
  const bool = booleanValue(value);
  if (bool !== null) {
    return (
      <span className="meta-row__bool">
        <Icon name={bool ? "check" : "close"} label={bool ? "true" : "false"} />
      </span>
    );
  }
  return <span className="meta-row__text">{value}</span>;
}

function MetaLink({ link }: { link: DocLink | undefined }) {
  if (!link) return null;
  if (link.href) {
    const external = link.external
      ? { target: "_blank", rel: "noopener noreferrer" }
      : {};
    return (
      <a className="meta-link" href={link.href} {...external}>
        {link.label}
      </a>
    );
  }
  return (
    <span
      className="meta-link meta-link--muted"
      title={link.excluded ? "Excluded from the corpus" : "Unresolved target"}
    >
      {link.label}
    </span>
  );
}

function CanvasMeta({ canvas }: { canvas: unknown }) {
  const file = (canvas ?? {}) as { nodes?: unknown[]; edges?: unknown[] };
  return (
    <dl className="meta-rows" role="list">
      <div className="meta-row" role="listitem">
        <dt className="meta-row__label">Nodes</dt>
        <dd className="meta-row__value">{file.nodes?.length ?? 0}</dd>
      </div>
      <div className="meta-row" role="listitem">
        <dt className="meta-row__label">Edges</dt>
        <dd className="meta-row__value">{file.edges?.length ?? 0}</dd>
      </div>
    </dl>
  );
}

interface HeadingRef {
  level: number;
  text: string;
}

/** Flatten outline heading nodes into TOC entries (list items excluded). */
function headingToc(items: OutlineItem[]): HeadingRef[] {
  const out: HeadingRef[] = [];
  const walk = (nodes: OutlineItem[]) => {
    for (const node of nodes) {
      if (node.kind === "heading") out.push({ level: node.depth, text: node.text });
      walk(node.children);
    }
  };
  walk(items);
  return out;
}

/** Scroll the nth rendered heading into view (Render mode only). */
function scrollToHeading(index: number): void {
  const node = document.querySelector(".doc-rendered")?.querySelectorAll("h1,h2,h3,h4,h5,h6")[index];
  node?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function dedupe(links: DocLink[]): DocLink[] {
  const seen = new Set<string>();
  const out: DocLink[] = [];
  for (const link of links) {
    const key = `${link.label}|${link.href ?? ""}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(link);
  }
  return out;
}
