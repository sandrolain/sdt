import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import { useActiveSection } from "../lib/activeSection";
import { categoryColor, categoryIcon } from "../lib/categories";
import {
  isCanvas,
  isMermaid,
  type CanvasResponse,
  type DocResponse,
  type MermaidResponse,
  type TreeEntry,
} from "../lib/api";
import { linkKind, loadCorpusIndex, type CorpusIndex } from "../lib/corpusIndex";
import { collectBodyLinks, collectMetaLinks, resolveDocLink, type DocLink } from "../lib/docLinks";
import {
  booleanValue,
  formatFieldDate,
  formatFieldDateOnly,
  isRelationVerb,
  parseFrontmatter,
  valueLabel,
  type FrontmatterField,
  type ValueTone,
} from "../lib/frontmatter";
import { loadFrontmatter, type FrontmatterParse } from "../lib/frontmatterYaml";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import {
  entryKind,
  kindColor,
  kindIcon,
  kindLabel,
  kindFromPath,
  type EntryFilterKind,
} from "../lib/kinds";
import { parseOutline, type OutlineItem } from "../lib/outline";
import { normalizeHeadingText } from "../lib/headings";
import { requestSection } from "../lib/sectionRequests";
import { entryState, plansByAnalysis, statusDot, tasksByPlan } from "../lib/statusDot";
import { useReloadToken } from "../lib/useReloadToken";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { Breadcrumbs } from "./Breadcrumbs";
import { ReferencedBy } from "./ReferencedBy";
import { SemanticNeighbours } from "./SemanticNeighbours";

/** The four metadata panels, each rendered by its own dockview panel/tab. */
export type MetaTabKind = "info" | "sections" | "links" | "related";

interface DocMetaPanelProps {
  doc?: DocResponse | CanvasResponse | MermaidResponse | null;
  /** Which metadata panel this instance renders. */
  tab?: MetaTabKind;
}

const DATE_KEYS = new Set(["created", "created_at", "updated"]);
const CHIP_KEYS = new Set(["tags", "type", "categories"]);
const LINK_KEYS = new Set(["links", "sources", "relations"]);
const IMAGE_KEYS = new Set(["image"]);

/** Right-column metadata panel: frontmatter rows, heading sections, relations. */
export function DocMetaPanel({ doc, tab = "info" }: DocMetaPanelProps) {
  const [index, setIndex] = useState<WikiIndex | undefined>(undefined);
  const [corpus, setCorpus] = useState<CorpusIndex | undefined>(undefined);
  const [parsed, setParsed] = useState<{ key: string; result: FrontmatterParse } | null>(null);
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
    if (!markdownDoc) return;
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
  }, [markdownDoc, reloadToken]);

  // real YAML parse (lazy chunk); the tolerant fields render until it resolves
  // and remain the fallback when the block does not parse. The result is keyed
  // by the raw block so a stale parse never applies to another document.
  useEffect(() => {
    const raw = markdownDoc?.frontmatter;
    if (!raw) return;
    let alive = true;
    loadFrontmatter(raw).then((result) => {
      if (alive) setParsed({ key: raw, result });
    });
    return () => {
      alive = false;
    };
  }, [markdownDoc]);

  const tolerantFields = useMemo(
    () => (markdownDoc ? parseFrontmatter(markdownDoc.frontmatter) : []),
    [markdownDoc],
  );
  const current = parsed && markdownDoc?.frontmatter === parsed.key ? parsed.result : null;
  const yamlFields = current && "fields" in current ? current.fields : null;
  const yamlError = current && "error" in current ? current.error : null;
  const fields = yamlFields ?? tolerantFields;
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
  const progress = useMemo(() => {
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

  // the declared-vs-derived disagreement, on every kind the panel can reach
  const drift = useMemo(() => {
    const treeEntry = doc ? corpus?.get(doc.path)?.entry : undefined;
    if (!treeEntry || !corpus) return undefined;
    const plannedAnalyses = plansByAnalysis([...corpus.values()].map((c) => c.entry));
    return entryState(treeEntry, plannedAnalyses, taskIndex).drift;
  }, [doc, corpus, taskIndex]);

  // the document's own kind: frontmatter `kind` first, corpus/folder fallback,
  // so the metadata row shows the same icon and label as the tree.
  const docKind = useMemo<EntryFilterKind>(() => {
    if (!doc) return "other";
    const declared = fields.find((f) => f.key === "kind")?.values[0];
    if (declared) return entryKind({ kind: declared, path: doc.path } as never);
    return kindFromPath(doc.path);
  }, [doc, fields]);

  // declared lifecycle status from the frontmatter, decorated with the value
  // vocabulary shared with the tree (label + tone), independent of the derived
  // progress dot above.
  const declaredStatus = useMemo(() => {
    const value = fields.find((f) => f.key === "status")?.values[0];
    if (!value) return undefined;
    return { value, valueInfo: valueLabel(docKind, value) };
  }, [docKind, fields]);

  if (!doc) return null;

  return (
    <aside className="panel panel--meta" aria-label="Document metadata">
      {tab === "info" && (
        <div className="meta-panel">
          <Breadcrumbs path={doc.path} />

          {declaredStatus && (
            <MetaStatusRow
              label="Status"
              dotTone={declaredStatus.valueInfo?.tone ?? "neutral"}
              value={declaredStatus.valueInfo?.label ?? declaredStatus.value}
              title={declaredStatus.valueInfo?.meaning ?? declaredStatus.value}
            />
          )}
          {progress && (
            <MetaStatusRow label="Progress" dotTone={progress.tone} value={progress.label} />
          )}
          {drift && <MetaDriftWarning drift={drift} />}
          {docKind && (
            <div className="meta-status" role="listitem">
              <MetaKindIcon kind={docKind} />
              <span className="meta-status__label">Kind</span>
              <span className="meta-status__value">{kindLabel(docKind)}</span>
            </div>
          )}
          {isCanvas(doc) ? (
            <CanvasMeta canvas={doc.canvas} />
          ) : isMermaid(doc) ? (
            <MermaidMeta source={doc.source} />
          ) : (
            <>
              {yamlError && <MetaParseWarning raw={doc.frontmatter} />}
              {fields.length === 0 && !yamlError && (
                <p className="content__empty">No frontmatter.</p>
              )}
              {fields.length > 0 && (
                <dl className="meta-rows" role="list">
                  {fields
                    .filter((field) => field.key !== "status" && field.key !== "kind")
                    .map((field, i) => (
                      <MetaRow
                        key={`${field.path.join(".")}-${i}`}
                        field={field}
                        basePath={doc.path}
                        index={index}
                        corpus={corpus}
                      />
                    ))}
                </dl>
              )}
            </>
          )}
        </div>
      )}

      {tab === "sections" && (
        <div className="meta-panel">
          {headings.length > 0 ? (
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
          ) : (
            <p className="content__empty">No sections.</p>
          )}
        </div>
      )}

      {tab === "links" && (
        <div className="meta-panel">
          <h3 className="meta-panel__title">Links out</h3>
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
          {markdownDoc && (
            <>
              <h3 className="meta-panel__title meta-panel__title--spaced">Referenced by</h3>
              <ReferencedBy path={markdownDoc.path} />
            </>
          )}
        </div>
      )}

      {tab === "related" && (
        <div className="meta-panel">
          {markdownDoc ? (
            <SemanticNeighbours path={markdownDoc.path} />
          ) : (
            <p className="content__empty">No semantic neighbours.</p>
          )}
        </div>
      )}
    </aside>
  );
}

/** Visible warning + literal block when the frontmatter does not parse as YAML. */
function MetaParseWarning({ raw }: { raw: string }) {
  return (
    <div className="meta-parse-warning" role="listitem">
      <div className="meta-parse-warning__head">
        <Icon name="warning" className="meta-parse-warning__icon" label="Invalid frontmatter" />
        <span className="meta-parse-warning__text">Not valid YAML — showing the raw block.</span>
      </div>
      <details className="meta-parse-warning__raw">
        <summary>Raw frontmatter</summary>
        <pre>{raw}</pre>
      </details>
    </div>
  );
}

/** Declared-vs-derived disagreement between the frontmatter status and the
 *  effective state, with the same remedy the CLI drift lint gives. */
function MetaDriftWarning({ drift }: { drift: string }) {
  return (
    <div className="meta-drift" role="listitem">
      <div className="meta-drift__head">
        <Icon name="warning" className="meta-drift__icon" label="State drift" />
        <span className="meta-drift__text">{drift}</span>
      </div>
    </div>
  );
}

/** One declared/derived status line: tone dot, label prefix and value. */
function MetaStatusRow({
  label,
  dotTone,
  value,
  title,
}: {
  label: string;
  dotTone: ValueTone;
  value: string;
  title?: string;
}) {
  return (
    <div className="meta-status" role="listitem">
      <span className={`meta-status__dot meta-status__dot--${dotTone}`} aria-hidden="true" />
      <span className="meta-status__label">{label}</span>
      <span className="meta-status__value" title={title}>
        {value}
      </span>
    </div>
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
    // a category carries its icon so the work type is recognisable at a glance
    if (field.key === "categories") {
      return (
        <span className="meta-chip meta-chip--category">
          <Icon
            name={categoryIcon(value)}
            className="meta-chip__icon"
            style={{ color: categoryColor(value) }}
            label={value}
          />
          {value}
        </span>
      );
    }
    return <span className="meta-chip">{value}</span>;
  }
  if (IMAGE_KEYS.has(field.key)) {
    return (
      <img className="meta-row__image" src={imageUrl(value, basePath)} alt={value} title={value} />
    );
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
    // Resolve the target through the corpus index for the row's category, date
    // and status dot (no extra fetch; the index is already loaded).
    const entry = link.external
      ? undefined
      : corpus?.get(link.href.replace(/^\/docs\//, ""))?.entry;
    const date = entry ? entryDateLine(entry) : "";
    const statusInfo = entry?.status ? valueLabel(entry.kind ?? "", entry.status) : null;
    return (
      <a
        className={`meta-link meta-link--row${current ? " is-current" : ""}`}
        href={link.href}
        aria-current={current ? "page" : undefined}
        {...external}
      >
        <span className="tree-entry__glyph">{kind && <MetaKindIcon kind={kind} />}</span>
        {entry?.categories?.[0] && (
          <Icon
            name={categoryIcon(entry.categories[0])}
            className="tree-entry__category"
            style={{ color: categoryColor(entry.categories[0]) }}
            title={entry.categories[0]}
          />
        )}
        <span className="tree-entry__text">
          <span className="tree-entry__title">{link.label}</span>
          {date && <span className="tree-entry__date">{date}</span>}
        </span>
        {statusInfo && (
          <span
            className={`tree-entry__dot tree-entry__dot--${statusInfo.tone}`}
            title={statusInfo.label}
            aria-label={statusInfo.label}
            role="img"
          />
        )}
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

/** Compact `created · modified` line for a sidebar link row. */
function entryDateLine(entry: TreeEntry): string {
  const parts: string[] = [];
  if (entry.created) parts.push(formatFieldDateOnly(entry.created));
  if (entry.modified && entry.modified !== entry.created)
    parts.push(formatFieldDateOnly(entry.modified));
  return parts.join(" · ");
}

/** Coloured kind glyph shown before a document link or the kind row. */
function MetaKindIcon({ kind }: { kind: EntryFilterKind }) {
  return (
    <Icon
      name={kindIcon(kind)}
      className="meta-link__icon"
      style={{ color: kindColor(kind) }}
      label={kindLabel(kind)}
      title={kindLabel(kind)}
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
      if (node.kind === "heading")
        out.push({ level: node.depth, text: normalizeHeadingText(node.text) });
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
