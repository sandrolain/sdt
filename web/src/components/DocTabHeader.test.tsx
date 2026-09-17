// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { IDockviewPanelHeaderProps } from "dockview-react";
import { resetCorpusIndexCache } from "../lib/corpusIndex";
import { DocTabHeader } from "./DocTabHeader";

function renderTab(path: string) {
  const api = { id: `doc:${path}`, title: path, close: vi.fn() };
  const props = {
    api,
    containerApi: {} as never,
    params: { path },
    tabLocation: "header",
  } as unknown as IDockviewPanelHeaderProps;
  render(<DocTabHeader {...props} />);
  return api;
}

beforeEach(() => {
  resetCorpusIndexCache();
  globalThis.fetch = vi.fn(() =>
    Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: [] }) }),
  ) as unknown as typeof fetch;
});

afterEach(cleanup);

describe("DocTabHeader", () => {
  it("shows the coloured kind glyph before the title", async () => {
    renderTab("context/analysis/a.md");
    expect(screen.getByText("A")).toBeTruthy();
    expect(await screen.findByLabelText("kind: analysis")).toBeTruthy();
  });

  it("derives the kind from the folder when the tree is empty", () => {
    renderTab("context/wiki/page.md");
    expect(screen.getByLabelText("kind: wiki")).toBeTruthy();
  });
});
