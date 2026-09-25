import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { useActiveSection } from "../lib/activeSection";
import {
  isCanvas,
  isMermaid,
  type CanvasResponse,
  type DocResponse,
  type MermaidResponse,
} from "../lib/api";
import { linkKind, loadCorpusIndex, type CorpusIndex } from "../lib/corpusIndex";
import { collectBodyLinks, collectMetaLinks, resolveDocLink, type DocLink } from "../lib/docLinks";
import {
  booleanValue,
  formatFieldDate,
  isRelationVerb,
  parseFrontmatter,
  type FrontmatterField,
} from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import { kindColor, kindIcon, type EntryFilterKind } from "../lib/kinds";
import { parseOutline, type OutlineItem } from "../lib/outline";
import { requestSection } from "../lib/sectionRequests";
import { plansByAnalysis, statusDot, tasksByPlan } from "../lib/statusDot";
import { useReloadToken } from "../lib/useReloadToken";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { Breadcrumbs } from "./Breadcrumbs";
import { RelatedPanel } from "./RelatedPanel";

interface DocMetaPanelProps {
  doc?: DocResponse | CanvasResponse | MermaidResponse | null;
  /** wiki page id; enables the relations card instead of the links-out card */
  relatedId?: string;
}

const DATE_KEYS = new Set(["created", "updated"]);
const CHIP_KEYS = new Set(["tags", "type"]);
const LINK_KEYS = new Set(["links", "sources", "relations"]);
const IMAGE_KEYS = new Set(["image"]);

/** Right-column metadata panel: frontmatter rows, heading sections, relations. */
export function DocMetaPanel({ doc, relatedId }: DocMetaPanelProps) {
  const [index, setIndex] = useState<WikiIndex | undefined>(undefined);
  const [corpus, setCorpus] = useState<CorpusIndex | undefined>(undefined);
  const activeSection = useActiveSection();
  const markdownDoc = doc && !isCanvas(doc) && !isMermaid(doc) ? doc : null;
  const reloadToken = useReloadToken();
  const activeHeading = activeSection.path === doc?.path ? activeSection.key : null;

  useEffect(() => {
    let alive = true;
    loadCorpusIndex()
      .then((ix) => {
        if (alive) setCorpus(ix);
      })
      .catch(() => {
        // kind colours degrade to the folder-derived fallback
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

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
  }, [relatedId, markdownDoc, reloadToken]);

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

  // status dot for the current doc, matching the tree indicator; derived from
  // the plan-reference and task indexes in the corpus.
  const taskIndex = useMemo(
    () => (corpus ? tasksByPlan([...corpus.values()].map((c) => c.entry)) : new Map()),
    [corpus],
  );
  const status = useMemo(() => {
    const treeEntry = doc ? corpus?.get(doc.path)?.entry : undefined;
    if (!treeEntry || !corpus) return undefined;
    if (
      treeEntry.kind !== "plan" &&
      treeEntry.kind !== "tasks" &&
      treeEntry.kind !== "analysis" &&
      treeEntry.kind !== "questions"
    ) {
      return undefined;
    }
    const plannedAnalyses = plansByAnalysis([...corpus.values()].map((c) => c.entry));
    return statusDot(treeEntry, plannedAnalyses, taskIndex) ?? undefined;
  }, [doc, corpus, taskIndex]);

  if (!doc) return null;

  return (
    <aside className="panel panel--meta" aria-label="Document metadata">
      <Breadcrumbs path={doc.path} />

      {headings.length > 0 && (
        <MetaCard title="Sections" icon="toc">
          <ul className="meta-toc" role="list">
            {headings.map((h, i) => {
              const isActive = activeHeading === h.text;
              return (
                <li key={`${h.text}-${i}`} style={{ paddingLeft: `${(h.level - 1) * 0.6}rem` }}>
                  <button
                    type="button"
                    className={`meta-toc__link${isActive ? " is-active" : ""}`}
                    aria-current={isActive ? "true" : undefined}
                    onClick={() => {
                      requestSection(doc.path, h.text);
                    }}
                  >
                    {h.text}
                  </button>
                </li>
              );
            })}
          </ul>
        </MetaCard>
      )}

      <MetaCard title="Metadata" icon="info">
        {status && (
          <div className="meta-status" role="listitem">
            <span
              className={`meta-status__dot meta-status__dot--${status.tone}`}
              aria-hidden="true"
            />
            <span className="meta-status__label">{status.label}</span>
          </div>
        )}
        {isCanvas(doc) ? (
          <CanvasMeta canvas={doc.canvas} />
        ) : isMermaid(doc) ? (
          <MermaidMeta source={doc.source} />
        ) : fields.length === 0 ? (
          <p className="content__empty">No frontmatter.</p>
        ) : (
          <dl className="meta-rows" role="list">
            {fields.map((field) => (
              <MetaRow
                key={field.key}
                field={field}
                basePath={doc.path}
                index={index}
                corpus={corpus}
              />
            ))}
          </dl>
        )}
      </MetaCard>

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
                  <MetaLink link={link} corpus={corpus} />
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
        <span className="meta-card__label">{title}</span>
        <Icon name="expand_more" className="meta-card__chevron" title={title} />
      </summary>
      <div className="meta-card__body">{children}</div>
    </details>
  );
}

function MetaRow({
  field,
  basePath,
  index,
  corpus,
}: {
  field: FrontmatterField;
  basePath: string;
  index?: WikiIndex;
  corpus?: CorpusIndex;
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
              corpus={corpus}
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
  corpus,
}: {
  field: FrontmatterField;
  value: string;
  basePath: string;
  index?: WikiIndex;
  corpus?: CorpusIndex;
}) {
  if (LINK_KEYS.has(field.key)) {
    const link = collectMetaLinks([{ key: field.key, values: [value] }], basePath, index)[0];
    return <MetaLink link={link} corpus={corpus} />;
  }
  if (isRelationVerb(field.key)) {
    return <MetaLink link={resolveDocLink(value, basePath, index)} corpus={corpus} />;
  }
  if (DATE_KEYS.has(field.key)) {
    return <span className="meta-row__text">{formatFieldDate(value)}</span>;
  }
  if (CHIP_KEYS.has(field.key)) {
    return <span className="meta-chip">{value}</span>;
  }
  if (IMAGE_KEYS.has(field.key)) {
    return <img className="meta-row__image" src={imageUrl(value, basePath)} alt="" />;
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

function MetaLink({ link, corpus }: { link: DocLink | undefined; corpus?: CorpusIndex }) {
  const location = useLocation();
  if (!link) return null;
  if (link.href) {
    const external = link.external ? { target: "_blank", rel: "noopener noreferrer" } : {};
    const current =
      !link.external &&
      link.href.startsWith("/") &&
      link.href === `${location.pathname}${location.search}`;
    const kind = link.external ? undefined : linkKind(link.href, corpus);
    return (
      <a
        className={`meta-link${current ? " is-current" : ""}`}
        href={link.href}
        aria-current={current ? "page" : undefined}
        {...external}
      >
        {kind && <MetaKindIcon kind={kind} />}
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

/** Coloured kind glyph shown before a document link. */
function MetaKindIcon({ kind }: { kind: EntryFilterKind }) {
  return (
    <Icon
      name={kindIcon(kind)}
      className="meta-link__icon"
      style={{ color: kindColor(kind) }}
      label={`kind: ${kind}`}
      title={kind}
    />
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

function MermaidMeta({ source }: { source: string }) {
  const lines = source === "" ? 0 : source.split("\n").length;
  return (
    <dl className="meta-rows" role="list">
      <div className="meta-row" role="listitem">
        <dt className="meta-row__label">Lines</dt>
        <dd className="meta-row__value">{lines}</dd>
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
