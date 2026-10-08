// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { SemanticMapView } from "./SemanticMapView";
import { OpenDocsProvider } from "./OpenDocsProvider";

// The ported engine is WebGL/RAF and cannot run in jsdom; the view is tested in
// `GraphView.test.tsx`, so here it is replaced by a node list exposing the same
// node/open callbacks.
vi.mock("./GraphView", () => ({
  GraphView: ({
    nodes,
    onNodeDoubleClick,
  }: {
    nodes: { id: string; label?: string }[];
    onNodeDoubleClick?: (n: { id: string; label?: string; path?: string }) => void;
  }) => (
    <div data-testid="graph-view">
      {nodes.map((n) => (
        <button key={n.id} onDoubleClick={() => onNodeDoubleClick?.(n)}>
          {n.label ?? n.id}
        </button>
      ))}
    </div>
  ),
}));

const GRAPH = {
  nodes: [
    { path: "context/notes/a.md", kind: "notes", title: "Alpha" },
    { path: "context/analysis/b.md", kind: "analysis", title: "Beta" },
  ],
  edges: [{ from: "context/notes/a.md", to: "context/analysis/b.md", score: 0.9 }],
};

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="location">{loc.pathname}</span>;
}

function renderView() {
  render(
    <MemoryRouter initialEntries={["/docs/map"]}>
      <OpenDocsProvider>
        <Routes>
          <Route path="/docs/map" element={<SemanticMapView />} />
          <Route path="/docs" element={<LocationProbe />} />
        </Routes>
      </OpenDocsProvider>
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SemanticMapView", () => {
  it("renders the scoped nodes in the graph engine", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    expect(await screen.findByTestId("graph-view")).toBeTruthy();
    expect(screen.getByText("Alpha")).toBeTruthy();
    expect(screen.getByText("Beta")).toBeTruthy();
  });

  it("opens a document and routes to /docs on node double-click", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    await userEvent.dblClick(await screen.findByText("Alpha"));
    expect((await screen.findByTestId("location")).textContent).toBe("/docs");
  });

  it("shows an empty state when the snapshot holds no nodes", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ nodes: [], edges: [] }) }),
    ) as unknown as typeof fetch;
    renderView();
    expect(await screen.findByText(/No semantic map yet/)).toBeTruthy();
    expect(screen.queryByTestId("graph-view")).toBeNull();
  });
});
