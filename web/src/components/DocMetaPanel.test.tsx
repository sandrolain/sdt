// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { DocMetaPanel } from "./DocMetaPanel";
import { resetActiveSection, setActiveSection } from "../lib/activeSection";
import { resetCorpusIndexCache } from "../lib/corpusIndex";
import { getSectionRequest, resetSectionRequest } from "../lib/sectionRequests";
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

beforeEach(() => {
  resetWikiIndexCache();
  resetCorpusIndexCache();
});
afterEach(() => {
  cleanup();
  resetActiveSection();
  resetSectionRequest();
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
    expect(link.getAttribute("href")).toBe("/docs/context/analysis/a.md");
    // kind glyph + colour from the corpus path fallback
    expect(screen.getAllByLabelText("kind: analysis").length).toBeGreaterThan(0);
    // sources under context/commands resolve as real corpus links (kind commands)
    const cmd = screen.getAllByText("C")[0];
    expect(cmd.getAttribute("href")).toBe("/docs/context/commands/c.md");
    expect(cmd.getAttribute("title")).toBeNull();
    // each meta card carries an open/collapsed chevron
    const chevrons = document.querySelectorAll(".meta-card__chevron");
    expect(chevrons.length).toBe(document.querySelectorAll(".meta-card").length);
    expect(chevrons.length).toBeGreaterThan(0);
  });

  it("renders the Sections card above the Metadata card", () => {
    mockFetch();
    renderPanel(DOC);
    const titles = Array.from(document.querySelectorAll(".meta-card__label")).map(
      (el) => el.textContent,
    );
    expect(titles.indexOf("Sections")).toBeGreaterThanOrEqual(0);
    expect(titles.indexOf("Sections")).toBeLessThan(titles.indexOf("Metadata"));
  });

  it("requests the selected section (jump handled by the document panel)", async () => {
    mockFetch();
    renderPanel(DOC);
    await userEvent.click(screen.getByRole("button", { name: "Second" }));
    expect(getSectionRequest()).toMatchObject({ path: DOC.path, text: "Second" });
  });

  it("renders nested relation verbs with a human label", () => {
    mockFetch();
    const doc = {
      path: "context/wiki/a.md",
      frontmatter: [
        "---",
        "kind: wiki",
        "relations:",
        "  part_of:",
        '    - "[[b|Beta]]"',
        "---",
      ].join("\n"),
      markdown: "body",
    };
    renderPanel(doc);
    expect(screen.getByText("Part of")).toBeTruthy();
    expect(screen.getByText("b")).toBeTruthy();
  });

  it("marks the link to the current route with aria-current", () => {
    mockFetch();
    render(
      <MemoryRouter initialEntries={["/docs/context/analysis/a.md"]}>
        <DocMetaPanel doc={DOC as never} />
      </MemoryRouter>,
    );
    const current = screen
      .getAllByText("A")
      .filter((el) => el.getAttribute("aria-current") === "page");
    expect(current.length).toBeGreaterThan(0);
    expect(current[0].className).toContain("is-current");
  });

  it("highlights the heading currently in view", () => {
    mockFetch();
    renderPanel(DOC);
    expect(screen.getByRole("button", { name: "Second" }).getAttribute("aria-current")).toBeNull();
    act(() => setActiveSection(DOC.path, "Second"));
    const active = screen.getByRole("button", { name: "Second" });
    expect(active.getAttribute("aria-current")).toBe("true");
    expect(active.className).toContain("is-active");
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

  it("renders the status dot row for plans/tasks from the corpus index", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [
                {
                  path: "context/plan/p.md",
                  kind: "plan",
                  title: "P",
                  status: "active",
                },
              ],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/plan/p.md",
      frontmatter: "---\nkind: plan\nstatus: active\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Plan not started")).toBeTruthy();
    expect(screen.getAllByText("Status").length).toBeGreaterThan(0);
    expect(document.querySelector(".meta-status__dot--danger")).toBeTruthy();
  });

  it("renders question lifecycle status in the metadata panel", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [{ path: "context/questions/q.md", kind: "questions", status: "active" }],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/questions/q.md",
      frontmatter: "---\nkind: questions\nstatus: active\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Question unresolved")).toBeTruthy();
    expect(document.querySelector(".meta-status__dot--danger")).toBeTruthy();
  });

  it("uses the neutral tone for an archived analysis metadata dot", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [{ path: "context/analysis/a.md", kind: "analysis", status: "archived" }],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: "---\nkind: analysis\nstatus: archived\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Analysis archived")).toBeTruthy();
    expect(document.querySelector(".meta-status__dot--neutral")).toBeTruthy();
  });

  it("omits the status row for kinds without a tree status", () => {
    mockFetch();
    renderPanel({
      path: "context/notes/x.md",
      frontmatter: "---\nkind: notes\n---\n",
      markdown: "body",
    });
    expect(document.querySelector(".meta-status")).toBeNull();
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
