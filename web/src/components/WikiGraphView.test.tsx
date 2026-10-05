// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { WikiGraphView } from "./WikiGraphView";
import { OpenDocsProvider } from "./OpenDocsProvider";

// The ported engine is WebGL/RAF and cannot run in jsdom; the view is tested in
// `GraphView.test.tsx`, so here it is replaced by a node list that drives the
// same select/open callbacks.
vi.mock("./GraphView", () => ({
  GraphView: ({
    nodes,
    mode,
    onSelect,
    onNodeDoubleClick,
  }: {
    nodes: { id: string; label?: string }[];
    mode: string;
    onSelect: (id: string | null) => void;
    onNodeDoubleClick?: (n: { id: string; label?: string }) => void;
  }) => (
    <div data-testid="graph-view" data-mode={mode}>
      {nodes.map((n) => (
        <button
          key={n.id}
          onClick={() => onSelect(n.id)}
          onDoubleClick={() => onNodeDoubleClick?.(n)}
        >
          {n.label ?? n.id}
        </button>
      ))}
    </div>
  ),
}));

const GRAPH = {
  nodes: [
    { id: "a", title: "Alpha", type: "concept", status: "active", path: "context/wiki/a.md" },
    { id: "b", title: "Beta", type: "module", status: "draft", path: "context/wiki/b.md" },
  ],
  edges: [{ source: "a", target: "b", verb: "depends_on", kind: "relation" }],
};

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="location">{loc.pathname}</span>;
}

function renderView() {
  render(
    <MemoryRouter initialEntries={["/wiki/graph"]}>
      <OpenDocsProvider>
        <Routes>
          <Route path="/wiki/graph" element={<WikiGraphView />} />
          <Route path="/docs/*" element={<LocationProbe />} />
        </Routes>
      </OpenDocsProvider>
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("WikiGraphView", () => {
  it("renders the controls, legend and the graph view", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    expect(await screen.findByTestId("graph-view")).toBeTruthy();
    expect(screen.getByRole("button", { name: "2D", pressed: true })).toBeTruthy();
    expect(screen.getByText("concept (1)")).toBeTruthy();
    expect(screen.getAllByText("depends_on").length).toBeGreaterThan(0);
  });

  it("selects a node, shows it and opens the detail route", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    await userEvent.click(await screen.findByRole("button", { name: "Alpha" }));
    expect(screen.getByText("Selected")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Open page" }));
    await waitFor(() =>
      expect(screen.getByTestId("location").textContent).toBe("/docs/context/wiki/a.md"),
    );
  });

  it("switches the mode and the layout", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    await userEvent.click(await screen.findByRole("button", { name: "3D" }));
    expect(screen.getByTestId("graph-view").dataset.mode).toBe("3d");
  });

  it("shows an error state when the graph API fails", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ error: "boom" }) }),
    ) as unknown as typeof fetch;
    renderView();
    expect(await screen.findByText(/Graph error: boom/)).toBeTruthy();
  });
});
