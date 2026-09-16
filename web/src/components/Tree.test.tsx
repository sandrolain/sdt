// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { Tree } from "./Tree";

const TREE = {
  entries: [
    { path: "context/wiki/zeta.md", kind: "wiki", title: "Zeta", created: "2026-09-02" },
    { path: "context/wiki/alpha.md", kind: "wiki", title: "Alpha", created: "2026-09-01" },
    { path: "context/notes/20260915-195559-note.md", kind: "notes", title: "Note" },
    { path: "context/plan/p.md", kind: "plan", title: "Plan", status: "active" },
    { path: "context/tasks/t.md", kind: "tasks", title: "Task", status: "in-progress" },
    { path: "context/analysis/a.md", kind: "analysis", title: "Analysis" },
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
  vi.restoreAllMocks();
});

describe("Tree", () => {
  it("renders kind folders with counts and icons", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    expect(await screen.findByText("Alpha")).toBeTruthy();
    // folder headers (also present as entry kind badges, hence *AllBy*)
    expect(screen.getAllByText("wiki").length).toBeGreaterThan(0);
    expect(screen.getAllByText("notes").length).toBeGreaterThan(0);
    // folder counts live in the folder header
    const headers = Array.from(document.querySelectorAll(".tree-folder__header"));
    const countFor = (kind: string) =>
      headers
        .find((h) => h.querySelector(".tree-folder__label")?.textContent === kind)
        ?.querySelector(".tree-folder__count")?.textContent;
    expect(countFor("wiki")).toBe("2");
    expect(countFor("notes")).toBe("1");
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

  it("sorts by title descending from the sort control", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");

    const titles = () => screen.getAllByRole("link").map((el) => el.textContent ?? "");
    expect(titles()[0]).toContain("Alpha");

    await userEvent.selectOptions(screen.getByLabelText("Sort entries by"), "title");
    await userEvent.click(screen.getByLabelText("Sort ascending"));
    expect(titles()[0]).toContain("Zeta");
    expect(screen.getByLabelText("Sort descending")).toBeTruthy();
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

  it("shows a date line from frontmatter or the filename prefix", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");
    // filename date prefix on the notes entry (locale-safe: no year assumption)
    expect(screen.getAllByText(/2026/).length).toBeGreaterThan(0);
  });
});
