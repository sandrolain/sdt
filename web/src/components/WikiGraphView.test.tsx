// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { WikiGraphView } from "./WikiGraphView";
import { OpenDocsProvider } from "./OpenDocsProvider";
import type { GNode } from "../lib/graphModel";

vi.mock("react-force-graph-2d", () => ({
  default: ({
    graphData,
    onNodeClick,
  }: {
    graphData: { nodes: GNode[] };
    onNodeClick?: (n: GNode) => void;
  }) => (
    <div data-testid="fg2d">
      {graphData.nodes.map((n) => (
        <button key={n.id} onClick={() => onNodeClick?.(n)}>
          {n.title}
        </button>
      ))}
    </div>
  ),
}));

vi.mock("react-force-graph-3d", () => ({
  default: () => <div data-testid="fg3d" />,
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
  it("renders the tools panel, legend and 2D graph", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    expect(await screen.findByTestId("fg2d")).toBeTruthy();
    expect(screen.getByLabelText("Graph tools")).toBeTruthy();
    expect(screen.getByRole("button", { name: "2D", pressed: true })).toBeTruthy();
    expect(screen.getByText("concept (1)")).toBeTruthy();
    expect(screen.getByText("depends_on")).toBeTruthy();
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

  it("lazy-loads the 3D renderer on mode switch", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(GRAPH) }),
    ) as unknown as typeof fetch;
    renderView();
    await userEvent.click(await screen.findByRole("button", { name: "3D" }));
    expect(await screen.findByTestId("fg3d")).toBeTruthy();
  });

  it("shows an error state when the graph API fails", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ error: "boom" }) }),
    ) as unknown as typeof fetch;
    renderView();
    expect(await screen.findByText(/Graph error: boom/)).toBeTruthy();
  });
});
