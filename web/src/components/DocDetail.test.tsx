// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { DocDetail } from "./DocDetail";

vi.mock("../lib/mermaidRender", () => ({
  renderMermaid: vi.fn(() => Promise.resolve()),
}));

afterEach(cleanup);

function renderDetail(doc: Parameters<typeof DocDetail>[0]["doc"], path: string) {
  render(
    <MemoryRouter>
      <DocDetail path={path} doc={doc} error={null} loading={false} />
    </MemoryRouter>,
  );
}

describe("DocDetail", () => {
  it("renders a .mmd document with the Mermaid mode", () => {
    renderDetail(
      { path: "context/wiki/flow.mmd", source: "flowchart TD\n  A-->B\n" },
      "context/wiki/flow.mmd",
    );
    expect(screen.getByRole("button", { name: "Mermaid", pressed: true })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Code" })).toBeTruthy();
    expect(document.querySelector(".doc-rendered--mermaid .md-mermaid")).toBeTruthy();
  });

  it("renders a .canvas document as raw JSON", () => {
    renderDetail(
      { path: "context/board.canvas", canvas: { nodes: [], edges: [] } },
      "context/board.canvas",
    );
    expect(screen.getByText(/"nodes"/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Mermaid" })).toBeNull();
  });

  it("renders a markdown document with Render mode", () => {
    renderDetail(
      {
        path: "context/wiki/alpha.md",
        frontmatter: "---\ntitle: Alpha\n---\n",
        markdown: "# Alpha\n\nHello **tokens**.",
      },
      "context/wiki/alpha.md",
    );
    expect(screen.getByRole("heading", { name: "Alpha" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
  });
});
