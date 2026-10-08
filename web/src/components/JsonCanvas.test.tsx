// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { createRef } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { JsonCanvas, type JsonCanvasHandle } from "./JsonCanvas";
import type { CanvasDocument } from "../lib/jsoncanvas/document";

// The 3D scene needs WebGL (unavailable in jsdom); stub it to exercise the pick.
vi.mock("../lib/jsoncanvas/layers3d", () => ({
  default: ({ onPick }: { onPick: (id: string) => void }) => (
    <button onClick={() => onPick("a")}>pick-a</button>
  ),
}));

const DOC: CanvasDocument = {
  nodes: [
    { id: "a", type: "text", x: 0, y: 0, width: 100, height: 50, text: "Alpha" },
    { id: "b", type: "text", x: 200, y: 0, width: 100, height: 50, text: "Beta" },
  ],
  edges: [{ id: "e", fromNode: "a", toNode: "b" }],
};

afterEach(cleanup);

describe("JsonCanvas (read-only)", () => {
  it("renders the nodes and the edge layer", () => {
    render(<JsonCanvas data={DOC} />);
    expect(screen.getByRole("button", { name: "Alpha" })).toBeTruthy();
    expect(screen.getByRole("application", { name: /JSON Canvas/ })).toBeTruthy();
    expect(document.querySelector(".jc-edge")).toBeTruthy();
  });

  it("honours showMinimap", () => {
    const { container } = render(<JsonCanvas data={DOC} showMinimap={false} />);
    expect(container.querySelector(".jc-mm")).toBeNull();
    cleanup();
    render(<JsonCanvas data={DOC} />);
    expect(document.querySelector(".jc-mm")).toBeTruthy();
  });

  it("exposes zoom and fit through the imperative handle without throwing", () => {
    const ref = createRef<JsonCanvasHandle>();
    render(<JsonCanvas ref={ref} data={DOC} />);
    expect(() => {
      ref.current?.zoomIn();
      ref.current?.zoomOut();
      ref.current?.fit();
      ref.current?.focusNode("a");
    }).not.toThrow();
  });

  it("routes a 3D pick to onPick, not onOpenNode (O4)", async () => {
    const onPick = vi.fn();
    const onOpenNode = vi.fn();
    render(<JsonCanvas data={DOC} mode="3d" onPick={onPick} onOpenNode={onOpenNode} />);
    await userEvent.click(await screen.findByRole("button", { name: "pick-a" }));
    expect(onPick).toHaveBeenCalledTimes(1);
    expect(onPick.mock.calls[0][0].id).toBe("a");
    expect(onOpenNode).not.toHaveBeenCalled();
  });

  it("keeps an unsafe link inert and a safe link navigable", () => {
    const doc: CanvasDocument = {
      nodes: [
        { id: "bad", type: "link", x: 0, y: 0, width: 100, height: 50, url: "javascript:alert(1)" },
        {
          id: "ok",
          type: "link",
          x: 0,
          y: 60,
          width: 100,
          height: 50,
          url: "https://example.com/a",
        },
      ],
      edges: [],
    };
    render(<JsonCanvas data={doc} />);
    const links = document.querySelectorAll("a.jc-cp");
    expect(links).toHaveLength(1);
    expect(links[0].getAttribute("href")).toBe("https://example.com/a");
  });
});
