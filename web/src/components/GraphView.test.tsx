// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { createRef } from "react";
import { GraphView, type GraphViewHandle } from "./GraphView";

const engine = vi.hoisted(() => ({
  setData: vi.fn(),
  setMode: vi.fn(),
  setLayout: vi.fn(),
  setSelected: vi.fn(),
  setStyle: vi.fn(),
  setPath: vi.fn(),
  setFilters: vi.fn(),
  setColors: vi.fn(),
  setCentrality: vi.fn(),
  setNeighborhood: vi.fn(),
  focusNode: vi.fn(),
  fitView: vi.fn(),
  toSVG: vi.fn(() => "<svg/>"),
  dispose: vi.fn(),
}));

vi.mock("../lib/graph/engine", () => ({
  GraphEngine: class {
    w = 100;
    h = 100;
    events: Record<string, unknown> = {};
    setData = engine.setData;
    setMode = engine.setMode;
    setLayout = engine.setLayout;
    setSelected = engine.setSelected;
    setStyle = engine.setStyle;
    setPath = engine.setPath;
    setFilters = engine.setFilters;
    setColors = engine.setColors;
    setCentrality = engine.setCentrality;
    setNeighborhood = engine.setNeighborhood;
    focusNode = engine.focusNode;
    fitView = engine.fitView;
    toSVG = engine.toSVG;
    dispose = engine.dispose;
  },
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const nodes = [{ id: "a", label: "Alpha" }];
const links = [{ source: "a", target: "b", type: "refers_to" }];

describe("GraphView", () => {
  it("pushes data and controlled state into the engine", () => {
    const { rerender } = render(
      <GraphView
        nodes={nodes}
        links={links}
        mode="2d"
        layout="force"
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    expect(engine.setData).toHaveBeenCalledWith(nodes, links);
    expect(engine.setMode).toHaveBeenCalledWith("2d");
    expect(engine.setLayout).toHaveBeenCalledWith("force");

    rerender(
      <GraphView
        nodes={nodes}
        links={links}
        mode="3d"
        layout="hierarchy"
        selectedId="a"
        onSelect={() => {}}
      />,
    );
    expect(engine.setMode).toHaveBeenLastCalledWith("3d");
    expect(engine.setLayout).toHaveBeenLastCalledWith("hierarchy");
    expect(engine.setSelected).toHaveBeenLastCalledWith("a");
  });

  it("exposes the imperative handle", () => {
    const ref = createRef<GraphViewHandle>();
    render(
      <GraphView
        ref={ref}
        nodes={nodes}
        links={links}
        mode="2d"
        layout="force"
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    ref.current?.fitView();
    ref.current?.focusNode("a");
    expect(engine.fitView).toHaveBeenCalled();
    expect(engine.focusNode).toHaveBeenCalledWith("a");
  });

  it("disposes the engine on unmount", () => {
    const { unmount } = render(
      <GraphView
        nodes={nodes}
        links={links}
        mode="2d"
        layout="force"
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    unmount();
    expect(engine.dispose).toHaveBeenCalled();
  });
});
