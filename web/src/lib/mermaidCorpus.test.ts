// @vitest-environment jsdom
import { beforeAll, describe, expect, it, vi } from "vitest";
import { render } from "pele";
import { sanitizeDiagramSvg } from "./diagramSanitize";
import { renderMarkdown } from "./markdown";

/**
 * The corpus contract for the diagram lane: every fenced `mermaid` block in the
 * served corpus becomes a `.md-mermaid` placeholder, and each of the three
 * kinds the corpus actually uses renders to an `<svg>`. No test asserts a
 * diagram's shape — the layout is the renderer's, and a parity check is a
 * browser pass (plan `20261030-103357`, phase 5).
 */
const CORPUS_SOURCES = {
  // context/notes/20260917-145548-markdown-rendering-showcase.md
  flowchart: "flowchart LR\n  A[Tree] --> B[Document tabs]\n  B --> C[Render]\n  B --> D[Code]",
  sequenceDiagram:
    "sequenceDiagram\n  participant U as User\n  participant S as sdtviewer\n  U->>S: GET /api/tree\n  S-->>U: entries[]",
  erDiagram: "erDiagram\n  TREE ||--o{ ENTRY : lists\n  ENTRY ||--o{ LINK : contains",
} as const;

beforeAll(() => {
  // jsdom has no canvas: stub it so Pele falls back to its metrics table
  // silently instead of logging "getContext() not implemented" per render.
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
});

function placeholder(source: string): HTMLElement {
  const html = renderMarkdown("```mermaid\n" + source + "\n```\n");
  const host = document.createElement("div");
  host.innerHTML = html;
  const node = host.querySelector<HTMLElement>(".md-mermaid");
  if (!node) throw new Error("markdown.ts emitted no .md-mermaid placeholder");
  return node;
}

describe("mermaid corpus contract", () => {
  for (const [kind, source] of Object.entries(CORPUS_SOURCES)) {
    it(`renders a ${kind} block to an svg inside its placeholder`, () => {
      const node = placeholder(source);
      expect(node.dataset.src).toBeTruthy();
      expect(decodeURIComponent(node.dataset.src ?? "")).toBe(source);

      node.innerHTML = sanitizeDiagramSvg(render(source).svg);
      expect(node.classList.contains("md-mermaid")).toBe(true);
      expect(node.querySelector("svg")).toBeTruthy();
      expect(node.textContent?.trim()).not.toBe("");
    });
  }

  it("binds the rendered markup to the CSS-variable theme", () => {
    const { svg } = render(CORPUS_SOURCES.flowchart);
    const clean = sanitizeDiagramSvg(svg);
    expect(clean).toContain("var(--pele-bg");
    expect(clean).toContain("var(--pele-line");
    expect(clean).toContain("var(--pele-surface");
  });

  it("falls back to the source for a diagram Pele cannot parse", () => {
    expect(() => render("not a diagram at all")).toThrow();
  });
});
