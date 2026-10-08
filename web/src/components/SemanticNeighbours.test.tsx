// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { SemanticNeighbours } from "./SemanticNeighbours";

const GRAPH = {
  nodes: [
    { path: "context/notes/a.md", kind: "notes", title: "Alpha" },
    { path: "context/analysis/b.md", kind: "analysis", title: "Beta" },
  ],
  edges: [{ from: "context/notes/a.md", to: "context/analysis/b.md", score: 0.9 }],
};

function renderTab(path: string) {
  render(
    <MemoryRouter>
      <SemanticNeighbours path={path} />
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SemanticNeighbours", () => {
  it("lists a document's neighbours with a link and score", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderTab("context/notes/a.md");
    const link = await screen.findByRole("link", { name: /Beta/ });
    expect(link.getAttribute("href")).toBe("/docs/context/analysis/b.md");
    expect(screen.getByText("0.90")).toBeTruthy();
  });

  it("shows an empty state when the index has nothing", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ nodes: [], edges: [] }) }),
    ) as unknown as typeof fetch;
    renderTab("context/notes/a.md");
    expect(await screen.findByText("No semantic neighbours.")).toBeTruthy();
  });
});
