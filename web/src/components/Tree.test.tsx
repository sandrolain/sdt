// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Tree } from "./Tree";
import { resetTreeSort, setTreeSortKey } from "../lib/treeSortStore";
import { resetTreeFilter, toggleHideCompleted } from "../lib/treeFilterStore";

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

  it("hides only completed entries when the not-completed filter is on", async () => {
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
                path: "context/tasks/wip.md",
                kind: "tasks",
                title: "Wip task",
                status: "in-progress",
              },
              {
                path: "context/plan/p.md",
                kind: "plan",
                title: "All-done plan",
                status: "active",
              },
              {
                path: "context/tasks/done.md",
                kind: "tasks",
                title: "Done task",
                status: "completed",
                sources: ["plan/p.md"],
              },
              { path: "context/notes/n.md", kind: "notes", title: "Note" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    toggleHideCompleted();
    renderTree();
    // completed entries (by status, or any plan whose tasks are all done) vanish
    expect(await screen.findByText("Wip task")).toBeTruthy();
    expect(screen.queryByText("Done analysis")).toBeNull();
    expect(screen.queryByText("All-done plan")).toBeNull();
    expect(screen.queryByText("Done task")).toBeNull();
    // unfinished items and non-done kinds stay: a plan without completed tasks
    // is kept, and dot-less entries (notes) are no longer hidden as a side effect
    expect(screen.getByText("Open analysis")).toBeTruthy();
    expect(screen.getByText("Note")).toBeTruthy();
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
                sources: ["plan/20260920-a-plan.md"],
              },
              { path: "context/tasks/t2.md", kind: "tasks", title: "T2", status: "pending" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("T1");
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
                sources: ["plan/20260920-a-plan.md"],
              },
              {
                path: "context/tasks/t2.md",
                kind: "tasks",
                title: "T2 open",
                status: "pending",
                sources: ["plan/20260920-a-plan.md"],
              },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("T1 done");
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
    const objectiveFolders = Array.from(document.querySelectorAll(".tree-folder--objective"));
    expect(objectiveFolders).toHaveLength(1);
    expect(objectiveFolders[0].querySelector(".tree-folder__label")?.textContent).toBe("viewer");
    expect(objectiveFolders[0].querySelector(".tree-folder__count")?.textContent).toBe("2");
    expect(objectiveFolders[0].textContent).toContain("Alpha");
    expect(objectiveFolders[0].textContent).toContain("Beta");
    // the objective-less analysis stays at the analysis-folder root
    expect(objectiveFolders[0].textContent).not.toContain("Gamma");
  });

  it("aggregates objective dots over visible analyses and styles archived/question dots", async () => {
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
                sources: ["plan/p.md"],
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
    const viewer = Array.from(document.querySelectorAll(".tree-folder--objective")).find(
      (folder) => folder.querySelector(".tree-folder__label")?.textContent === "viewer",
    ) as HTMLElement;
    expect(viewer.querySelector(".tree-folder__label")?.textContent).toBe("viewer");
    expect(viewer.querySelector(".tree-folder__dot--warn")?.getAttribute("aria-label")).toBe(
      "1/2 analyses completed",
    );
    expect(
      document.querySelector(
        'a[href="/docs/context/analysis/archived.md"] .tree-entry__dot--neutral',
      ),
    ).toBeTruthy();
    const retired = Array.from(document.querySelectorAll(".tree-folder--objective")).find(
      (folder) => folder.querySelector(".tree-folder__label")?.textContent === "retired",
    );
    expect(retired?.querySelector(".tree-folder__dot")).toBeNull();
    expect(screen.getByLabelText("Question unresolved")).toBeTruthy();
    expect(screen.getByLabelText("Question resolved")).toBeTruthy();

    act(() => toggleHideCompleted());
    await waitFor(() => {
      expect(viewer.querySelector(".tree-folder__count")?.textContent).toBe("2");
      expect(document.querySelector('a[href="/docs/context/analysis/archived.md"]')).toBeNull();
      expect(viewer.querySelector(".tree-folder__dot--warn")?.getAttribute("aria-label")).toBe(
        "1/2 analyses completed",
      );
      expect(
        Array.from(document.querySelectorAll(".tree-folder--objective")).some(
          (folder) => folder.querySelector(".tree-folder__label")?.textContent === "retired",
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
    const objectiveNames = () =>
      Array.from(document.querySelectorAll(".tree-folder--objective")).map(
        (f) => f.querySelector(".tree-folder__label")?.textContent,
      );
    // default created_desc: the group with the latest created comes first
    expect(objectiveNames()).toEqual(["zeta", "alpha"]);

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
    await waitFor(() => expect(objectiveNames()).toEqual(["alpha", "zeta"]));
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
                sources: ["plan/p.md"],
              },
              {
                path: "context/tasks/t2.md",
                kind: "tasks",
                title: "Task two",
                status: "completed",
                sources: ["plan/p.md"],
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

    const folder = (label: string) =>
      Array.from(document.querySelectorAll(".tree-folder")).find(
        (f) => f.querySelector(".tree-folder__label")?.textContent === label,
      );
    const planObjective = Array.from(
      folder("Plans")?.querySelectorAll(".tree-folder--objective") ?? [],
    ).find((f) => f.querySelector(".tree-folder__label")?.textContent === "viewer");
    expect(planObjective?.textContent).toContain("Plan");
    // the objective-less plan stays at the plan-folder root
    expect(planObjective?.textContent).not.toContain("Other plan");

    const taskObjective = Array.from(
      folder("Tasks")?.querySelectorAll(".tree-folder--objective") ?? [],
    ).find((f) => f.querySelector(".tree-folder__label")?.textContent === "viewer");
    const planSubGroup = taskObjective?.querySelector(".tree-folder--plan");
    expect(planSubGroup?.textContent).toContain("Task one");
    expect(planSubGroup?.textContent).toContain("Task two");
    // the loose task (no plan, no objective) stays at the task-folder root
    expect(folder("Tasks")?.textContent).toContain("Loose task");
  });
});
