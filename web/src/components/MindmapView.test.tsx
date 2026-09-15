// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MindmapView } from "./MindmapView";
import type { MindNode } from "../lib/mindmap";

const setData = vi.fn((_data: MindNode, _opts?: { autoFit?: boolean }) => Promise.resolve());
const fit = vi.fn(() => Promise.resolve());
const destroy = vi.fn();
const selectAll = vi.fn(() => ({ data: () => ({ join: () => group }) }));
const group: Record<string, unknown> = {
  attr: () => group,
  lower: () => group,
  text: () => group,
  selectAll,
};

vi.mock("markmap-view", () => ({
  Markmap: {
    create: (_svg: SVGElement, _opts?: unknown, _data?: unknown) => ({
      g: { selectAll },
      state: { data: undefined },
      setData: (data: MindNode, opts?: { autoFit?: boolean }) => {
        setData(data, opts);
        return Promise.resolve();
      },
      fit,
      destroy,
    }),
  },
}));

const TREE_FIXTURE = {
  entries: [
    { path: "context/wiki/topic.map.md", title: "Topic Map", isMap: true, mapId: "topic.map" },
    { path: "context/wiki/other.map.md", title: "Other Map", isMap: true, mapId: "other.map" },
  ],
};

function mockFetch() {
  return vi.fn((url: string) => {
    if (url === "/api/tree")
      return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE_FIXTURE) });
    if (url.startsWith("/api/doc")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            path: "context/wiki/other.map.md",
            frontmatter: "",
            markdown: "# Other\n\n- x\n",
          }),
      });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  });
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("MindmapView", () => {
  it("renders an accessible svg and pushes data to markmap", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    render(
      <MindmapView
        markdown="# Root\n\n- child\n"
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    const svg = screen.getByRole("img", { name: "Mindmap: Topic" });
    expect(svg).toBeTruthy();
    await waitFor(() => expect(setData).toHaveBeenCalled());
  });

  it("exposes Current/Fused toggles for maps with references", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    render(
      <MindmapView
        markdown={"# Root\n\n- [Other](#/wiki/other.map)\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(await screen.findByRole("button", { name: "Current Map", pressed: true })).toBeTruthy();
    const fused = screen.getByRole("button", { name: "Fused Map" });
    await userEvent.click(fused);
    expect(screen.getByRole("button", { name: "Fused Map", pressed: true })).toBeTruthy();
    expect(await screen.findByText(/imported/)).toBeTruthy();
  });

  it("hides the fused toggle for ordinary documents", () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    render(<MindmapView markdown="# Root\n" basePath="context/analysis/x.md" title="X" />);
    expect(screen.queryByRole("button", { name: "Fused Map" })).toBeNull();
  });
});
