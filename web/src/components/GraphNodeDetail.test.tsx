// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphNodeDetail } from "./GraphNodeDetail";
import { graphNeighbours } from "../lib/graph/neighbours";
import type { AdaptedEngineGraph } from "../lib/graph/adapter";

const adapted: AdaptedEngineGraph = {
  nodes: [
    { id: "a", label: "Alpha", group: "concept", color: "#cba6f7", description: "first" },
    { id: "b", label: "Beta", group: "module", color: "#89b4fa" },
    { id: "c", label: "Gamma", group: "module", color: "#89b4fa" },
  ],
  links: [
    { source: "a", target: "b", type: "depends_on", kind: "relation" },
    { source: "c", target: "a", type: "depends_on", kind: "relation" },
    { source: "a", target: "c", type: "contains", kind: "link" },
  ],
  palette: new Map(),
  allVerbs: ["contains", "depends_on"],
  allKinds: ["link", "relation"],
};

afterEach(cleanup);

describe("GraphNodeDetail", () => {
  it("shows an explicit empty state", () => {
    render(<GraphNodeDetail node={null} neighbours={[]} onSelect={vi.fn()} onOpen={vi.fn()} />);
    expect(screen.getByText("Select a node")).toBeTruthy();
  });

  it("shows the node, its metadata and opens it", async () => {
    const onOpen = vi.fn();
    render(
      <GraphNodeDetail
        node={{
          id: "a",
          label: "Alpha",
          group: "concept",
          color: "#cba6f7",
          description: "first",
          type: "analysis",
          status: "active",
          tags: ["ui/x", "graph"],
          path: "context/wiki/a.md",
        }}
        neighbours={graphNeighbours(adapted, "a")}
        onSelect={vi.fn()}
        onOpen={onOpen}
      />,
    );
    expect(screen.getByRole("heading", { name: "Alpha" })).toBeTruthy();
    expect(screen.getByText("concept")).toBeTruthy();
    expect(screen.getByText("first")).toBeTruthy();
    expect(screen.getByText("ui/x, graph")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Open page" }));
    expect(onOpen).toHaveBeenCalledWith("a");
  });

  it("selects a neighbour on click", async () => {
    const onSelect = vi.fn();
    render(
      <GraphNodeDetail
        node={{ id: "a", label: "Alpha" }}
        neighbours={graphNeighbours(adapted, "a")}
        onSelect={onSelect}
        onOpen={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Beta" }));
    expect(onSelect).toHaveBeenCalledWith("b");
  });
});
