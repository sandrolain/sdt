// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Tree } from "./Tree";
import { resetTreeSort, setTreeSortKey } from "../lib/treeSortStore";
import { resetTreeFilter, setGroupMode, setHiddenStates } from "../lib/treeFilterStore";
import { clearReadingState, recordReading } from "../lib/readingState";

const TREE = {
  entries: [
    { path: "context/wiki/zeta.md", kind: "wiki", title: "Zeta", created: "2026-09-02" },
    { path: "context/wiki/alpha.md", kind: "wiki", title: "Alpha", created: "2026-09-01" },
    { path: "context/notes/20260915-195559-note.md", kind: "notes", title: "Note" },
    { path: "context/plan/p.md", kind: "plan", title: "Plan", status: "active" },
    { path: "context/tasks/t.md", kind: "tasks", title: "Task", status: "in-progress" },
    { path: "context/analysis/a.md", kind: "analysis", title: "Analysis" },
    { path: "context/prompts/p.md", kind: "prompt", title: "Prompt" },
    { path: "context/wiki/topic.map.md", kind: "wiki", title: "Topic map", isMap: true },
  ],
};

function renderTree() {
  render(
    <MemoryRouter>
      <Tree />
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  resetTreeSort();
  resetTreeFilter();
  clearReadingState();
  vi.restoreAllMocks();
});

describe("Tree", () => {
  it("renders kind folders with counts and icons", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Alpha")).toBeTruthy();
    // folder headers carry the human kind label (no per-entry badges anymore)
    expect(screen.getAllByText("Wiki").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Notes").length).toBeGreaterThan(0);
    // per-entry kind badges are gone from the rows
    expect(document.querySelector(".tree-entry__kind")).toBeNull();
    // folder counts live in the folder header
    const headers = Array.from(document.querySelectorAll(".tree-folder__header"));
    const countFor = (label: string) =>
      headers
        .find((h) => h.querySelector(".tree-folder__label")?.textContent === label)
        ?.querySelector(".tree-folder__count")?.textContent;
    expect(countFor("Wiki")).toBe("3");
    expect(countFor("Notes")).toBe("1");
    // empty kind folders render at count 0 (questions/proposals/research/…)
    expect(countFor("Questions")).toBe("0");
    expect(countFor("Proposals")).toBe("0");
    expect(countFor("Commands")).toBe("0");
  });

  it("renders mermaid documents in their own folder, linked to the docs route", async () => {
    const data = {
      entries: [{ path: "context/wiki/flow.mmd", kind: "mermaid", title: "Flow", mermaid: true }],
    };
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(data) }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Flow")).toBeTruthy();
    const labels = Array.from(document.querySelectorAll(".tree-folder__label")).map(
      (h) => h.textContent,
    );
    expect(labels).toContain("Diagrams");
    expect(document.querySelector('a[href="/docs/context/wiki/flow.mmd"]')).toBeTruthy();
  });

  it("shows a No documents. message in an empty kind folder", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");
    const folder = (label: string) =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (h) => h.querySelector(".tree-folder__label")?.textContent === label,
      );
    expect(folder("Questions")?.querySelector(".tree-empty")?.textContent).toBe("No documents.");
    expect(folder("Wiki")?.querySelector(".tree-empty")).toBeNull();
  });

  it("shows status dots for plans, tasks and unplanned analyses", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Plan");
    // a plan without referenced tasks reads as not started
    expect(screen.getByLabelText("Plan not started")).toBeTruthy();
    expect(screen.getByLabelText("Task in progress")).toBeTruthy();
    expect(screen.getByLabelText("Analysis without a plan")).toBeTruthy();
  });

  it("defaults to created date descending", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");

    const titles = () => screen.getAllByRole("link").map((el) => el.textContent ?? "");
    // newest wiki entry (Zeta, created 2026-09-02) first; entries without a
    // created date stay last
    expect(titles()[0]).toContain("Zeta");
  });

  it("sorts by title ascending from the sort control", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");

    const titles = () => screen.getAllByRole("link").map((el) => el.textContent ?? "");
    await userEvent.click(screen.getByRole("button", { name: /Sort entries by/ }));
    await userEvent.click(await screen.findByRole("option", { name: "Title ASC" }));
    expect(titles()[0]).toContain("Alpha");
  });

  it("labels the sort, state and grouping controls", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");

    const labels = Array.from(document.querySelectorAll(".ui-field__label")).map(
      (el) => el.textContent,
    );
    expect(labels).toContain("Sort");
    expect(labels).toContain("States");
    expect(labels).toContain("Group");
    // the accessible names are unchanged: a visible label is added, not swapped
    expect(screen.getByRole("button", { name: /Sort entries by/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Visible states" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Grouping/ })).toBeTruthy();
  });

  it("groups prompt-kind entries under the prompts section", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Prompt");
    const headers = Array.from(document.querySelectorAll(".tree-folder__header"));
    const prompts = headers.find(
      (h) => h.querySelector(".tree-folder__label")?.textContent === "Prompts",
    );
    expect(prompts?.querySelector(".tree-folder__count")?.textContent).toBe("1");
  });

  it("marks .map.md entries with a coloured map icon, not a text chip", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Topic map");
    expect(screen.getByRole("img", { name: "Map document" })).toBeTruthy();
    expect(document.querySelector(".tree-entry__kind--map")).toBeNull();
  });

  it("hides every state the filter deselects, starting with the reported case", async () => {
    // The report: an analysis declared `active` whose plan and task files are
    // all completed. The dot is green, so deselecting Completed must hide it —
    // the filter reads the same state the dot renders.
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/done-a.md",
                kind: "analysis",
                title: "Done analysis",
                status: "completed",
              },
              {
                path: "context/analysis/open-api.md",
                kind: "analysis",
                title: "Open analysis",
              },
              {
                path: "context/analysis/stuck.md",
                kind: "analysis",
                plans: ["context/plan/p.md"],
                title: "Stuck analysis",
                status: "active",
              },
              {
                path: "context/analysis/resolved.md",
                kind: "analysis",
                title: "Resolved analysis",
                status: "resolved",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "completed",
                // the payload keeps the derivation prose and the resolved edge:
                // the plan's parent is `stuck`, and `resolved` has no plan
                analysis: "context/analysis/stuck.md",
                sources: ["analysis/stuck.md", "analysis/resolved.md"],
              },
              {
                path: "context/tasks/done.md",
                kind: "tasks",
                title: "Done task",
                status: "completed",
                plan: "context/plan/p.md",
              },
              {
                path: "context/tasks/wip.md",
                kind: "tasks",
                title: "Wip task",
                status: "in-progress",
              },
              { path: "context/questions/q.md", kind: "questions", title: "Answered" },
              {
                path: "context/questions/o.md",
                kind: "questions",
                title: "Open",
                status: "active",
              },
              { path: "context/notes/n.md", kind: "notes", title: "Note" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Stuck analysis")).toBeTruthy();
    // nothing hidden by default: the tree is the unfiltered one
    for (const title of [
      "Done analysis",
      "Open analysis",
      "Stuck analysis",
      "Resolved analysis",
      "Done task",
      "Wip task",
      "Answered",
      "Open",
      "Note",
    ]) {
      expect(screen.getByText(title)).toBeTruthy();
    }

    setHiddenStates(["completed"]);
    await waitFor(() => expect(screen.queryByText("Stuck analysis")).toBeNull());
    // every completed state goes: the declared-completed analysis, the plan, the
    // task file and the analysis whose plans are all done (the reported case)
    expect(screen.queryByText("Done analysis")).toBeNull();
    expect(screen.queryByText("Done task")).toBeNull();
    // out-of-vocabulary `resolved` reads as archived, so it stays
    expect(screen.getByText("Resolved analysis")).toBeTruthy();
    // and everything not completed stays
    for (const title of ["Open analysis", "Wip task", "Answered", "Open", "Note"]) {
      expect(screen.getByText(title)).toBeTruthy();
    }

    setHiddenStates([]);
    await waitFor(() => expect(screen.getByText("Stuck analysis")).toBeTruthy());
  });

  it("filters every kind by its own state, questions included", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/questions/q.md",
                kind: "questions",
                title: "Answered",
                status: "resolved",
              },
              {
                path: "context/questions/o.md",
                kind: "questions",
                title: "Open",
                status: "active",
              },
              { path: "context/wiki/w.md", kind: "wiki", title: "Live page", status: "active" },
              { path: "context/notes/n.md", kind: "notes", title: "Note" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Answered")).toBeTruthy();

    setHiddenStates(["resolved"]);
    await waitFor(() => expect(screen.queryByText("Answered")).toBeNull());
    expect(screen.getByText("Open")).toBeTruthy();
    expect(screen.getByText("Live page")).toBeTruthy();
    // a kind with no status at all is its own state
    setHiddenStates(["no-state"]);
    await waitFor(() => expect(screen.queryByText("Note")).toBeNull());
    expect(screen.getByText("Open")).toBeTruthy();

    // `active` is one state across kinds: it hides the open question and the
    // live wiki page together, and leaves the resolved question alone
    setHiddenStates(["active"]);
    await waitFor(() => expect(screen.queryByText("Live page")).toBeNull());
    expect(screen.queryByText("Open")).toBeNull();
    expect(screen.getByText("Answered")).toBeTruthy();
  });

  it("warns on an entry whose declared status disagrees with its effective state", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/done.md",
                kind: "analysis",
                plans: ["context/plan/p.md"],
                title: "Derived",
                status: "active",
              },
              {
                path: "context/analysis/odd.md",
                kind: "analysis",
                title: "Odd",
                status: "resolved",
              },
              {
                path: "context/analysis/later.md",
                kind: "analysis",
                title: "Later",
                status: "postponed",
              },
              {
                path: "context/analysis/clean.md",
                kind: "analysis",
                title: "Clean",
                status: "draft",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "completed",
                analysis: "context/analysis/done.md",
                sources: [
                  "analysis/done.md",
                  "analysis/odd.md",
                  "analysis/later.md",
                  "analysis/clean.md",
                ],
              },
              {
                path: "context/tasks/t.md",
                kind: "tasks",
                title: "Task",
                status: "completed",
                plan: "context/plan/p.md",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Derived")).toBeTruthy();
    // the derived-completed analysis keeps its green dot and gains the warning
    const row = document.querySelector('a[href="/docs/context/analysis/done.md"]');
    expect(row?.querySelector(".tree-entry__dot--ok")).toBeTruthy();
    expect(row?.querySelector(".tree-entry__warn")?.getAttribute("aria-label")).toBe(
      "analysis is derivably completed (all plans done) — run `sdt context sync`",
    );
    // the out-of-vocabulary value is reported too
    expect(
      document
        .querySelector('a[href="/docs/context/analysis/odd.md"] .tree-entry__warn')
        ?.getAttribute("aria-label"),
    ).toContain("declared status `resolved` is outside the analysis vocabulary");
    // a user-owned status and a coherent one stay silent
    expect(
      document.querySelector('a[href="/docs/context/analysis/later.md"] .tree-entry__warn'),
    ).toBeNull();
    expect(
      document.querySelector('a[href="/docs/context/analysis/clean.md"] .tree-entry__warn'),
    ).toBeNull();
  });

  it("offers every state of every kind in the toolbar and no not-completed switch", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Alpha")).toBeTruthy();
    expect(screen.queryByRole("switch", { name: "Not completed" })).toBeNull();
    // grouping defaults to "By type", shown in the select trigger
    expect(document.querySelector(".tree-grouping .ui-select__value")?.textContent).toBe("By type");

    // every state is selected by default, so the trigger reads "All"
    const trigger = screen.getByRole("button", { name: "Visible states" });
    expect(trigger.querySelector(".ui-select__value")?.textContent).toBe("All");
    await userEvent.click(trigger);
    // the five lifecycle families are section headers, each with a bulk action
    const headers = Array.from(document.querySelectorAll(".ui-select__section-header")).map(
      (el) => el.textContent ?? "",
    );
    for (const label of ["Open", "Concluded", "Deferred", "Retired", "Unclassified"]) {
      expect(headers.some((header) => header.includes(label))).toBe(true);
    }
    // the quick presets render as buttons
    for (const label of ["All", "Open", "Closed", "Deferred"]) {
      expect(screen.getByRole("button", { name: label })).toBeTruthy();
    }
    for (const label of ["Completed", "In progress", "Postponed", "No state", "No plan"]) {
      expect(screen.getByRole("option", { name: label })).toBeTruthy();
    }
    // deselecting Completed hides it and the summary counts the rest. The
    // trigger leaves the a11y tree while its popover is open, so close it first.
    await userEvent.click(screen.getByRole("option", { name: "Completed" }));
    await userEvent.keyboard("{Escape}");
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Visible states" }).querySelector(".ui-select__value")
          ?.textContent,
      ).toBe("22 selected"),
    );
  });

  it("shows a thumbnail for entries with a frontmatter image", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/wiki/cover.md",
                kind: "wiki",
                title: "Cover",
                image: "context/assets/cover.png",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Cover");
    const thumb = document.querySelector("img.tree-entry__thumb");
    expect(thumb?.getAttribute("src")).toBe("/api/file?path=context%2Fassets%2Fcover.png");
  });

  it("opens and reveals the active document's kind folder", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    render(
      <MemoryRouter initialEntries={["/docs/context/notes/20260915-195559-note.md"]}>
        <Tree />
      </MemoryRouter>,
    );
    await screen.findByText("Note");
    // the open document is highlighted, not just its folder opened
    await waitFor(() => {
      expect(document.querySelector(".tree-entry.is-active")?.textContent).toContain("Note");
    });
    const folder = (label: string) =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (h) => h.querySelector(".tree-folder__label")?.textContent === label,
      );
    await waitFor(() => {
      expect(folder("Notes")?.hasAttribute("open")).toBe(true);
    });
    expect(folder("Wiki")?.hasAttribute("open")).toBe(false);
  });

  it("starts kind folders collapsed", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");
    const folders = document.querySelectorAll(".tree-folder");
    expect(folders.length).toBeGreaterThan(0);
    for (const folder of folders) {
      expect(folder.hasAttribute("open")).toBe(false);
    }
  });

  it("reveals the active document's objective and plan groups", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "active",
                objective: "viewer",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "Task one",
                status: "pending",
                plan: "context/plan/p.md",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    setGroupMode("full");
    render(
      <MemoryRouter initialEntries={["/docs/context/tasks/t1.md"]}>
        <Tree />
      </MemoryRouter>,
    );
    await screen.findByText("Task one");
    const tasksFolder = () =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (folder) => folder.querySelector(".tree-folder__label")?.textContent === "Tasks",
      );
    await waitFor(() => {
      expect(tasksFolder()?.querySelector(".tree-folder--objective")?.hasAttribute("open")).toBe(
        true,
      );
      expect(tasksFolder()?.querySelector(".tree-folder--plan")?.hasAttribute("open")).toBe(true);
    });
  });

  it("collapses every open group from the toolbar", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "active",
                objective: "viewer",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "Task one",
                status: "pending",
                plan: "context/plan/p.md",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    setGroupMode("full");
    render(
      <MemoryRouter initialEntries={["/docs/context/tasks/t1.md"]}>
        <Tree />
      </MemoryRouter>,
    );
    await screen.findByText("Task one");
    const tasksFolder = () =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (folder) => folder.querySelector(".tree-folder__label")?.textContent === "Tasks",
      );
    await waitFor(() =>
      expect(tasksFolder()?.querySelector(".tree-folder--objective")?.hasAttribute("open")).toBe(
        true,
      ),
    );
    await userEvent.click(screen.getByRole("button", { name: "Collapse all" }));
    await waitFor(() => {
      for (const folder of document.querySelectorAll(".tree-folder")) {
        expect(folder.hasAttribute("open")).toBe(false);
      }
    });
  });

  it("does not reopen collapsed groups when the grouping mode changes", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "active",
                objective: "viewer",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "Task one",
                status: "pending",
                plan: "context/plan/p.md",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    setGroupMode("full");
    render(
      <MemoryRouter initialEntries={["/docs/context/tasks/t1.md"]}>
        <Tree />
      </MemoryRouter>,
    );
    await screen.findByText("Task one");
    await waitFor(() =>
      expect(document.querySelectorAll(".tree-folder[open]").length).toBeGreaterThan(0),
    );
    await userEvent.click(screen.getByRole("button", { name: "Collapse all" }));
    await waitFor(() => expect(document.querySelectorAll(".tree-folder[open]")).toHaveLength(0));
    setGroupMode("flat");
    await waitFor(() => expect(document.querySelectorAll(".tree-folder")).toHaveLength(0));
    setGroupMode("full");
    await waitFor(() =>
      expect(document.querySelectorAll(".tree-folder").length).toBeGreaterThan(0),
    );
    expect(document.querySelectorAll(".tree-folder[open]")).toHaveLength(0);
  });

  it("collapses the active kind folder on click, without forcing it open", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    render(
      <MemoryRouter initialEntries={["/docs/context/notes/20260915-195559-note.md"]}>
        <Tree />
      </MemoryRouter>,
    );
    await screen.findByText("Note");
    const notesFolder = () =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (folder) => folder.querySelector(".tree-folder__label")?.textContent === "Notes",
      );
    await waitFor(() => expect(notesFolder()?.hasAttribute("open")).toBe(true));
    await userEvent.click(notesFolder()!.querySelector("summary") as HTMLElement);
    await waitFor(() => expect(notesFolder()?.hasAttribute("open")).toBe(false));
  });

  it("uses the kind glyph, uncoloured, for entries without an image", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    const link = (await screen.findByText("Analysis")).closest(".tree-entry") as HTMLElement;
    const glyph = link.querySelector(".tree-entry__glyph");
    expect(glyph?.querySelector(".ms-icon")?.textContent).toBe("analytics");
    expect(glyph?.getAttribute("style")).toBeNull();
    // the hover title ends with the kind
    expect(link.getAttribute("title")).toBe("context/analysis/a.md · Analyses");
  });

  it("shows a date line from frontmatter or the filename prefix", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");
    // filename date prefix on the notes entry (locale-safe: no year assumption)
    expect(screen.getAllByText(/2026/).length).toBeGreaterThan(0);
  });

  it("nests wiki entries under path-derived folder subgroups", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              { path: "context/wiki/root.md", kind: "wiki", title: "Root" },
              { path: "context/wiki/regulations/cra.md", kind: "wiki", title: "CRA" },
              { path: "context/wiki/regulations/dora.md", kind: "wiki", title: "DORA" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Root");
    const dir = document.querySelector(".tree-folder--dir");
    expect(dir?.querySelector(".tree-folder__label")?.textContent).toBe("regulations");
    expect(dir?.querySelector(".tree-folder__count")?.textContent).toBe("2");
    expect(dir?.textContent).toContain("CRA");
    // the wiki-root entry stays outside the folder subgroup
    expect(dir?.textContent).not.toContain("Root");
  });

  it("groups tasks under their plan", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/plan/20260920-a-plan.md",
                kind: "plan",
                title: "Plan A",
                created: "2026-09-20",
                status: "active",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "T1",
                status: "pending",
                plan: "context/plan/20260920-a-plan.md",
              },
              { path: "context/tasks/t2.md", kind: "tasks", title: "T2", status: "pending" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("T1");
    // default grouping is By type; switch to the nested mode
    setGroupMode("full");
    await waitFor(() => expect(document.querySelector(".tree-folder--plan")).toBeTruthy());
    const plan = document.querySelector(".tree-folder--plan");
    expect(plan?.querySelector(".tree-folder__label")?.textContent).toBe("Plan A");
    expect(plan?.querySelector(".tree-folder__count")?.textContent).toBe("1");
    expect(plan?.textContent).toContain("T1");
    // the plan-less task stays at the tasks-folder root
    expect(plan?.textContent).not.toContain("T2");
  });

  it("renders a task-progress dot on the plan task-group header", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/plan/20260920-a-plan.md",
                kind: "plan",
                title: "Plan A",
                created: "2026-09-20",
                status: "active",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "T1 done",
                status: "completed",
                plan: "context/plan/20260920-a-plan.md",
              },
              {
                path: "context/tasks/t2.md",
                kind: "tasks",
                title: "T2 open",
                status: "pending",
                plan: "context/plan/20260920-a-plan.md",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("T1 done");
    // default grouping is By type; switch to the nested mode
    setGroupMode("full");
    await waitFor(() =>
      expect(document.querySelector(".tree-folder--plan .tree-folder__dot")).toBeTruthy(),
    );
    const dot = document.querySelector(".tree-folder--plan .tree-folder__dot");
    expect(dot?.classList.contains("tree-folder__dot--warn")).toBe(true);
    expect(dot?.getAttribute("aria-label")).toBe("1/2 tasks completed");
    // the plan entry row itself shows the same aggregate tone
    expect(screen.getByLabelText("Plan in progress")).toBeTruthy();
  });

  it("nests analyses under their objective folder", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/a.md",
                kind: "analysis",
                title: "Alpha",
                objective: "viewer",
              },
              {
                path: "context/analysis/b.md",
                kind: "analysis",
                title: "Beta",
                objective: "viewer",
              },
              { path: "context/analysis/c.md", kind: "analysis", title: "Gamma" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");
    // default grouping is By type; switch to the nested mode
    setGroupMode("full");
    await waitFor(() => expect(document.querySelector(".tree-folder--objective")).toBeTruthy());
    const objectiveFolders = Array.from(document.querySelectorAll(".tree-folder--objective"));
    expect(objectiveFolders).toHaveLength(1);
    // the folder shows a readable label; the grouping key stays the raw slug
    expect(objectiveFolders[0].querySelector(".tree-folder__label")?.textContent).toBe("Viewer");
    expect(objectiveFolders[0].querySelector(".tree-folder__count")?.textContent).toBe("2");
    expect(objectiveFolders[0].textContent).toContain("Alpha");
    expect(objectiveFolders[0].textContent).toContain("Beta");
    // the objective-less analysis stays at the analysis-folder root
    expect(objectiveFolders[0].textContent).not.toContain("Gamma");
  });

  it("aggregates objective dots over visible analyses and styles archived/question dots", async () => {
    // The state filter hides what the dot calls completed, including the
    // derived-completed analysis the old boolean left visible.
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/unplanned.md",
                kind: "analysis",
                title: "Unplanned",
                objective: "viewer",
                status: "active",
              },
              {
                path: "context/analysis/planned.md",
                kind: "analysis",
                plans: ["context/plan/p.md"],
                title: "Planned",
                objective: "viewer",
                status: "active",
              },
              {
                path: "context/analysis/archived.md",
                kind: "analysis",
                title: "Archived",
                objective: "viewer",
                status: "archived",
              },
              {
                path: "context/analysis/retired.md",
                kind: "analysis",
                title: "Retired",
                objective: "retired",
                status: "archived",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                sources: ["analysis/planned.md"],
              },
              {
                path: "context/tasks/t.md",
                kind: "tasks",
                status: "completed",
                plan: "context/plan/p.md",
              },
              {
                path: "context/questions/open.md",
                kind: "questions",
                title: "Open question",
                status: "active",
              },
              {
                path: "context/questions/resolved.md",
                kind: "questions",
                title: "Resolved question",
                status: "resolved",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();

    expect(await screen.findByText("Unplanned")).toBeTruthy();
    // default grouping is By type; switch to the nested mode
    setGroupMode("full");
    await waitFor(() => expect(document.querySelector(".tree-folder--objective")).toBeTruthy());
    const viewer = Array.from(document.querySelectorAll(".tree-folder--objective")).find(
      (folder) => folder.querySelector(".tree-folder__label")?.textContent === "Viewer",
    ) as HTMLElement;
    expect(viewer.querySelector(".tree-folder__label")?.textContent).toBe("Viewer");
    expect(viewer.querySelector(".tree-folder__dot--warn")?.getAttribute("aria-label")).toBe(
      "1/2 analyses completed",
    );
    expect(
      document.querySelector(
        'a[href="/docs/context/analysis/archived.md"] .tree-entry__dot--neutral',
      ),
    ).toBeTruthy();
    const retired = Array.from(document.querySelectorAll(".tree-folder--objective")).find(
      (folder) => folder.querySelector(".tree-folder__label")?.textContent === "Retired",
    );
    expect(retired?.querySelector(".tree-folder__dot")).toBeNull();
    expect(screen.getByLabelText("Question unresolved")).toBeTruthy();
    expect(screen.getByLabelText("Question resolved")).toBeTruthy();

    act(() => setHiddenStates(["completed", "archived"]));
    await waitFor(() => {
      // `planned` is completed in effect (its plan's tasks are all done) even
      // though it declares `active`, so it goes too: 3 analyses -> 1.
      expect(viewer.querySelector(".tree-folder__count")?.textContent).toBe("1");
      expect(document.querySelector('a[href="/docs/context/analysis/archived.md"]')).toBeNull();
      expect(document.querySelector('a[href="/docs/context/analysis/planned.md"]')).toBeNull();
      // the aggregate follows the filtered set: only the unplanned one is left
      expect(viewer.querySelector(".tree-folder__dot--danger")?.getAttribute("aria-label")).toBe(
        "0/1 analyses completed",
      );
      expect(
        Array.from(document.querySelectorAll(".tree-folder--objective")).some(
          (folder) => folder.querySelector(".tree-folder__label")?.textContent === "Retired",
        ),
      ).toBe(false);
    });
  });

  it("shows the latest created date in objective group headers and orders groups by it under a date sort", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/a.md",
                kind: "analysis",
                title: "Alpha",
                objective: "alpha",
                created: "2026-09-01",
              },
              {
                path: "context/analysis/b.md",
                kind: "analysis",
                title: "Zeta",
                objective: "zeta",
                created: "2026-09-20",
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");
    // default grouping is By type; switch to the nested mode
    setGroupMode("full");
    await waitFor(() => expect(document.querySelector(".tree-folder--objective")).toBeTruthy());
    const objectiveNames = () =>
      Array.from(document.querySelectorAll(".tree-folder--objective")).map(
        (f) => f.querySelector(".tree-folder__label")?.textContent,
      );
    // default created_desc: the group with the latest created comes first
    expect(objectiveNames()).toEqual(["Zeta", "Alpha"]);

    // header dates mirror the latest created, shown regardless of the sort
    const dateSpans = Array.from(document.querySelectorAll(".tree-folder__date"));
    expect(dateSpans).toHaveLength(2);
    expect(dateSpans[0].textContent).toContain("2026");

    // the date sits on its own line (outside the title row); the count stays in
    // the title row alongside the label
    const group = document.querySelector(".tree-folder--objective");
    expect(group?.querySelector(".tree-folder__title-row .tree-folder__date")).toBeNull();
    expect(group?.querySelector(".tree-folder__title-row .tree-folder__count")).toBeTruthy();
    const text = group?.querySelector(".tree-folder__text");
    expect(Array.from(text?.children ?? []).map((c) => c.className)).toEqual([
      "tree-folder__title-row",
      "tree-folder__date",
    ]);

    act(() => setTreeSortKey("name_asc"));
    await waitFor(() => expect(objectiveNames()).toEqual(["Alpha", "Zeta"]));
    // header dates remain under a name sort
    expect(document.querySelectorAll(".tree-folder__date")).toHaveLength(2);
  });

  it("groups plans and inherited tasks by objective with tasks nested under their plan", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/a.md",
                kind: "analysis",
                title: "Analysis",
                objective: "viewer",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "active",
                objective: "viewer",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "Task one",
                status: "pending",
                plan: "context/plan/p.md",
              },
              {
                path: "context/tasks/t2.md",
                kind: "tasks",
                title: "Task two",
                status: "completed",
                plan: "context/plan/p.md",
              },
              {
                path: "context/plan/other.md",
                kind: "plan",
                title: "Other plan",
                status: "active",
              },
              { path: "context/tasks/u.md", kind: "tasks", title: "Loose task", status: "pending" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Task one");
    // default grouping is By type; switch to the nested mode
    setGroupMode("full");
    await waitFor(() => expect(document.querySelector(".tree-folder--objective")).toBeTruthy());

    const folder = (label: string) =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (f) => f.querySelector(".tree-folder__label")?.textContent === label,
      );
    const planObjective = Array.from(
      folder("Plans")?.querySelectorAll(".tree-folder--objective") ?? [],
    ).find((f) => f.querySelector(".tree-folder__label")?.textContent === "Viewer");
    expect(planObjective?.textContent).toContain("Plan");
    // the objective-less plan stays at the plan-folder root
    expect(planObjective?.textContent).not.toContain("Other plan");

    const taskObjective = Array.from(
      folder("Tasks")?.querySelectorAll(".tree-folder--objective") ?? [],
    ).find((f) => f.querySelector(".tree-folder__label")?.textContent === "Viewer");
    const planSubGroup = taskObjective?.querySelector(".tree-folder--plan");
    expect(planSubGroup?.textContent).toContain("Task one");
    expect(planSubGroup?.textContent).toContain("Task two");
    // the loose task (no plan, no objective) stays at the task-folder root
    expect(folder("Tasks")?.textContent).toContain("Loose task");
  });

  it("flattens the objective folders in By type mode, keeping the wiki folders", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/a.md",
                kind: "analysis",
                title: "Analysis",
                objective: "viewer",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "active",
                objective: "viewer",
              },
              {
                path: "context/tasks/t1.md",
                kind: "tasks",
                title: "Task one",
                status: "pending",
                plan: "context/plan/p.md",
              },
              {
                path: "context/tasks/t2.md",
                kind: "tasks",
                title: "Task two",
                status: "completed",
                plan: "context/plan/p.md",
              },
              { path: "context/wiki/sub/deep.md", kind: "wiki", title: "Deep wiki" },
              { path: "context/notes/n.md", kind: "notes", title: "Note" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Analysis");
    // default grouping is By type: nest to exercise the objective folders
    setGroupMode("full");
    await waitFor(() =>
      expect(document.querySelectorAll(".tree-folder--objective")).toHaveLength(3),
    );
    // grouped on: the objective folders exist under analyses, plans and tasks
    expect(document.querySelectorAll(".tree-folder--objective")).toHaveLength(3);
    expect(document.querySelectorAll(".tree-folder--plan")).toHaveLength(1);

    // the two flags are independent: the state filter drops the done task, so
    // the task objective group counts 1 instead of 2
    const taskObjectiveCount = () => {
      const tasks = Array.from(document.querySelectorAll(".tree-folder")).find(
        (f) => f.querySelector(".tree-folder__label")?.textContent === "Tasks",
      );
      const objective = Array.from(tasks?.querySelectorAll(".tree-folder--objective") ?? []).find(
        (f) => f.querySelector(".tree-folder__label")?.textContent === "Viewer",
      );
      return objective?.querySelector(".tree-folder__count")?.textContent;
    };
    expect(taskObjectiveCount()).toBe("2");
    setHiddenStates(["completed"]);
    await waitFor(() => expect(taskObjectiveCount()).toBe("1"));

    setGroupMode("type");
    await waitFor(() =>
      expect(document.querySelectorAll(".tree-folder--objective")).toHaveLength(0),
    );
    // the plan nesting is gone too, and every entry is still listed
    expect(document.querySelectorAll(".tree-folder--plan")).toHaveLength(0);
    for (const title of ["Analysis", "Plan", "Task one", "Note"]) {
      expect(screen.getByText(title)).toBeTruthy();
    }
    // the wiki folder hierarchy mirrors the corpus and is never collapsed
    expect(document.querySelectorAll(".tree-folder--dir")).toHaveLength(1);
    expect(document.querySelector(".tree-folder--dir")?.textContent).toContain("Deep wiki");
  });

  it("renders a single flat list with no kind folders in Flat mode", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/a.md",
                kind: "analysis",
                title: "Analysis",
                objective: "viewer",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "Plan",
                status: "active",
                objective: "viewer",
              },
              { path: "context/wiki/sub/deep.md", kind: "wiki", title: "Deep wiki" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Analysis");
    setGroupMode("flat");
    await waitFor(() => expect(document.querySelectorAll(".tree-folder")).toHaveLength(0));
    for (const title of ["Analysis", "Plan", "Deep wiki"]) {
      expect(screen.getByText(title)).toBeTruthy();
    }
  });

  it("groups notes by objective when grouping is on and flattens them when off", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/notes/dead-end.md",
                kind: "notes",
                title: "Dead end",
                objective: "viewer",
              },
              { path: "context/notes/plain.md", kind: "notes", title: "Plain note" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Dead end");

    const notesFolder = () =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (f) => f.querySelector(".tree-folder__label")?.textContent === "Notes",
      );
    // default grouping is By type: nest to exercise the objective folders
    setGroupMode("full");
    await waitFor(() =>
      expect(notesFolder()?.querySelector(".tree-folder--objective")).toBeTruthy(),
    );

    const objective = Array.from(
      notesFolder()?.querySelectorAll(".tree-folder--objective") ?? [],
    ).find((f) => f.querySelector(".tree-folder__label")?.textContent === "Viewer");
    expect(objective?.querySelector(".tree-folder__count")?.textContent).toBe("1");
    expect(objective?.textContent).toContain("Dead end");
    // the objective-less note stays at the notes-folder root
    expect(objective?.textContent).not.toContain("Plain note");
    expect(notesFolder()?.textContent).toContain("Plain note");

    setGroupMode("type");
    await waitFor(() =>
      expect(document.querySelectorAll(".tree-folder--objective")).toHaveLength(0),
    );
    expect(notesFolder()?.textContent).toContain("Dead end");
    expect(notesFolder()?.textContent).toContain("Plain note");
  });
  it("shows one category icon per row", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/analysis/a.md",
                kind: "analysis",
                title: "A",
                status: "active",
                categories: ["bug", "refactor"],
              },
              { path: "context/analysis/b.md", kind: "analysis", title: "B", status: "active" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("A");
    const icons = Array.from(document.querySelectorAll(".tree-entry__category"));
    // one per row: the first category only, and none for a row without one
    expect(icons).toHaveLength(1);
    expect(icons[0].textContent).toBe("bug_report");
    expect(icons[0].getAttribute("title")).toBe("bug");
  });

  it("renders a status bar with visible, total and open counts that follow the filter", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              { path: "context/analysis/a.md", kind: "analysis", title: "A", status: "active" },
              { path: "context/analysis/b.md", kind: "analysis", title: "B", status: "completed" },
              { path: "context/notes/n.md", kind: "notes", title: "N" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("A");
    const bar = () => document.querySelector(".tree-status") as HTMLElement;
    expect(bar().textContent).toContain("3 shown");
    expect(bar().textContent).toContain("3 total");
    // the active analysis is the only open-family document; no-state and
    // completed are not
    expect(bar().textContent).toContain("1 open");

    // the counts describe the visible set: hiding completed drops one analysis
    act(() => setHiddenStates(["completed"]));
    await waitFor(() => expect(bar().textContent).toContain("2 shown"));
    expect(bar().textContent).toContain("3 total");
    expect(bar().textContent).toContain("1 open");
  });

  it("reads the open count from the effective state, not the declared status", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              // declared active, but every task is done: the effective state
              // concludes, so the plan is not open
              { path: "context/plan/p.md", kind: "plan", title: "P", status: "active" },
              {
                path: "context/tasks/t.md",
                kind: "tasks",
                title: "T",
                status: "completed",
                plan: "context/plan/p.md",
              },
              // declared active with no plan: stays open
              { path: "context/plan/q.md", kind: "plan", title: "Q", status: "active" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("P");
    const bar = document.querySelector(".tree-status") as HTMLElement;
    expect(bar.textContent).toContain("3 shown");
    expect(bar.textContent).toContain("1 open");
  });

  it("renders an empty corpus status bar without kind counts", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: [] }) }),
    ) as unknown as typeof fetch;
    renderTree();
    await waitFor(() => expect(document.querySelector(".tree-status")).toBeTruthy());
    const bar = document.querySelector(".tree-status") as HTMLElement;
    expect(bar.textContent).toContain("0 shown");
    expect(bar.textContent).toContain("0 total");
    expect(bar.textContent).toContain("0 open");
    expect(bar.querySelector(".tree-status__kinds")).toBeNull();
  });

  describe("recent documents", () => {
    it("stays hidden until something has been read", async () => {
      globalThis.fetch = vi.fn(() =>
        Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
      ) as unknown as typeof fetch;
      renderTree();
      await screen.findByText("Alpha");
      expect(document.querySelector(".tree-folder--recent")).toBeNull();
    });

    it("lists what was read, newest first, and links to the document", async () => {
      globalThis.fetch = vi.fn(() =>
        Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
      ) as unknown as typeof fetch;
      recordReading("context/wiki/zeta.md", 10, null, 0);
      recordReading("context/analysis/a.md", 20, null, 0);
      renderTree();
      await screen.findByText("Alpha");

      const recent = document.querySelector(".tree-folder--recent");
      expect(recent).toBeTruthy();
      const links = Array.from(recent!.querySelectorAll("a")).map((a) => a.getAttribute("href"));
      expect(links).toEqual(["/docs/context/analysis/a.md", "/docs/context/wiki/zeta.md"]);
      expect(recent!.querySelector(".tree-folder__count")?.textContent).toBe("2");
    });

    it("drops a document that left the corpus", async () => {
      globalThis.fetch = vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ entries: [TREE.entries[0]] }),
        }),
      ) as unknown as typeof fetch;
      recordReading("context/wiki/zeta.md", 10, null, 0);
      recordReading("context/analysis/a.md", 20, null, 0);
      renderTree();
      // "Zeta" appears twice: in the tree and in the recent list
      await screen.findAllByText("Zeta");
      const recent = document.querySelector(".tree-folder--recent");
      expect(recent).toBeTruthy();
      expect(recent!.querySelectorAll("a")).toHaveLength(1);
      expect(recent!.querySelector("a")?.getAttribute("href")).toBe("/docs/context/wiki/zeta.md");
    });

    it("collapses with the shared open-state model", async () => {
      globalThis.fetch = vi.fn(() =>
        Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
      ) as unknown as typeof fetch;
      recordReading("context/wiki/zeta.md", 10, null, 0);
      renderTree();
      await screen.findAllByText("Zeta");
      const recent = document.querySelector(".tree-folder--recent") as HTMLDetailsElement;
      expect(recent.open).toBe(false);
      await userEvent.click(recent.querySelector("summary") as HTMLElement);
      await waitFor(() => expect(recent.open).toBe(true));
    });
  });
});
