// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { DocumentView } from "./DocumentView";
import { getSectionRequest, requestSection, resetSectionRequest } from "../lib/sectionRequests";
import { activeSectionKey, resetActiveSection } from "../lib/activeSection";
import { clearReadingState, flushReading, recordReading } from "../lib/readingState";
import { closeFind, openFind, resetFindOptions, setFindQuery } from "../lib/findInDocStore";

vi.mock("./MindmapView", () => ({
  MindmapView: ({ title }: { title?: string }) => (
    <div role="img" aria-label={`Mindmap: ${title ?? ""}`} />
  ),
}));

vi.mock("./SlidesView", () => ({
  SlidesView: ({ markdown }: { markdown: string }) => (
    <div data-testid="slides-view" data-len={markdown.length} />
  ),
}));

vi.mock("../lib/mermaidRender", () => ({
  renderMermaid: vi.fn(() => Promise.resolve()),
}));

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="probe">{loc.pathname + loc.search + loc.hash}</span>;
}

function renderView(
  props: Partial<Parameters<typeof DocumentView>[0]> & { path: string },
  initial = "/docs/x",
) {
  render(
    <MemoryRouter initialEntries={[initial]}>
      <Routes>
        <Route
          path="/docs/*"
          element={
            <DocumentView
              path={props.path}
              markdown={props.markdown ?? "# Title\n\nHello **tokens**."}
              frontmatter={props.frontmatter ?? "---\nkind: wiki\n---\n"}
              isMap={props.isMap}
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  resetActiveSection();
  resetSectionRequest();
  clearReadingState();
  closeFind();
  setFindQuery("");
  resetFindOptions();
});

afterEach(() => {
  cleanup();
  closeFind();
  setFindQuery("");
  resetFindOptions();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("DocumentView", () => {
  it("defaults ordinary markdown to Render mode", () => {
    renderView({ path: "context/wiki/alpha.md" });
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
    expect(screen.getByText(/Hello/)).toBeTruthy();
    // the leading `# Title` is the document title, not repeated in the body
    expect(screen.queryByRole("heading", { name: "Title" })).toBeNull();
  });

  it("defaults map documents to Map mode with a map icon and renders the mindmap", async () => {
    renderView({ path: "context/wiki/topic.map.md", markdown: "# Top\n\n- item\n" });
    expect(screen.getByRole("button", { name: "Map", pressed: true })).toBeTruthy();
    expect(screen.getByRole("img", { name: "Map document" })).toBeTruthy();
    expect(await screen.findByRole("img", { name: "Mindmap: topic.map" })).toBeTruthy();
  });

  it("offers Map mode only for .map.md documents", () => {
    renderView({ path: "context/wiki/alpha.md" }, "/docs/x?view=map");
    expect(screen.queryByRole("button", { name: "Map" })).toBeNull();
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
  });

  it("defaults .mmd documents to Mermaid mode with Code, no Render/Map", () => {
    renderView({ path: "context/wiki/flow.mmd", markdown: "flowchart TD\n  A-->B\n" });
    expect(screen.getByRole("button", { name: "Mermaid", pressed: true })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Code" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Render" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Map" })).toBeNull();
    expect(screen.getByRole("img", { name: "Mermaid document" })).toBeTruthy();
    expect(document.querySelector(".doc-rendered--mermaid .md-mermaid")).toBeTruthy();
  });

  it("defaults .slide.md decks to Slides mode with Code+Render, no Map/Mermaid", async () => {
    renderView({ path: "context/notes/deck.slide.md", markdown: "# One\n\n---\n\n# Two\n" });
    expect(screen.getByRole("button", { name: "Slides", pressed: true })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Code" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Render" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Map" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Mermaid" })).toBeNull();
    expect(screen.getByRole("img", { name: "Slide deck" })).toBeTruthy();
    expect(await screen.findByTestId("slides-view")).toBeTruthy();
  });

  it("offers Slides mode only for .slide.md documents", () => {
    renderView({ path: "context/wiki/alpha.md" }, "/docs/x?view=slides");
    expect(screen.queryByRole("button", { name: "Slides" })).toBeNull();
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
  });

  it("still opens a deck in Render and Code modes", async () => {
    renderView({ path: "context/notes/deck.slide.md", markdown: "# One\n\nbody\n" });
    await userEvent.click(screen.getByRole("button", { name: "Render" }));
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    expect(screen.getByRole("button", { name: "Code", pressed: true })).toBeTruthy();
  });

  it("shows the raw mermaid source in Code mode for .mmd documents", async () => {
    renderView({ path: "context/wiki/flow.mmd", markdown: "flowchart TD\n  A-->B\n" });
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    expect(screen.getByRole("button", { name: "Code", pressed: true })).toBeTruthy();
    expect(screen.getByText(/flowchart TD/)).toBeTruthy();
    // .mmd has no frontmatter: the code surface is the source only
    const code = document.querySelector("pre.doc-code code") as HTMLElement;
    expect(code.textContent).toBe("flowchart TD\n  A-->B\n");
  });

  it("copies a rendered code block from its copy button", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    renderView({ path: "context/wiki/alpha.md", markdown: "```js\nconst x = 1;\n```" });
    await userEvent.click(screen.getByRole("button", { name: "Copy code" }));
    expect(writeText).toHaveBeenCalledWith("const x = 1;");
    expect(screen.getByRole("button", { name: "Copied" })).toBeTruthy();
  });

  it("copies a multi-line block verbatim, one line per source line", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    const source = "const a = 1;\nconst b = 2;\nreturn a + b;";
    renderView({ path: "context/wiki/alpha.md", markdown: `\`\`\`js\n${source}\n\`\`\`` });
    await userEvent.click(screen.getByRole("button", { name: "Copy code" }));
    expect(writeText).toHaveBeenCalledWith(source);
  });

  it("switches to Code mode showing highlighted raw markdown", async () => {
    renderView({ path: "context/wiki/alpha.md" });
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    expect(screen.getByRole("button", { name: "Code", pressed: true })).toBeTruthy();
    expect(screen.getByText(/Hello/)).toBeTruthy();
  });

  it("shows the whole file (frontmatter plus body) in Code mode with file-true line numbers", async () => {
    const frontmatter = "---\nkind: wiki\nstatus: active\n---\n";
    const markdown = "# Title\n\nHello **tokens**.";
    renderView({ path: "context/wiki/alpha.md", frontmatter, markdown });
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    const code = document.querySelector("pre.doc-code code") as HTMLElement;
    expect(code.textContent).toBe(frontmatter + markdown);
    const gutter = document.querySelector("pre.doc-code__gutter") as HTMLElement;
    const count = (frontmatter + markdown).split("\n").length;
    expect(gutter.textContent).toBe(Array.from({ length: count }, (_, i) => i + 1).join("\n"));
  });

  it("shows the whole file in Code mode for .map.md documents", async () => {
    const frontmatter = "---\nmarkmap:\n  colorFreezeLevel: 2\n---\n";
    const markdown = "# Top\n\n- item\n";
    renderView({ path: "context/wiki/topic.map.md", frontmatter, markdown });
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    const code = document.querySelector("pre.doc-code code") as HTMLElement;
    expect(code.textContent).toBe(frontmatter + markdown);
  });

  it("renders exactly one pressed mode button (exclusive group)", () => {
    renderView({ path: "context/wiki/alpha.md" });
    expect(screen.getAllByRole("button", { pressed: true })).toHaveLength(1);
  });

  it("toggles long-line wrapping on a code block", async () => {
    renderView({ path: "context/wiki/alpha.md", markdown: "```js\nconst x = 1;\n```" });
    const pre = document.querySelector("pre.md-code") as HTMLElement;
    expect(pre.classList.contains("is-wrapped")).toBe(false);
    await userEvent.click(screen.getByRole("button", { name: "Toggle line wrapping" }));
    expect(pre.classList.contains("is-wrapped")).toBe(true);
  });

  it("clears the find query when the open document changes", async () => {
    const { rerender } = render(
      <MemoryRouter>
        <DocumentView path="context/notes/a.md" markdown={"# A\n\n## One\n\ntokens\n"} />
      </MemoryRouter>,
    );
    act(() => openFind());
    const input = (await screen.findByRole("textbox", {
      name: "Find in document",
    })) as HTMLInputElement;
    await userEvent.type(input, "tokens");
    await userEvent.click(screen.getByRole("button", { name: "Match case" }));
    expect(input.value).toBe("tokens");
    expect(screen.getByRole("button", { name: "Match case" }).getAttribute("aria-pressed")).toBe(
      "true",
    );

    rerender(
      <MemoryRouter>
        <DocumentView path="context/notes/b.md" markdown={"# B\n\n## Two\n\nother\n"} />
      </MemoryRouter>,
    );
    const switched = (await screen.findByRole("textbox", {
      name: "Find in document",
    })) as HTMLInputElement;
    await waitFor(() => expect(switched.value).toBe(""));
    expect(screen.getByRole("button", { name: "Match case" }).getAttribute("aria-pressed")).toBe(
      "true",
    );
  });

  it("copies a deep link from a heading anchor", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    renderView({
      path: "context/notes/x.md",
      markdown: "# Title\n\n## Section One\n\ntext\n",
    });
    await userEvent.click(screen.getAllByRole("button", { name: "Copy link to section" })[0]);
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("#section-one"));
  });

  it("renders inline math with KaTeX in render mode", async () => {
    renderView({ path: "context/notes/m.md", markdown: "# Title\n\nInline $a^2 + b^2$ math.\n" });
    await waitFor(() => expect(document.querySelector(".doc-rendered .katex")).toBeTruthy());
  });

  it("switches to render mode and scrolls when a section is requested", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    renderView(
      { path: "context/notes/x.md", markdown: "# Alpha\n\n## Beta\n\ntext\n" },
      "/docs/context/notes/x.md?view=code",
    );
    expect(screen.getByRole("button", { name: "Code", pressed: true })).toBeTruthy();

    act(() => requestSection("context/notes/x.md", "Beta"));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
    });
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
    expect(getSectionRequest()).toBeNull();
  });

  it("scrolls to a heading whose normalised text matches, inline code included", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    renderView({
      path: "context/notes/code.md",
      markdown: "# Top\n\n## Option A: fold into `development.md`\n\ntext\n",
    });

    act(() => requestSection("context/notes/code.md", "Option A: fold into development.md"));
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
    expect(getSectionRequest()).toBeNull();
  });

  it("renders a mindmap for plain prose in Map mode", async () => {
    renderView({ path: "context/wiki/prose.md", isMap: true, markdown: "just prose" });
    expect(await screen.findByRole("img", { name: "Mindmap: prose" })).toBeTruthy();
  });

  describe("reading position", () => {
    const LONG = "# Title\n\n## One\n\ntext\n\n## Two\n\ntext\n\n## Three\n\ntext\n";

    /** jsdom reports zero for every layout box, so fake the scroll metrics. */
    function stubScrollMetrics(scrollHeight: number, clientHeight: number) {
      vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(scrollHeight);
      vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(clientHeight);
    }

    function renderLong(path: string) {
      const { container } = render(
        <MemoryRouter>
          <DocumentView path={path} markdown={LONG} />
        </MemoryRouter>,
      );
      // `.doc-rendered` is the scroll container; the article is only its parent
      return container.querySelector(".doc-rendered") as HTMLElement;
    }

    it("restores the stored offset for the document", () => {
      stubScrollMetrics(4000, 800);
      recordReading("context/wiki/alpha.md", 640, "Two", 0);
      expect(renderLong("context/wiki/alpha.md").scrollTop).toBe(640);
    });

    it("clamps a stored offset the shorter document cannot reach", () => {
      stubScrollMetrics(1200, 800);
      recordReading("context/wiki/alpha.md", 5000, "Two", 0);
      expect(renderLong("context/wiki/alpha.md").scrollTop).toBe(400);
    });

    it("leaves an unread document at the top", () => {
      stubScrollMetrics(4000, 800);
      expect(renderLong("context/wiki/alpha.md").scrollTop).toBe(0);
    });

    it("records the offset and the heading in view while scrolling", async () => {
      stubScrollMetrics(4000, 800);
      const article = renderLong("context/wiki/alpha.md");
      article.scrollTop = 250;
      await act(async () => {
        article.dispatchEvent(new Event("scroll"));
        await new Promise((resolve) => requestAnimationFrame(resolve));
      });
      flushReading();
      const stored = JSON.parse(localStorage.getItem("sdt-reading") ?? "{}");
      expect(stored.positions["context/wiki/alpha.md"].scrollTop).toBe(250);
      // the heading travels with the offset, so a restore can fall back to it
      expect(stored.positions["context/wiki/alpha.md"].headingId).toBe(
        activeSectionKey("context/wiki/alpha.md"),
      );
      expect(stored.recent[0]).toBe("context/wiki/alpha.md");
    });
  });
  describe("deep links", () => {
    const LONG = "# Title\n\n## One\n\ntext\n\n## Findings\n\ntext\n\n";

    // the fragment is read from the real location, so every case resets it
    beforeEach(() => window.history.replaceState(null, "", "/docs/x"));
    afterEach(() => window.history.replaceState(null, "", "/docs/x"));

    it("resolves a fragment to its heading and asks for the scroll", async () => {
      const scrollIntoView = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoView;
      window.history.replaceState(null, "", "/docs/context/notes/x.md#findings");
      render(
        <MemoryRouter initialEntries={["/docs/context/notes/x.md#findings"]}>
          <DocumentView path="context/notes/x.md" markdown={LONG} />
        </MemoryRouter>,
      );
      await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
      expect(getSectionRequest()).toBeNull();
    });

    it("keeps the fragment when the deep link has to switch to render mode", async () => {
      const scrollIntoView = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoView;
      window.history.replaceState(null, "", "/docs/context/notes/x.md?view=code#findings");
      render(
        <MemoryRouter initialEntries={["/docs/context/notes/x.md?view=code#findings"]}>
          <Routes>
            <Route path="/docs/*" element={<LocationProbe />} />
          </Routes>
          <DocumentView path="context/notes/x.md" markdown={LONG} />
        </MemoryRouter>,
      );
      // the router location is what changes; window.location is untouched
      // under MemoryRouter
      await waitFor(() => {
        expect(screen.getByTestId("probe").textContent).toContain("#findings");
      });
      expect(screen.getByTestId("probe").textContent).toContain("view=render");
    });

    it("ignores a fragment with no matching heading", async () => {
      const scrollIntoView = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoView;
      render(
        <MemoryRouter initialEntries={["/docs/x#no-such-section"]}>
          <DocumentView path="context/notes/x.md" markdown={LONG} />
        </MemoryRouter>,
      );
      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it("prefers the deep link over the remembered offset", () => {
      // with a fragment present the reader lands on the section, not the offset
      recordReading("context/notes/x.md", 900, "Findings", 0);
      const article = document.createElement("div");
      Object.defineProperty(article, "scrollHeight", { value: 4000, configurable: true });
      Object.defineProperty(article, "clientHeight", { value: 800, configurable: true });
      document.body.appendChild(article);
      window.history.replaceState(null, "", "/docs/context/notes/x.md#findings");
      render(
        <MemoryRouter initialEntries={["/docs/context/notes/x.md#findings"]}>
          <DocumentView path="context/notes/x.md" markdown={LONG} />
        </MemoryRouter>,
      );
      expect(getSectionRequest()).toBeNull();
      document.body.innerHTML = "";
    });
  });
});
