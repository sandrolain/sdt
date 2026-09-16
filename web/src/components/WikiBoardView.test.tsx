// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { WikiBoardView } from "./WikiBoardView";
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
    nodes: [{ id: "c", type: "text", x: 0, y: 0, width: 100, height: 50, text: "From file" }],
    edges: [],
  },
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
  vi.restoreAllMocks();
});

describe("WikiBoardView", () => {
  it("renders the default graph board and the source selector", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    render(
      <MemoryRouter>
        <OpenDocsProvider>
          <WikiBoardView />
        </OpenDocsProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByRole("button", { name: "Alpha" })).toBeTruthy();
    const select = screen.getByLabelText("Board source") as HTMLSelectElement;
    expect(select.value).toBe("");
    expect(screen.getByRole("option", { name: "Wiki graph (default)" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "board" })).toBeTruthy();
  });

  it("switches to a selected .canvas file", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    render(
      <MemoryRouter>
        <OpenDocsProvider>
          <WikiBoardView />
        </OpenDocsProvider>
      </MemoryRouter>,
    );
    await screen.findByRole("button", { name: "Alpha" });
    await userEvent.selectOptions(
      screen.getByLabelText("Board source"),
      "context/wiki/board.canvas",
    );
    expect(await screen.findByRole("button", { name: "From file" })).toBeTruthy();
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
    render(
      <MemoryRouter>
        <OpenDocsProvider>
          <WikiBoardView />
        </OpenDocsProvider>
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText(/Board error: boom/)).toBeTruthy());
  });
});
