import { useEffect, useMemo, useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { fetchTree, type TreeEntry } from "../lib/api";
import { MAP_ICON } from "../lib/documentModes";
import { formatFieldDate } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import { entryKind, kindColor, kindIcon, kindLabel, type EntryFilterKind } from "../lib/kinds";
import { planReferencedAnalyses, normalizeRef, statusDot } from "../lib/statusDot";
import { displayTitle, filenameDate } from "../lib/titles";
import { useTreeFilter } from "../lib/treeFilterStore";
import {
  folderCount,
  groupByFolder,
  groupByKind,
  groupByObjective,
  groupByPlan,
  sortEntries,
  type FolderGroup,
} from "../lib/treeSort";
import { useTreeSort } from "../lib/treeSortStore";
import { useReloadToken } from "../lib/useReloadToken";
import { SkeletonLines } from "./Skeleton";
import { TreeToolbar } from "./TreeToolbar";

export function Tree({ onResetLayout }: { onResetLayout?: () => void }) {
  const [entries, setEntries] = useState<TreeEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { key: sortKey } = useTreeSort();
  const { hideCompleted } = useTreeFilter();
  const [openKinds, setOpenKinds] = useState<Set<EntryFilterKind>>(() => new Set());
  const reloadToken = useReloadToken();
  const location = useLocation();

  useEffect(() => {
    let alive = true;
    fetchTree()
      .then((res) => {
        if (alive) setEntries(res.entries);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  // kind section of the document currently open in the documents route
  const activeKind = useMemo(() => {
    if (!entries || !location.pathname.startsWith("/docs/")) return null;
    const path = decodeURIComponent(location.pathname.slice("/docs/".length));
    const entry = entries.find((e) => e.path === path);
    return entry ? entryKind(entry) : null;
  }, [entries, location.pathname]);

  // reveal the active entry once its section is open (sections are derived-open)
  useEffect(() => {
    if (!entries || !activeKind) return;
    document
      .querySelector(".panel--tree .tree-entry.is-active")
      ?.scrollIntoView({ block: "nearest" });
  }, [entries, location.pathname, activeKind]);

  const plannedAnalyses = planReferencedAnalyses(entries ?? []);
  const visibleEntries =
    entries && hideCompleted
      ? entries.filter((entry) => statusDot(entry, plannedAnalyses))
      : entries;
  const groups = visibleEntries
    ? groupByKind(visibleEntries).map((group) => ({
        kind: group.kind,
        entries: sortEntries(group.entries, sortKey),
      }))
    : [];
  // plan lookup for task grouping (labels/order); uses every entry so a
  // filtered-out completed plan still labels its tasks.
  const planIndex = useMemo(() => {
    const map = new Map<string, TreeEntry>();
    for (const entry of entries ?? []) {
      if (entryKind(entry) === "plan") map.set(normalizeRef(entry.path), entry);
    }
    return map;
  }, [entries]);

  return (
    <aside className="panel panel--tree" aria-label="Corpus tree">
      <TreeToolbar onResetLayout={onResetLayout} />
      {error ? (
        <p className="content__empty">Tree error: {error}</p>
      ) : !entries ? (
        <SkeletonLines count={6} label="Loading tree" />
      ) : (
        <div className="tree-groups">
          {groups.map((group) => (
            <details
              key={group.kind}
              className="tree-folder"
              open={openKinds.has(group.kind) || activeKind === group.kind}
              onToggle={(e) => {
                const open = e.currentTarget.open;
                setOpenKinds((prev) => {
                  const next = new Set(prev);
                  if (open) next.add(group.kind);
                  else next.delete(group.kind);
                  return next;
                });
              }}
            >
              <summary className="tree-folder__header">
                <Icon name="expand_more" className="tree-folder__chevron" />
                <Icon
                  name={kindIcon(group.kind)}
                  className="tree-folder__icon"
                  style={{ color: kindColor(group.kind) }}
                  title={kindLabel(group.kind)}
                />
                <span className="tree-folder__label">{kindLabel(group.kind)}</span>
                <span className="tree-folder__count">{group.entries.length}</span>
              </summary>
              {group.entries.length === 0 ? (
                <p className="content__empty tree-empty">No documents.</p>
              ) : group.kind === "analysis" ? (
                <AnalysisEntries entries={group.entries} plannedAnalyses={plannedAnalyses} />
              ) : group.kind === "wiki" ? (
                <WikiEntries entries={group.entries} plannedAnalyses={plannedAnalyses} />
              ) : group.kind === "tasks" ? (
                <PlanEntries
                  entries={group.entries}
                  plans={planIndex}
                  plannedAnalyses={plannedAnalyses}
                />
              ) : (
                <EntryList entries={group.entries} plannedAnalyses={plannedAnalyses} />
              )}
            </details>
          ))}
        </div>
      )}
    </aside>
  );
}

type PlannedAnalyses = ReturnType<typeof planReferencedAnalyses>;

/** Flat entry list for a kind folder. */
function EntryList({
  entries,
  plannedAnalyses,
}: {
  entries: TreeEntry[];
  plannedAnalyses: PlannedAnalyses;
}) {
  return (
    <ul role="list">
      {entries.map((entry) => (
        <TreeEntryRow key={entry.path} entry={entry} plannedAnalyses={plannedAnalyses} />
      ))}
    </ul>
  );
}

/** Wiki kind folder: entries at the wiki root stay flat, deeper entries nest
 *  under path-derived folder subgroups. */
function WikiEntries({
  entries,
  plannedAnalyses,
}: {
  entries: TreeEntry[];
  plannedAnalyses: PlannedAnalyses;
}) {
  const { rootEntries, folders } = groupByFolder(entries, "context/wiki/");
  return (
    <>
      {rootEntries.length > 0 && (
        <EntryList entries={rootEntries} plannedAnalyses={plannedAnalyses} />
      )}
      {folders.map((folder) => (
        <FolderNodeView key={folder.path} node={folder} plannedAnalyses={plannedAnalyses} />
      ))}
    </>
  );
}

/** One path-derived wiki folder (recursive), with a subtree count badge. */
function FolderNodeView({
  node,
  plannedAnalyses,
}: {
  node: FolderGroup;
  plannedAnalyses: PlannedAnalyses;
}) {
  return (
    <details className="tree-folder tree-folder--dir">
      <summary className="tree-folder__header">
        <Icon name="expand_more" className="tree-folder__chevron" />
        <Icon name="folder" className="tree-folder__icon" />
        <span className="tree-folder__label">{node.name}</span>
        <span className="tree-folder__count">{folderCount(node)}</span>
      </summary>
      {node.entries.length > 0 && (
        <EntryList entries={node.entries} plannedAnalyses={plannedAnalyses} />
      )}
      {node.children.map((child) => (
        <FolderNodeView key={child.path} node={child} plannedAnalyses={plannedAnalyses} />
      ))}
    </details>
  );
}

/** Task kind folder: tasks grouped under the plan they reference, tasks
 *  without a plan reference stay at the folder root. */
function PlanEntries({
  entries,
  plans,
  plannedAnalyses,
}: {
  entries: TreeEntry[];
  plans: Map<string, TreeEntry>;
  plannedAnalyses: PlannedAnalyses;
}) {
  return (
    <>
      {groupByPlan(entries, plans).map((group) =>
        group.plan === "" ? (
          <EntryList key="__ungrouped" entries={group.entries} plannedAnalyses={plannedAnalyses} />
        ) : (
          <details key={group.plan} className="tree-folder tree-folder--plan">
            <summary className="tree-folder__header">
              <Icon name="expand_more" className="tree-folder__chevron" />
              <Icon name="map" className="tree-folder__icon" />
              <span className="tree-folder__label">{group.label}</span>
              <span className="tree-folder__count">{group.entries.length}</span>
            </summary>
            <EntryList entries={group.entries} plannedAnalyses={plannedAnalyses} />
          </details>
        ),
      )}
    </>
  );
}

/** Analysis kind folder: named `objective` sub-folders, entries without an
 *  objective stay at the folder root. */
function AnalysisEntries({
  entries,
  plannedAnalyses,
}: {
  entries: TreeEntry[];
  plannedAnalyses: PlannedAnalyses;
}) {
  return (
    <>
      {groupByObjective(entries).map((group) =>
        group.objective === "" ? (
          <EntryList key="__ungrouped" entries={group.entries} plannedAnalyses={plannedAnalyses} />
        ) : (
          <details key={group.objective} className="tree-folder tree-folder--objective">
            <summary className="tree-folder__header">
              <Icon name="expand_more" className="tree-folder__chevron" />
              <Icon name="flag" className="tree-folder__icon" />
              <span className="tree-folder__label">{group.objective}</span>
              <span className="tree-folder__count">{group.entries.length}</span>
            </summary>
            <EntryList entries={group.entries} plannedAnalyses={plannedAnalyses} />
          </details>
        ),
      )}
    </>
  );
}

function TreeEntryRow({
  entry,
  plannedAnalyses,
}: {
  entry: TreeEntry;
  plannedAnalyses: PlannedAnalyses;
}) {
  const dot = statusDot(entry, plannedAnalyses);
  const kind = entryKind(entry);
  const kindName = kindLabel(kind);
  return (
    <li>
      <NavLink
        to={
          entry.canvas
            ? `/wiki/board?file=${encodeURIComponent(entry.path)}`
            : `/docs/${entry.path}`
        }
        className={({ isActive }) => `tree-entry${isActive ? " is-active" : ""}`}
        title={`${entry.summary || entry.path} · ${kindName}`}
        end
      >
        <span className="tree-entry__glyph">
          {entry.image ? (
            <img className="tree-entry__thumb" src={imageUrl(entry.image, entry.path)} alt="" />
          ) : (
            <Icon name={kindIcon(kind)} title={kindName} />
          )}
        </span>
        <span className="tree-entry__text">
          <span className="tree-entry__title">{entryTitle(entry)}</span>
          {entryDate(entry) && <span className="tree-entry__date">{entryDate(entry)}</span>}
        </span>
        {dot && (
          <span
            className={`tree-entry__dot tree-entry__dot--${dot.tone}`}
            title={dot.label}
            aria-label={dot.label}
            role="img"
          />
        )}
        {entry.isMap && (
          <span className="tree-entry__map" title="Map document">
            <Icon name={MAP_ICON} label="Map document" />
          </span>
        )}
      </NavLink>
    </li>
  );
}

function entryTitle(e: TreeEntry): string {
  if (e.canvas) return e.path;
  return displayTitle({ title: e.title, path: e.path });
}

/** Small date line: frontmatter `created`, else the filename date prefix. */
function entryDate(e: TreeEntry): string {
  const raw = e.created || filenameDate(e.path);
  return raw ? formatFieldDate(raw) : "";
}
