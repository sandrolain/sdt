// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { DocMetaPanel } from "./DocMetaPanel";
import { resetWikiIndexCache } from "../lib/wikiIndexLoader";

const DOC = {
  path: "context/notes/x.md",
  frontmatter: [
    "---",
    "kind: notes",
    "title: X",
    "status: active",
    "created: 2026-09-15",
    "tags: [alpha, beta]",
    "sources:",
    "  - analysis/a.md",
    "  - context/commands/c.md",
    "---",
  ].join("\n"),
  markdown: "# First\n\ntext\n\n## Second\n",
};

function mockFetch() {
  globalThis.fetch = vi.fn((url: string) => {
    if (url === "/api/tree") {
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: [] }) });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  }) as unknown as typeof fetch;
}

function renderPanel(doc: unknown, relatedId?: string) {
  return render(
    <MemoryRouter>
      <DocMetaPanel doc={doc as never} relatedId={relatedId} />
    </MemoryRouter>,
  );
}

beforeEach(() => resetWikiIndexCache());
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("DocMetaPanel", () => {
  it("renders frontmatter labels, chips and resolved links", () => {
    mockFetch();
    renderPanel(DOC);
    expect(screen.getByText("Kind")).toBeTruthy();
    expect(screen.getByText("notes")).toBeTruthy();
    expect(screen.getByText("alpha")).toBeTruthy();
    expect(screen.getByText("beta")).toBeTruthy();
    const link = screen.getAllByText("A")[0];
    expect(link.getAttribute("href")).toBe("#/docs/context/analysis/a.md");
    const excluded = screen.getAllByText("C")[0];
    expect(excluded.getAttribute("href")).toBeNull();
    expect(excluded.getAttribute("title")).toBe("Excluded from the corpus");
  });

  it("lists heading sections and scrolls to the rendered heading", async () => {
    mockFetch();
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    render(
      <MemoryRouter>
        <DocMetaPanel doc={DOC as never} />
        <div className="doc-rendered">
          <h1>First</h1>
          <h2>Second</h2>
        </div>
      </MemoryRouter>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Second" }));
    expect(scrollIntoView).toHaveBeenCalledTimes(1);
  });

  it("shows canvas node/edge counts instead of frontmatter", () => {
    mockFetch();
    renderPanel({ path: "context/board.canvas", canvas: { nodes: [{}, {}], edges: [{}] } });
    expect(screen.getByText("Nodes")).toBeTruthy();
    expect(screen.getByText("2")).toBeTruthy();
    expect(screen.getByText("Edges")).toBeTruthy();
    expect(screen.getByText("1")).toBeTruthy();
  });

  it("shows an empty frontmatter state", () => {
    mockFetch();
    renderPanel({ path: "context/notes/x.md", frontmatter: "", markdown: "body" });
    expect(screen.getByText("No frontmatter.")).toBeTruthy();
  });

  it("renders the relations card for a wiki page", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url.startsWith("/api/wiki/rel")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ id: "a", title: "A", outbound: {}, inbound: {} }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel(DOC, "a");
    const related = await screen.findByText("No relations.");
    expect(related).toBeTruthy();
    expect(within(screen.getByLabelText("Document metadata")).getByText("Related")).toBeTruthy();
  });
});
