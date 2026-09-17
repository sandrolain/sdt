// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
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
    expect(countFor("wiki")).toBe("3");
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
    expect(screen.getByLabelText("Sort descending")).toBeTruthy();
  });

  it("sorts by title ascending from the sort control", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) }),
    ) as unknown as typeof fetch;
    renderTree();
    await screen.findByText("Alpha");

    const titles = () => screen.getAllByRole("link").map((el) => el.textContent ?? "");
    await userEvent.click(screen.getByRole("button", { name: /Sort entries by/ }));
    await userEvent.click(await screen.findByRole("option", { name: "Title" }));
    // direction is still descending: reverse-alphabetical first
    expect(titles()[0]).toContain("Zeta");
    await userEvent.click(screen.getByLabelText("Sort descending"));
    expect(titles()[0]).toContain("Alpha");
    expect(screen.getByLabelText("Sort ascending")).toBeTruthy();
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
