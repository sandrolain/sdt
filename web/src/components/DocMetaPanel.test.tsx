// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { DocMetaPanel } from "./DocMetaPanel";
import { resetActiveSection, setActiveSection } from "../lib/activeSection";
import { resetCorpusIndexCache } from "../lib/corpusIndex";
import { clearFrontmatterCache } from "../lib/frontmatterYaml";
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
  clearFrontmatterCache();
  vi.restoreAllMocks();
});

describe("DocMetaPanel", () => {
  it("renders frontmatter labels, chips and resolved links", () => {
    mockFetch();
    renderPanel(DOC);
    expect(screen.getByText("Kind")).toBeTruthy();
    expect(screen.getByText("Notes")).toBeTruthy();
    expect(screen.getByText("alpha")).toBeTruthy();
    expect(screen.getByText("beta")).toBeTruthy();
    const link = screen.getAllByText("A")[0];
    expect(link.getAttribute("href")).toBe("/docs/context/analysis/a.md");
    // kind glyph + colour from the corpus path fallback, with the human label
    expect(screen.getAllByLabelText("Analyses").length).toBeGreaterThan(0);
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

  it("scopes a nested map value with its parent path", async () => {
    mockFetch();
    renderPanel({
      path: "context/notes/building.map.md",
      frontmatter: "---\ntitle: T\nmarkmap:\n  colorFreezeLevel: 2\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Markmap · Color Freeze Level")).toBeTruthy();
    expect(screen.getByText("2")).toBeTruthy();
  });

  it("warns and shows the raw block when the frontmatter does not parse", async () => {
    mockFetch();
    const frontmatter = "---\nderived_from:\n\t- a\n---\n";
    renderPanel({ path: "context/notes/broken.md", frontmatter, markdown: "body" });
    expect(await screen.findByText(/Not valid YAML/)).toBeTruthy();
    expect(screen.getByText("Raw frontmatter")).toBeTruthy();
    expect(document.querySelector(".meta-parse-warning__raw pre")?.textContent).toBe(frontmatter);
  });

  it("localises the legacy created_at key", () => {
    mockFetch();
    renderPanel({
      path: "context/notes/x.md",
      frontmatter: "---\nkind: notes\ncreated_at: 2026-09-15\n---\n",
      markdown: "body",
    });
    expect(screen.getByText("Created at")).toBeTruthy();
    // the raw ISO value is replaced by the localised date
    expect(screen.queryByText("2026-09-15")).toBeNull();
    expect(screen.getByText(/2026/)).toBeTruthy();
  });

  it("renders derived_from as a document link", () => {
    mockFetch();
    renderPanel({
      path: "context/notes/x.md",
      frontmatter: "---\nkind: notes\nderived_from:\n  - analysis/a.md\n---\n",
      markdown: "body",
    });
    expect(screen.getByText("A").getAttribute("href")).toBe("/docs/context/analysis/a.md");
  });

  it("exposes the image path on the thumbnail", () => {
    mockFetch();
    renderPanel({
      path: "context/notes/x.md",
      frontmatter: "---\nkind: notes\nimage: context/assets/cover.png\n---\n",
      markdown: "body",
    });
    const img = document.querySelector(".meta-row__image") as HTMLImageElement;
    expect(img.getAttribute("title")).toBe("context/assets/cover.png");
    expect(img.getAttribute("alt")).toBe("context/assets/cover.png");
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
    expect(screen.getByText("Progress")).toBeTruthy();
    expect(document.querySelector(".meta-status__dot--danger")).toBeTruthy();
  });

  it("shows the declared-vs-derived drift with the CLI remedy", async () => {
    // an analysis declared `active` whose plan and task files are all completed
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [
                {
                  path: "context/analysis/a.md",
                  kind: "analysis",
                  title: "A",
                  status: "active",
                },
                {
                  path: "context/plan/p.md",
                  kind: "plan",
                  title: "P",
                  status: "completed",
                  sources: ["analysis/a.md"],
                },
                {
                  path: "context/tasks/t.md",
                  kind: "tasks",
                  title: "T",
                  status: "completed",
                  sources: ["plan/p.md"],
                },
              ],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: "---\nkind: analysis\nstatus: active\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Analysis completed")).toBeTruthy();
    const block = document.querySelector(".meta-drift");
    expect(block?.textContent).toContain(
      "analysis is derivably completed (all plans done) — run `sdt context sync`",
    );
    // the declared row still shows what the frontmatter says
    expect(screen.getByText("Status")).toBeTruthy();
    expect(screen.getByText("Active")).toBeTruthy();
  });

  it("reports an out-of-vocabulary declared status as drift", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [
                { path: "context/analysis/a.md", kind: "analysis", title: "A", status: "resolved" },
              ],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: "---\nkind: analysis\nstatus: resolved\n---\n",
      markdown: "body",
    });
    // the effective state reads as archived, the declared row keeps the raw value
    await screen.findByText("Analysis archived");
    expect(screen.getByText("resolved")).toBeTruthy();
    expect(document.querySelector(".meta-drift__text")?.textContent).toContain(
      "declared status `resolved` is outside the analysis vocabulary",
    );
  });

  it("shows no drift block when the declared status agrees with the state", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [
                { path: "context/analysis/a.md", kind: "analysis", title: "A", status: "draft" },
              ],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: "---\nkind: analysis\nstatus: draft\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Analysis to be written")).toBeTruthy();
    expect(document.querySelector(".meta-drift")).toBeNull();
  });

  it("decorates the declared status with its label and meaning tooltip", async () => {
    mockFetch();
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: "---\nkind: analysis\nstatus: archived\n---\n",
      markdown: "body",
    });
    expect(screen.getByText("Status")).toBeTruthy();
    const value = screen.getByText("Archived");
    expect(value.getAttribute("title")).toBe("Superseded or retired by the user");
    expect(document.querySelector(".meta-status__dot--neutral")).toBeTruthy();
  });

  it("degrades an out-of-vocabulary status to neutral with the raw value", () => {
    mockFetch();
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: "---\nkind: analysis\nstatus: bogus\n---\n",
      markdown: "body",
    });
    const value = screen.getByText("bogus");
    expect(value.getAttribute("title")).toBe("bogus");
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
    expect(screen.getByText("Progress")).toBeTruthy();
    expect(document.querySelector(".meta-status__dot--danger")).toBeTruthy();
  });

  it("renders the categories field as a labelled chip row", () => {
    mockFetch();
    renderPanel({
      path: "context/analysis/a.md",
      frontmatter: [
        "---",
        "kind: analysis",
        "status: draft",
        "categories: [bug, refactor]",
        "---",
      ].join("\n"),
      markdown: "body",
    });
    expect(screen.getByText("Categories")).toBeTruthy();
    expect(screen.getByText("bug")).toBeTruthy();
    expect(screen.getByText("refactor")).toBeTruthy();
  });

  it("shows a draft analysis with the dedicated draft tone", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              entries: [{ path: "context/analysis/d.md", kind: "analysis", status: "draft" }],
            }),
        });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderPanel({
      path: "context/analysis/d.md",
      frontmatter: "---\nkind: analysis\nstatus: draft\n---\n",
      markdown: "body",
    });
    expect(await screen.findByText("Analysis to be written")).toBeTruthy();
    expect(document.querySelector(".meta-status__dot--draft")).toBeTruthy();
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
    expect(screen.getByText("Progress")).toBeTruthy();
    expect(document.querySelector(".meta-status__dot--neutral")).toBeTruthy();
  });

  it("shows the declared status but no derived progress row without a tree status", () => {
    mockFetch();
    renderPanel({
      path: "context/notes/x.md",
      frontmatter: "---\nkind: notes\nstatus: active\n---\n",
      markdown: "body",
    });
    expect(screen.getByText("Status")).toBeTruthy();
    expect(screen.queryByText("Progress")).toBeNull();
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
