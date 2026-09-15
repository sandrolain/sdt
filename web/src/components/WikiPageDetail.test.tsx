// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { WikiPageDetail } from "./WikiPageDetail";

vi.mock("./MindmapView", () => ({
  MindmapView: ({ title }: { title: string }) => <div data-testid="mindmap">mindmap {title}</div>,
}));

const DOC = {
  path: "context/wiki/topic.map.md",
  frontmatter: "---\nkind: wiki\ntitle: Topic Map\n---\n",
  markdown: "# Topic\n\nBody text.\n",
};

function renderDetail() {
  render(
    <MemoryRouter initialEntries={["/wiki/topic.map"]}>
      <Routes>
        <Route path="/wiki/*" element={<WikiPageDetail />} />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("WikiPageDetail", () => {
  it("toggles between Document and Mindmap views", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve(DOC) }),
    ) as unknown as typeof fetch;
    renderDetail();

    expect(await screen.findByRole("heading", { name: "Topic Map" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Document", pressed: true })).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Mindmap" }));
    expect(await screen.findByTestId("mindmap")).toBeTruthy();
    expect(screen.getByText("mindmap Topic Map")).toBeTruthy();
  });
});
