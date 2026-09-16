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
    // folder counts
    expect(screen.getByText("2")).toBeTruthy();
    expect(screen.getByText("1")).toBeTruthy();
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
