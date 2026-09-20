// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Tree } from "./Tree";
import { resetTreeSort } from "../lib/treeSortStore";
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
    // folder headers carry the kind label (no per-entry badges anymore)
    expect(screen.getAllByText("wiki").length).toBeGreaterThan(0);
    expect(screen.getAllByText("notes").length).toBeGreaterThan(0);
    // per-entry kind badges are gone from the rows
    expect(document.querySelector(".tree-entry__kind")).toBeNull();
    // folder counts live in the folder header
    const headers = Array.from(document.querySelectorAll(".tree-folder__header"));
    const countFor = (kind: string) =>
      headers
        .find((h) => h.querySelector(".tree-folder__label")?.textContent === kind)
        ?.querySelector(".tree-folder__count")?.textContent;
    expect(countFor("wiki")).toBe("3");
    expect(countFor("notes")).toBe("1");
    // empty kind folders render at count 0 (questions/proposals/research/…)
    expect(countFor("questions")).toBe("0");
    expect(countFor("proposals")).toBe("0");
    expect(countFor("commands")).toBe("0");
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
    expect(folder("questions")?.querySelector(".tree-empty")?.textContent).toBe("No documents.");
    expect(folder("wiki")?.querySelector(".tree-empty")).toBeNull();
  });

  it("shows status dots for plans, tasks and unplanned analyses", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Plan");
    expect(screen.getByLabelText("Plan not executed")).toBeTruthy();
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
      (h) => h.querySelector(".tree-folder__label")?.textContent === "prompts",
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

  it("keeps only in-work plans/tasks and unplanned analyses when the not-completed filter is on", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            entries: [
              {
                path: "context/plan/done.md",
                kind: "plan",
                title: "Done plan",
                status: "completed",
              },
              {
                path: "context/tasks/wip.md",
                kind: "tasks",
                title: "Wip task",
                status: "in-progress",
              },
              { path: "context/notes/n.md", kind: "notes", title: "Note" },
            ],
          }),
      }),
    ) as unknown as typeof fetch;
    toggleHideCompleted();
    renderTree();
    // still-to-work indicators survive; done plans and status-less entries vanish
    expect(await screen.findByText("Wip task")).toBeTruthy();
    expect(screen.queryByText("Done plan")).toBeNull();
    expect(screen.queryByText("Note")).toBeNull();
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
      expect(folder("notes")?.hasAttribute("open")).toBe(true);
    });
    expect(folder("wiki")?.hasAttribute("open")).toBe(false);
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
    expect(link.getAttribute("title")).toBe("context/analysis/a.md · analysis");
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
});
