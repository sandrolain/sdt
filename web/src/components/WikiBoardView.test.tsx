// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { WikiBoardView } from "./WikiBoardView";
import { renderNestedBody } from "./nestedCanvasBody";
import type { CanvasNode } from "../lib/jsoncanvas/document";
import { resetCanvasCache } from "../lib/jsoncanvas/canvasLoader";
import { OpenDocsProvider } from "./OpenDocsProvider";

const TREE = {
  entries: [
    { path: "context/wiki/board.canvas", kind: "canvas", title: "board", canvas: true },
    { path: "context/wiki/x.md", kind: "wiki", title: "X" },
  ],
};

const DEFAULT_BOARD = {
  nodes: [{ id: "a", type: "text", x: 0, y: 0, width: 100, height: 50, text: "Alpha" }],
  edges: [],
};

const FILE_BOARD = {
  path: "context/wiki/board.canvas",
  canvas: {
    nodes: [
      {
        id: "c",
        type: "text",
        x: 0,
        y: 0,
        width: 100,
        height: 50,
        text: "From file",
        "x-layer": 2,
      },
    ],
    edges: [],
    "x-layers": [{ id: 2, name: "Deep" }],
  },
};

/** A board with an inline `nested-canvas` node carrying a child text node. */
const NESTED_BOARD = {
  nodes: [
    { id: "root", type: "text", x: 0, y: 0, width: 120, height: 40, text: "Root" },
    {
      id: "nc",
      type: "nested-canvas",
      x: 200,
      y: 0,
      width: 200,
      height: 120,
      title: "Investigation",
      canvas: {
        nodes: [{ id: "inner", type: "text", x: 0, y: 0, width: 100, height: 40, text: "Inner" }],
        edges: [],
      },
    },
  ],
  edges: [],
};

/** A board whose only node is an external `.canvas` file reference. */
const EXTERNAL_BOARD = {
  nodes: [{ id: "ext", type: "text", x: 0, y: 0, width: 120, height: 40, text: "External board" }],
  edges: [],
};

const EXTERNAL_REF_BOARD = {
  nodes: [
    {
      id: "f",
      type: "file",
      x: 0,
      y: 0,
      width: 150,
      height: 60,
      file: "context/wiki/other.canvas",
    },
  ],
  edges: [],
};

function mockFetch() {
  return vi.fn((url: string) => {
    if (url === "/api/tree")
      return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
    if (url.startsWith("/api/wiki/board")) {
      const isFile = url.includes("file=");
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(isFile ? FILE_BOARD : DEFAULT_BOARD),
      });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  });
}

afterEach(() => {
  cleanup();
  resetCanvasCache();
  vi.restoreAllMocks();
});

function renderView(initial = "/wiki/board") {
  return render(
    <MemoryRouter initialEntries={[initial]}>
      <OpenDocsProvider>
        <Routes>
          <Route path="/wiki/board/*" element={<WikiBoardView />} />
        </Routes>
      </OpenDocsProvider>
    </MemoryRouter>,
  );
}

describe("WikiBoardView", () => {
  it("renders the default graph board and the docked controls", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderView();
    expect(await screen.findByRole("button", { name: "Alpha" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Wiki graph/ })).toBeTruthy();
    expect(screen.getByRole("switch", { name: "Minimap" })).toBeTruthy();
  });

  it("switches to a selected .canvas file from the sidebar", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderView();
    await screen.findByRole("button", { name: "Alpha" });
    await userEvent.click(screen.getByRole("button", { name: /Wiki graph/ }));
    await userEvent.click(await screen.findByRole("option", { name: "board" }));
    expect(await screen.findByRole("button", { name: "From file" })).toBeTruthy();
  });

  it("exposes the 2D/3D mode and hides layer switches in 2D", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderView();
    await screen.findByRole("button", { name: "Alpha" });
    expect(screen.getByRole("button", { name: /3D/ })).toBeTruthy();
    // Layers are 3D-only (O1); the full view mounts WebGL in 3D, which jsdom
    // cannot, so the 3D layer switch is asserted in BoardControls.test.tsx.
    expect(screen.queryByRole("switch", { name: "Level 0" })).toBeNull();
  });

  it("shows an error state when the board API fails", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      return Promise.resolve({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: "boom" }),
      });
    }) as unknown as typeof fetch;
    renderView();
    await waitFor(() => expect(screen.getByText(/Board error: boom/)).toBeTruthy());
  });

  it("drills into a nested-canvas node and back with Escape", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      if (url.startsWith("/api/wiki/board"))
        return Promise.resolve({ ok: true, json: () => Promise.resolve(NESTED_BOARD) });
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    const { container } = renderView();
    const node = await screen.findByRole("button", { name: "Investigation" });
    await userEvent.dblClick(node);
    await waitFor(() =>
      expect(container.querySelector(".board-breadcrumb")?.textContent).toContain("Investigation"),
    );
    // The active level is the child canvas; the root node is gone.
    expect(screen.queryByRole("button", { name: "Root" })).toBeNull();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(container.querySelector(".board-breadcrumb")).toBeNull());
    expect(await screen.findByRole("button", { name: "Root" })).toBeTruthy();
  });

  it("drills into an inline nested canvas without refetching the board", async () => {
    let boardCalls = 0;
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      if (url.startsWith("/api/wiki/board")) {
        boardCalls += 1;
        return Promise.resolve({ ok: true, json: () => Promise.resolve(NESTED_BOARD) });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    const { container } = renderView();
    const node = await screen.findByRole("button", { name: "Investigation" });
    expect(boardCalls).toBe(1);
    await userEvent.dblClick(node);
    await waitFor(() =>
      expect(container.querySelector(".board-breadcrumb")?.textContent).toContain("Investigation"),
    );
    // The child level came from the loaded model: no second board fetch.
    expect(boardCalls).toBe(1);
    expect(screen.queryByText(/Loading board/)).toBeNull();
  });

  it("enters a nested canvas through the zoom-in affordance", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      if (url.startsWith("/api/wiki/board"))
        return Promise.resolve({ ok: true, json: () => Promise.resolve(NESTED_BOARD) });
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    const { container } = renderView();
    await screen.findByRole("button", { name: "Investigation" });
    await userEvent.click(await screen.findByRole("button", { name: "Zoom into Investigation" }));
    // The enter is delayed by the "zoom into" animation step.
    await waitFor(() =>
      expect(container.querySelector(".board-breadcrumb")?.textContent).toContain("Investigation"),
    );
    expect(screen.queryByRole("button", { name: "Root" })).toBeNull();
  });

  it("navigates to an external .canvas target through the board route", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      if (
        url.includes("file=context%2Fwiki%2Fother.canvas") ||
        url.includes("file=context/wiki/other.canvas")
      )
        return Promise.resolve({ ok: true, json: () => Promise.resolve(EXTERNAL_BOARD) });
      if (url.startsWith("/api/wiki/board"))
        return Promise.resolve({ ok: true, json: () => Promise.resolve(EXTERNAL_REF_BOARD) });
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderView();
    const ref = await screen.findByRole("button", { name: "context/wiki/other.canvas" });
    await userEvent.dblClick(ref);
    expect(await screen.findByRole("button", { name: "External board" })).toBeTruthy();
  });
});

describe("renderNestedBody", () => {
  const node = {
    id: "nc",
    type: "nested-canvas",
    title: "Inv",
    x: 0,
    y: 0,
    width: 100,
    height: 100,
    canvas: { nodes: [{ id: "x", type: "text", x: 0, y: 0, width: 10, height: 10 }], edges: [] },
  } as CanvasNode;

  it("shows the opaque placeholder below the threshold", () => {
    const { container } = render(<>{renderNestedBody(node, 0.5)}</>);
    expect(container.querySelector(".nested-canvas--placeholder")).toBeTruthy();
  });

  it("renders a nested miniature above the threshold", () => {
    const { container } = render(<>{renderNestedBody(node, 1)}</>);
    expect(container.querySelector(".nested-canvas--mini")).toBeTruthy();
  });

  it("falls through (undefined) for a non-nested node", () => {
    expect(
      renderNestedBody({ id: "t", type: "text", x: 0, y: 0, width: 10, height: 10 }, 1),
    ).toBeUndefined();
  });
});

describe("renderNestedBody — external .canvas thumbnails", () => {
  const fileNode = {
    id: "f",
    type: "file",
    x: 0,
    y: 0,
    width: 150,
    height: 60,
    file: "context/wiki/other.canvas",
  } as CanvasNode;

  function mockCanvasFetch(payload: unknown) {
    return vi.fn((url: string) => {
      if (url.startsWith("/api/wiki/board?file="))
        return Promise.resolve({ ok: true, json: () => Promise.resolve(payload) });
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
  }

  it("shows a placeholder below the threshold without fetching", () => {
    const fetchMock = vi.fn();
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const { container } = render(<>{renderNestedBody(fileNode, 0.5)}</>);
    expect(container.querySelector(".nested-canvas--placeholder")).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("fetches and renders a miniature above the threshold", async () => {
    globalThis.fetch = mockCanvasFetch(EXTERNAL_BOARD);
    const { container } = render(<>{renderNestedBody(fileNode, 1)}</>);
    await waitFor(() => expect(container.querySelector(".nested-canvas--mini")).toBeTruthy());
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringContaining("file=context%2Fwiki%2Fother.canvas"),
    );
  });

  it("shows a missing-canvas placeholder when the target is empty", async () => {
    globalThis.fetch = mockCanvasFetch({ nodes: [], edges: [] });
    const { container } = render(<>{renderNestedBody(fileNode, 1)}</>);
    await waitFor(() =>
      expect(container.querySelector(".nested-canvas__hint")?.textContent).toBe("Missing canvas"),
    );
  });

  it("offers a zoom affordance that calls onEnter", async () => {
    globalThis.fetch = mockCanvasFetch(EXTERNAL_BOARD);
    const onEnter = vi.fn();
    const { container } = render(<>{renderNestedBody(fileNode, 1, { onEnter })}</>);
    const zoom = await waitFor(() => {
      const b = container.querySelector<HTMLButtonElement>(".nested-canvas__zoom");
      if (!b) throw new Error("zoom affordance not rendered");
      return b;
    });
    await userEvent.click(zoom);
    expect(onEnter).toHaveBeenCalledWith(fileNode);
  });
});
