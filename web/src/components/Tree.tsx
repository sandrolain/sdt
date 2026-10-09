import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { NavLink, useLocation } from "react-router-dom";
import { fetchTree, type TreeEntry } from "../lib/api";
import { MAP_ICON } from "../lib/documentModes";
import { formatFieldDate, formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import { entryKind, kindColor, kindIcon, kindLabel } from "../lib/kinds";
import { categoryColor, categoryIcon } from "../lib/categories";
import {
  entryState,
  groupDot,
  normalizeRef,
  plansByAnalysis,
  stateMeta,
  taskObjective,
  taskProgress,
  taskProgressLabel,
  tasksByPlan,
  withTaskObjectives,
  type StatusDot,
} from "../lib/statusDot";
import { useRecentDocuments } from "../lib/readingState";
import { togglePin, usePinnedDocs } from "../lib/pinnedDocs";
import { displayTitle, filenameDate, objectiveLabel } from "../lib/titles";
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
import { useTreeKeyboard } from "../lib/treeKeyboard";
import { useTreeFocusRequests } from "../lib/treeFocus";
import { useReloadToken } from "../lib/useReloadToken";
import { SkeletonLines } from "./Skeleton";
import { TreeStatusBar } from "./TreeStatusBar";
import { TreeToolbar } from "./TreeToolbar";

/** Stable ids for the tree's collapsible groups, so one controlled open-state
 *  set drives every kind folder and every sub-folder (kind, objective, plan,
 *  wiki dir). */
const kindGroupId = (kind: string) => `kind:${kind}`;
const objectiveGroupId = (kind: string, objective: string) => `objective:${kind}:${objective}`;
const planGroupId = (plan: string) => `plan:${plan}`;
const dirGroupId = (path: string) => `dir:${path}`;

/** The recent-documents folder shares the controlled open-state with the rest. */
const RECENT_GROUP_ID = "recent";

/** Wiki folder ids on the path to an entry, outermost first. */
function wikiAncestorIds(path: string): string[] {
  const prefix = "context/wiki/";
  if (!path.startsWith(prefix)) return [];
  const segments = path.slice(prefix.length).split("/").filter(Boolean);
  segments.pop();
  const ids: string[] = [];
  let rel = "";
  for (const segment of segments) {
    rel = rel ? `${rel}/${segment}` : segment;
    ids.push(dirGroupId(rel));
  }
  return ids;
}

/** Every group id on the path to an entry, outermost first. */
function ancestorGroupIds(entry: TreeEntry, plans: Map<string, TreeEntry>): string[] {
  const kind = entryKind(entry);
  if (kind === "wiki") return [kindGroupId(kind), ...wikiAncestorIds(entry.path)];
  const ids = [kindGroupId(kind)];
  const objective = kind === "tasks" ? taskObjective(entry, plans) : entry.objective;
  if (objective) ids.push(objectiveGroupId(kind, objective));
  if (kind === "tasks" && entry.plan) ids.push(planGroupId(entry.plan));
  return ids;
}

/** One controlled open-state model for the tree: the id set of the open groups
 *  and the setter every `<details>` reads. Collapse-all clears the set; the
 *  reveal effect adds the active entry's ancestors. */
const GroupOpenContext = createContext<{
  open: ReadonlySet<string>;
  setOpen: (id: string, open: boolean) => void;
}>({ open: new Set(), setOpen: () => {} });

function useGroupOpen() {
  return useContext(GroupOpenContext);
}

export function Tree({ onResetLayout }: { onResetLayout?: () => void }) {
  const [entries, setEntries] = useState<TreeEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { key: sortKey } = useTreeSort();
  const { hiddenStates, groupMode, hideEmpty } = useTreeFilter();
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set());
  const reloadToken = useReloadToken();
  const location = useLocation();
  const groupsRef = useRef<HTMLDivElement | null>(null);
  const focusRequests = useTreeFocusRequests();
  useTreeKeyboard(groupsRef, focusRequests, entries);

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

  // plan lookup for task grouping (labels/order) and for the reveal effect;
  // uses every entry so a filtered-out completed plan still labels its tasks.
  const planIndex = useMemo(() => {
    const map = new Map<string, TreeEntry>();
    for (const entry of entries ?? []) {
      if (entryKind(entry) === "plan") map.set(normalizeRef(entry.path), entry);
    }
    return map;
  }, [entries]);

  const setGroupOpen = useCallback((id: string, open: boolean) => {
    setOpenGroups((prev) => {
      const next = new Set(prev);
      if (open) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  // The document currently open in the documents route.
  const activeEntry = useMemo(() => {
    if (!entries || !location.pathname.startsWith("/docs/")) return null;
    const path = decodeURIComponent(location.pathname.slice("/docs/".length));
    return entries.find((entry) => entry.path === path) ?? null;
  }, [entries, location.pathname]);

  // Reaching an active document opens the groups on its path, so a grouped
  // entry is visible without the folder forcing itself open (the old behaviour
  // made the header un-collapsible). Derived once per path during render; a
  // later manual collapse is not undone, and changing groupMode never
  // re-opens a group on its own.
  const [revealedPath, setRevealedPath] = useState<string | null>(null);
  if (activeEntry && activeEntry.path !== revealedPath) {
    setRevealedPath(activeEntry.path);
    const ids = ancestorGroupIds(activeEntry, planIndex);
    setOpenGroups((prev) => {
      const next = new Set(prev);
      for (const id of ids) next.add(id);
      return next;
    });
  }

  // Scroll the revealed row into view after the group opens.
  useEffect(() => {
    if (!activeEntry) return;
    document
      .querySelector(".panel--tree .tree-entry.is-active")
      ?.scrollIntoView({ block: "nearest" });
  }, [activeEntry]);

  const groupOpen = useMemo(
    () => ({ open: openGroups, setOpen: setGroupOpen }),
    [openGroups, setGroupOpen],
  );

  const plannedAnalyses = plansByAnalysis(entries ?? []);
  const gsort = groupSort(sortKey);
  // task→plan index for hide filtering, plan dots and task-group header dots
  const taskIndex = useMemo(() => tasksByPlan(entries ?? []), [entries]);
  // the filter runs on the same state the dot renders, so an entry can never
  // look completed and stay visible at the same time
  const visibleEntries =
    entries && hiddenStates.length > 0
      ? entries.filter(
          (entry) => !hiddenStates.includes(entryState(entry, plannedAnalyses, taskIndex).key),
        )
      : entries;
  const groups = visibleEntries
    ? groupByKind(visibleEntries)
        .map((group) => ({
          kind: group.kind,
          entries: sortEntries(group.entries, sortKey),
        }))
        .filter((group) => !hideEmpty || group.entries.length > 0)
    : [];
  // the status row's third count: visible documents whose effective state is in
  // the `open` lifecycle family (the same family the toolbar groups by)
  const openCount = visibleEntries
    ? visibleEntries.filter(
        (entry) => stateMeta[entryState(entry, plannedAnalyses, taskIndex).key].family === "open",
      ).length
    : 0;
  return (
    <aside className="panel panel--tree" aria-label="Corpus tree">
      <TreeToolbar onResetLayout={onResetLayout} onCollapseAll={() => setOpenGroups(new Set())} />
      <GroupOpenContext.Provider value={groupOpen}>
        {error ? (
          <p className="content__empty">Tree error: {error}</p>
        ) : !entries ? (
          <SkeletonLines count={6} label="Loading tree" />
        ) : (
          <div className="tree-groups" ref={groupsRef}>
            <PinnedDocs entries={entries} />
            <RecentDocs
              entries={entries}
              open={openGroups.has(RECENT_GROUP_ID)}
              onToggle={(open) => setGroupOpen(RECENT_GROUP_ID, open)}
            />
            {groupMode === "flat" ? (
              <div className="tree-flat">
                <EntryList
                  entries={visibleEntries ? sortEntries(visibleEntries, sortKey) : []}
                  plannedAnalyses={plannedAnalyses}
                  taskIndex={taskIndex}
                />
              </div>
            ) : (
              groups.map((group) => (
                <details
                  key={group.kind}
                  className="tree-folder"
                  open={openGroups.has(kindGroupId(group.kind))}
                  onToggle={(e) => setGroupOpen(kindGroupId(group.kind), e.currentTarget.open)}
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
                      grouped={groupMode === "full"}
                    />
                  ) : group.kind === "plan" ? (
                    <PlanKindEntries
                      entries={group.entries}
                      plannedAnalyses={plannedAnalyses}
                      taskIndex={taskIndex}
                      sort={gsort}
                      grouped={groupMode === "full"}
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
                      grouped={groupMode === "full"}
                    />
                  ) : group.kind === "notes" ? (
                    <NotesKindEntries
                      entries={group.entries}
                      sort={gsort}
                      grouped={groupMode === "full"}
                    />
                  ) : (
                    <EntryList
                      entries={group.entries}
                      plannedAnalyses={plannedAnalyses}
                      taskIndex={taskIndex}
                    />
                  )}
                </details>
              ))
            )}
          </div>
        )}
      </GroupOpenContext.Provider>
      {entries && !error && (
        <TreeStatusBar
          shown={visibleEntries?.length ?? 0}
          total={entries.length}
          open={openCount}
        />
      )}
    </aside>
  );
}

type PlannedAnalyses = ReturnType<typeof plansByAnalysis>;

/** Pinned documents, shown at the top of the tree; pin order, newest first. */
function PinnedDocs({ entries }: { entries: TreeEntry[] }) {
  const pinnedPaths = usePinnedDocs();
  const byPath = useMemo(() => new Map(entries.map((entry) => [entry.path, entry])), [entries]);
  const rows = pinnedPaths
    .map((path) => byPath.get(path))
    .filter((e): e is TreeEntry => e !== undefined);
  if (rows.length === 0) return null;
  return (
    <details className="tree-folder tree-folder--pinned" open>
      <summary className="tree-folder__header">
        <Icon name="expand_more" className="tree-folder__chevron" />
        <Icon name="push_pin" className="tree-folder__icon" title="Pinned" />
        <span className="tree-folder__text">
          <span className="tree-folder__title-row">
            <span className="tree-folder__label">Pinned</span>
            <span className="tree-folder__count">{rows.length}</span>
          </span>
        </span>
      </summary>
      <ul role="list">
        {rows.map((entry) => (
          <TreeEntryRow key={entry.path} entry={entry} />
        ))}
      </ul>
    </details>
  );
}

/**
 * The most recently read documents, in reading order. It reuses the tree row
 * styling but skips the state dot and the plan nesting: the point is the
 * shortest route back to what was being read, not a second tree.
 */
function RecentDocs({
  entries,
  open,
  onToggle,
}: {
  entries: TreeEntry[];
  open: boolean;
  onToggle: (open: boolean) => void;
}) {
  const recent = useRecentDocuments();
  const byPath = useMemo(() => new Map(entries.map((entry) => [entry.path, entry])), [entries]);
  // a document removed from the corpus drops out of the list
  const rows = recent
    .map((path) => byPath.get(path))
    .filter((e): e is TreeEntry => e !== undefined);
  if (rows.length === 0) return null;
  return (
    <details
      className="tree-folder tree-folder--recent"
      open={open}
      onToggle={(e) => onToggle(e.currentTarget.open)}
    >
      <summary className="tree-folder__header">
        <Icon name="expand_more" className="tree-folder__chevron" />
        <Icon name="history" className="tree-folder__icon" title="Recently read" />
        <span className="tree-folder__text">
          <span className="tree-folder__title-row">
            <span className="tree-folder__label">Recent</span>
            <span className="tree-folder__count">{rows.length}</span>
          </span>
        </span>
      </summary>
      <ul role="list">
        {rows.map((entry) => (
          <TreeEntryRow key={entry.path} entry={entry} />
        ))}
      </ul>
    </details>
  );
}

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
  const { open, setOpen } = useGroupOpen();
  const groupId = dirGroupId(node.path);
  return (
    <details
      className="tree-folder tree-folder--dir"
      open={open.has(groupId)}
      onToggle={(event) => setOpen(groupId, event.currentTarget.open)}
    >
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
  groupId,
  dot,
  entries,
  children,
}: {
  objective: string;
  groupId: string;
  dot: StatusDot | null;
  entries: TreeEntry[];
  children: ReactNode;
}) {
  const { open, setOpen } = useGroupOpen();
  return (
    <details
      className="tree-folder tree-folder--objective"
      open={open.has(groupId)}
      onToggle={(event) => setOpen(groupId, event.currentTarget.open)}
    >
      <summary className="tree-folder__header">
        <Icon name="expand_more" className="tree-folder__chevron" />
        <Icon name="flag" className="tree-folder__icon" />
        <span className="tree-folder__text">
          <span className="tree-folder__title-row">
            <span className="tree-folder__label">{objectiveLabel(objective)}</span>
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
            groupId={objectiveGroupId("plan", group.objective)}
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
            groupId={objectiveGroupId("tasks", group.objective)}
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
  const { open, setOpen } = useGroupOpen();
  return (
    <>
      {groupByPlan(entries, plans, sort).map((group) => {
        const groupId = planGroupId(group.plan);
        return group.plan === "" ? (
          <EntryList
            key="__ungrouped"
            entries={group.entries}
            plannedAnalyses={plannedAnalyses}
            taskIndex={taskIndex}
          />
        ) : (
          <details
            key={group.plan}
            className="tree-folder tree-folder--plan"
            open={open.has(groupId)}
            onToggle={(event) => setOpen(groupId, event.currentTarget.open)}
          >
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
        );
      })}
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
            groupId={objectiveGroupId("analysis", group.objective)}
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
            groupId={objectiveGroupId("notes", group.objective)}
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
  const state = entryState(entry, plannedAnalyses ?? new Map(), taskIndex ?? new Map());
  const dot = state.key === "no-state" ? null : { tone: state.tone, label: state.label };
  const kind = entryKind(entry);
  const kindName = kindLabel(kind);
  const pinnedPaths = usePinnedDocs();
  const pinned = pinnedPaths.includes(entry.path);
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
        {entry.categories?.[0] && (
          <Icon
            name={categoryIcon(entry.categories[0])}
            className="tree-entry__category"
            style={{ color: categoryColor(entry.categories[0]) }}
            title={entry.categories[0]}
          />
        )}
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
        <button
          type="button"
          className={`tree-entry__pin${pinned ? " is-pinned" : ""}`}
          aria-pressed={pinned}
          title={pinned ? "Unpin" : "Pin to top"}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            togglePin(entry.path);
          }}
        >
          <Icon name="push_pin" />
        </button>
        {state.drift && (
          <Icon
            name="warning"
            className="tree-entry__warn"
            label={state.drift}
            title={state.drift}
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

/** Small date line: frontmatter `created` and `modified`, else the filename date. */
function entryDate(e: TreeEntry): string {
  const created = e.created || filenameDate(e.path);
  const parts: string[] = [];
  if (created) parts.push(formatFieldDateOnly(created));
  if (e.modified && e.modified !== created) parts.push(formatFieldDateOnly(e.modified));
  return parts.join(" · ");
}
