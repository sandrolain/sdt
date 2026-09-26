import { useEffect, useMemo, useState, type ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { fetchTree, type TreeEntry } from "../lib/api";
import { MAP_ICON } from "../lib/documentModes";
import { formatFieldDate } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import { entryKind, kindColor, kindIcon, kindLabel, type EntryFilterKind } from "../lib/kinds";
import {
  entryCompleted,
  groupDot,
  normalizeRef,
  plansByAnalysis,
  statusDot,
  taskProgress,
  taskProgressLabel,
  tasksByPlan,
  withTaskObjectives,
  type StatusDot,
} from "../lib/statusDot";
import { displayTitle, filenameDate } from "../lib/titles";
import { useTreeFilter } from "../lib/treeFilterStore";
import {
  folderCount,
  folderEntries,
  groupByFolder,
  groupByKind,
  groupByObjective,
  groupByPlan,
  groupDate,
  groupSort,
  sortEntries,
  type FolderGroup,
  type GroupSort,
} from "../lib/treeSort";
import { useTreeSort } from "../lib/treeSortStore";
import { useReloadToken } from "../lib/useReloadToken";
import { SkeletonLines } from "./Skeleton";
import { TreeToolbar } from "./TreeToolbar";

export function Tree({ onResetLayout }: { onResetLayout?: () => void }) {
  const [entries, setEntries] = useState<TreeEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { key: sortKey } = useTreeSort();
  const { hideCompleted, grouped } = useTreeFilter();
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

  const plannedAnalyses = plansByAnalysis(entries ?? []);
  const gsort = groupSort(sortKey);
  // task→plan index for hide filtering, plan dots and task-group header dots
  const taskIndex = useMemo(() => tasksByPlan(entries ?? []), [entries]);
  const visibleEntries =
    entries && hideCompleted
      ? entries.filter((entry) => !entryCompleted(entry, taskIndex))
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
                <span className="tree-folder__text">
                  <span className="tree-folder__title-row">
                    <span className="tree-folder__label">{kindLabel(group.kind)}</span>
                    <span className="tree-folder__count">{group.entries.length}</span>
                  </span>
                </span>
              </summary>
              {group.entries.length === 0 ? (
                <p className="content__empty tree-empty">No documents.</p>
              ) : group.kind === "analysis" ? (
                <AnalysisEntries
                  entries={group.entries}
                  plannedAnalyses={plannedAnalyses}
                  taskIndex={taskIndex}
                  sort={gsort}
                  grouped={grouped}
                />
              ) : group.kind === "plan" ? (
                <PlanKindEntries
                  entries={group.entries}
                  plannedAnalyses={plannedAnalyses}
                  taskIndex={taskIndex}
                  sort={gsort}
                  grouped={grouped}
                />
              ) : group.kind === "wiki" ? (
                <WikiEntries
                  entries={group.entries}
                  plannedAnalyses={plannedAnalyses}
                  taskIndex={taskIndex}
                  sort={gsort}
                />
              ) : group.kind === "tasks" ? (
                <TaskKindEntries
                  entries={group.entries}
                  plans={planIndex}
                  plannedAnalyses={plannedAnalyses}
                  taskIndex={taskIndex}
                  sort={gsort}
                  grouped={grouped}
                />
              ) : group.kind === "notes" ? (
                <NotesKindEntries entries={group.entries} sort={gsort} grouped={grouped} />
              ) : (
                <EntryList
                  entries={group.entries}
                  plannedAnalyses={plannedAnalyses}
                  taskIndex={taskIndex}
                />
              )}
            </details>
          ))}
        </div>
      )}
    </aside>
  );
}

type PlannedAnalyses = ReturnType<typeof plansByAnalysis>;

/** Flat entry list for a kind folder. The dot indexes are optional: kinds
 *  without a derived dot (notes) pass neither. */
function EntryList({
  entries,
  plannedAnalyses,
  taskIndex,
}: {
  entries: TreeEntry[];
  plannedAnalyses?: PlannedAnalyses;
  taskIndex?: Map<string, TreeEntry[]>;
}) {
  return (
    <ul role="list">
      {entries.map((entry) => (
        <TreeEntryRow
          key={entry.path}
          entry={entry}
          plannedAnalyses={plannedAnalyses}
          taskIndex={taskIndex}
        />
      ))}
    </ul>
  );
}

/** Wiki kind folder: entries at the wiki root stay flat, deeper entries nest
 *  under path-derived folder subgroups. */
function WikiEntries({
  entries,
  plannedAnalyses,
  taskIndex,
  sort,
}: {
  entries: TreeEntry[];
  plannedAnalyses: PlannedAnalyses;
  taskIndex: Map<string, TreeEntry[]>;
  sort: GroupSort | null;
}) {
  const { rootEntries, folders } = groupByFolder(entries, "context/wiki/", sort);
  return (
    <>
      {rootEntries.length > 0 && (
        <EntryList entries={rootEntries} plannedAnalyses={plannedAnalyses} taskIndex={taskIndex} />
      )}
      {folders.map((folder) => (
        <FolderNodeView
          key={folder.path}
          node={folder}
          plannedAnalyses={plannedAnalyses}
          taskIndex={taskIndex}
        />
      ))}
    </>
  );
}

/** One path-derived wiki folder (recursive), with a subtree count badge. */
function FolderNodeView({
  node,
  plannedAnalyses,
  taskIndex,
}: {
  node: FolderGroup;
  plannedAnalyses: PlannedAnalyses;
  taskIndex: Map<string, TreeEntry[]>;
}) {
  return (
    <details className="tree-folder tree-folder--dir">
      <summary className="tree-folder__header">
        <Icon name="expand_more" className="tree-folder__chevron" />
        <Icon name="folder" className="tree-folder__icon" />
        <span className="tree-folder__text">
          <span className="tree-folder__title-row">
            <span className="tree-folder__label">{node.name}</span>
            <span className="tree-folder__count">{folderCount(node)}</span>
          </span>
          <GroupHeaderDate entries={folderEntries(node)} />
        </span>
      </summary>
      {node.entries.length > 0 && (
        <EntryList entries={node.entries} plannedAnalyses={plannedAnalyses} taskIndex={taskIndex} />
      )}
      {node.children.map((child) => (
        <FolderNodeView
          key={child.path}
          node={child}
          plannedAnalyses={plannedAnalyses}
          taskIndex={taskIndex}
        />
      ))}
    </details>
  );
}

/** Latest created date for a group header; hidden when the group has no dates. */
function GroupHeaderDate({ entries }: { entries: TreeEntry[] }) {
  const date = groupDate(entries);
  if (!date) return null;
  return <span className="tree-folder__date">{formatFieldDate(date)}</span>;
}

/** Status dot shared by plan and objective group headers. */
function GroupProgressDot({ dot }: { dot: StatusDot | null }) {
  if (!dot) return null;
  return (
    <span
      className={`tree-folder__dot tree-folder__dot--${dot.tone}`}
      title={dot.label}
      aria-label={dot.label}
      role="img"
    />
  );
}

function taskGroupDot(entries: TreeEntry[]): StatusDot {
  const progress = taskProgress(entries);
  return { tone: progress.tone, label: taskProgressLabel(progress) };
}

/** Aggregate task-progress dot for a group of plans (none started / partial /
 *  all done), derived from each plan's referenced tasks. */
function plansGroupDot(plans: TreeEntry[], taskIndex: Map<string, TreeEntry[]>): StatusDot | null {
  const tasks = plans.flatMap((plan) => taskIndex.get(normalizeRef(plan.path)) ?? []);
  if (tasks.length === 0) return null;
  return taskGroupDot(tasks);
}

/** One `objective` sub-folder inside a kind folder: label, aggregate progress
 *  dot, entry count and the latest date. `dot` is null for kinds without a
 *  derived aggregate (notes). */
function ObjectiveFolder({
  objective,
  dot,
  entries,
  children,
}: {
  objective: string;
  dot: StatusDot | null;
  entries: TreeEntry[];
  children: ReactNode;
}) {
  return (
    <details className="tree-folder tree-folder--objective">
      <summary className="tree-folder__header">
        <Icon name="expand_more" className="tree-folder__chevron" />
        <Icon name="flag" className="tree-folder__icon" />
        <span className="tree-folder__text">
          <span className="tree-folder__title-row">
            <span className="tree-folder__label">{objective}</span>
            <GroupProgressDot dot={dot} />
            <span className="tree-folder__count">{entries.length}</span>
          </span>
          <GroupHeaderDate entries={entries} />
        </span>
      </summary>
      {children}
    </details>
  );
}

/** Plan kind folder: named `objective` sub-folders; plans without an objective
 *  stay at the folder root. Flat list when grouping is off. */
function PlanKindEntries({
  entries,
  plannedAnalyses,
  taskIndex,
  sort,
  grouped,
}: {
  entries: TreeEntry[];
  plannedAnalyses: PlannedAnalyses;
  taskIndex: Map<string, TreeEntry[]>;
  sort: GroupSort | null;
  grouped: boolean;
}) {
  if (!grouped) {
    return <EntryList entries={entries} plannedAnalyses={plannedAnalyses} taskIndex={taskIndex} />;
  }
  return (
    <>
      {groupByObjective(entries, sort).map((group) =>
        group.objective === "" ? (
          <EntryList
            key="__ungrouped"
            entries={group.entries}
            plannedAnalyses={plannedAnalyses}
            taskIndex={taskIndex}
          />
        ) : (
          <ObjectiveFolder
            key={group.objective}
            objective={group.objective}
            dot={plansGroupDot(group.entries, taskIndex)}
            entries={group.entries}
          >
            <EntryList
              entries={group.entries}
              plannedAnalyses={plannedAnalyses}
              taskIndex={taskIndex}
            />
          </ObjectiveFolder>
        ),
      )}
    </>
  );
}

/** Task kind folder: named `objective` sub-folders (inherited from the plan)
 *  with tasks nested under their plan inside each one; tasks without an
 *  objective stay at the folder root and keep the plan nesting. Flat list when
 *  grouping is off. */
function TaskKindEntries({
  entries,
  plans,
  plannedAnalyses,
  taskIndex,
  sort,
  grouped,
}: {
  entries: TreeEntry[];
  plans: Map<string, TreeEntry>;
  plannedAnalyses: PlannedAnalyses;
  taskIndex: Map<string, TreeEntry[]>;
  sort: GroupSort | null;
  grouped: boolean;
}) {
  if (!grouped) {
    return <EntryList entries={entries} plannedAnalyses={plannedAnalyses} taskIndex={taskIndex} />;
  }
  const withObjective = withTaskObjectives(entries, plans);
  return (
    <>
      {groupByObjective(withObjective, sort).map((group) =>
        group.objective === "" ? (
          <PlanEntries
            key="__ungrouped"
            entries={group.entries}
            plans={plans}
            plannedAnalyses={plannedAnalyses}
            taskIndex={taskIndex}
            sort={sort}
          />
        ) : (
          <ObjectiveFolder
            key={group.objective}
            objective={group.objective}
            dot={taskGroupDot(group.entries)}
            entries={group.entries}
          >
            <PlanEntries
              entries={group.entries}
              plans={plans}
              plannedAnalyses={plannedAnalyses}
              taskIndex={taskIndex}
              sort={sort}
            />
          </ObjectiveFolder>
        ),
      )}
    </>
  );
}

/** Task kind folder: tasks grouped under the plan they reference, tasks
 *  without a plan reference stay at the folder root. */
function PlanEntries({
  entries,
  plans,
  plannedAnalyses,
  taskIndex,
  sort,
}: {
  entries: TreeEntry[];
  plans: Map<string, TreeEntry>;
  plannedAnalyses: PlannedAnalyses;
  taskIndex: Map<string, TreeEntry[]>;
  sort: GroupSort | null;
}) {
  return (
    <>
      {groupByPlan(entries, plans, sort).map((group) =>
        group.plan === "" ? (
          <EntryList
            key="__ungrouped"
            entries={group.entries}
            plannedAnalyses={plannedAnalyses}
            taskIndex={taskIndex}
          />
        ) : (
          <details key={group.plan} className="tree-folder tree-folder--plan">
            <summary className="tree-folder__header">
              <Icon name="expand_more" className="tree-folder__chevron" />
              <Icon name="map" className="tree-folder__icon" />
              <span className="tree-folder__text">
                <span className="tree-folder__title-row">
                  <span className="tree-folder__label">{group.label}</span>
                  <GroupProgressDot dot={taskGroupDot(group.entries)} />
                  <span className="tree-folder__count">{group.entries.length}</span>
                </span>
                <GroupHeaderDate entries={group.entries} />
              </span>
            </summary>
            <EntryList
              entries={group.entries}
              plannedAnalyses={plannedAnalyses}
              taskIndex={taskIndex}
            />
          </details>
        ),
      )}
    </>
  );
}

/** Analysis kind folder: named `objective` sub-folders, entries without an
 *  objective stay at the folder root. Flat list when grouping is off. */
function AnalysisEntries({
  entries,
  plannedAnalyses,
  taskIndex,
  sort,
  grouped,
}: {
  entries: TreeEntry[];
  plannedAnalyses: PlannedAnalyses;
  taskIndex: Map<string, TreeEntry[]>;
  sort: GroupSort | null;
  grouped: boolean;
}) {
  if (!grouped) {
    return <EntryList entries={entries} plannedAnalyses={plannedAnalyses} taskIndex={taskIndex} />;
  }
  return (
    <>
      {groupByObjective(entries, sort).map((group) =>
        group.objective === "" ? (
          <EntryList
            key="__ungrouped"
            entries={group.entries}
            plannedAnalyses={plannedAnalyses}
            taskIndex={taskIndex}
          />
        ) : (
          <ObjectiveFolder
            key={group.objective}
            objective={group.objective}
            dot={groupDot(group.entries, plannedAnalyses, taskIndex)}
            entries={group.entries}
          >
            <EntryList
              entries={group.entries}
              plannedAnalyses={plannedAnalyses}
              taskIndex={taskIndex}
            />
          </ObjectiveFolder>
        ),
      )}
    </>
  );
}

/** Notes kind folder: named `objective` sub-folders (mostly dead-end notes tied
 *  to their objective, as reindex buckets them), notes without an objective stay
 *  at the folder root. Flat list when grouping is off. No progress dot: notes
 *  carry no derived aggregate. */
function NotesKindEntries({
  entries,
  sort,
  grouped,
}: {
  entries: TreeEntry[];
  sort: GroupSort | null;
  grouped: boolean;
}) {
  if (!grouped) {
    return <EntryList entries={entries} />;
  }
  return (
    <>
      {groupByObjective(entries, sort).map((group) =>
        group.objective === "" ? (
          <EntryList key="__ungrouped" entries={group.entries} />
        ) : (
          <ObjectiveFolder
            key={group.objective}
            objective={group.objective}
            dot={null}
            entries={group.entries}
          >
            <EntryList entries={group.entries} />
          </ObjectiveFolder>
        ),
      )}
    </>
  );
}

function TreeEntryRow({
  entry,
  plannedAnalyses,
  taskIndex,
}: {
  entry: TreeEntry;
  plannedAnalyses?: PlannedAnalyses;
  taskIndex?: Map<string, TreeEntry[]>;
}) {
  const dot = statusDot(entry, plannedAnalyses ?? new Map(), taskIndex ?? new Map());
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
