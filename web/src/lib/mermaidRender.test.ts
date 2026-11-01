// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderMermaid, resetMermaidLoader, serializeDiagramSvg } from "./mermaidRender";

const mocks = vi.hoisted(() => ({
  render: vi.fn(),
}));

vi.mock("pele", () => ({
  render: (...args: unknown[]) => mocks.render(...args),
}));

const panZoom = vi.hoisted(() => ({
  instance: {
    zoomIn: vi.fn(),
    zoomOut: vi.fn(),
    reset: vi.fn(),
    destroy: vi.fn(),
  },
}));

vi.mock("svg-pan-zoom", () => ({
  default: vi.fn(() => panZoom.instance),
}));

const FLOWCHART = "flowchart LR\n  A --> B";

function mermaidNode(source: string): HTMLElement {
  const div = document.createElement("div");
  div.className = "md-mermaid";
  div.dataset.src = encodeURIComponent(source);
  return div;
}

beforeEach(() => {
  mocks.render.mockReset();
  mocks.render.mockReturnValue({
    svg: '<svg xmlns="http://www.w3.org/2000/svg" data-testid="diagram"></svg>',
    width: 100,
    height: 50,
    type: "flowchart",
    mermaidType: "flowchart",
    links: [],
  });
  resetMermaidLoader();
});

afterEach(() => {
  document.body.innerHTML = "";
  document.documentElement.removeAttribute("style");
  resetMermaidLoader();
});

describe("renderMermaid", () => {
  it("renders a diagram and marks the node", async () => {
    const host = document.createElement("div");
    const node = mermaidNode(FLOWCHART);
    host.append(node);
    await renderMermaid(host);
    expect(mocks.render).toHaveBeenCalledWith(FLOWCHART, expect.anything());
    expect(node.dataset.rendered).toBe("1");
    expect(node.querySelector("svg")).toBeTruthy();
    expect(node.querySelector(".md-mermaid__download")).toBeTruthy();
    expect(node.querySelector(".md-mermaid__controls")).toBeTruthy();
  });

  it("sanitizes the renderer's SVG before it reaches the DOM", async () => {
    mocks.render.mockReturnValueOnce({
      svg: '<svg xmlns="http://www.w3.org/2000/svg"><text><script>alert(1)</script>ok</text></svg>',
      width: 10,
      height: 10,
      type: "flowchart",
      mermaidType: "flowchart",
      links: [],
    });
    const host = document.createElement("div");
    const node = mermaidNode(FLOWCHART);
    host.append(node);
    await renderMermaid(host);
    expect(node.innerHTML).not.toContain("<script");
    expect(node.textContent).toContain("ok");
  });

  it("falls back to the source when rendering fails", async () => {
    mocks.render.mockImplementationOnce(() => {
      throw new Error("boom");
    });
    const host = document.createElement("div");
    const node = mermaidNode("broken diagram");
    host.append(node);
    await renderMermaid(host);
    expect(node.classList.contains("md-mermaid--error")).toBe(true);
    expect(node.querySelector(".md-mermaid__source")?.textContent).toBe("broken diagram");
  });

  it("skips already-rendered nodes", async () => {
    const host = document.createElement("div");
    const first = mermaidNode("graph TD\n  A");
    host.append(first);
    await renderMermaid(host);
    mocks.render.mockClear();
    const second = mermaidNode("graph TD\n  B");
    second.dataset.rendered = "1";
    host.append(second);
    await renderMermaid(host);
    expect(mocks.render).not.toHaveBeenCalled();
  });

  it("does not re-render when the theme changes: CSS variables restyle in place", async () => {
    const host = document.createElement("div");
    const node = mermaidNode("graph TD\n  A");
    host.append(node);
    await renderMermaid(host);
    expect(mocks.render).toHaveBeenCalledTimes(1);
    document.documentElement.style.setProperty("--bg-base", "#222222");
    await renderMermaid(host);
    expect(mocks.render).toHaveBeenCalledTimes(1);
    expect(node.dataset.rendered).toBe("1");
  });

  it("is a no-op without diagrams or a root", async () => {
    await expect(renderMermaid(null)).resolves.toBeUndefined();
    await expect(renderMermaid(document.createElement("div"))).resolves.toBeUndefined();
    expect(mocks.render).not.toHaveBeenCalled();
  });
});

describe("serializeDiagramSvg", () => {
  it("resolves the CSS-variable theme into the downloaded SVG", () => {
    // jsdom does not cascade custom properties from :root into a descendant's
    // computed style, so the resolved values are stubbed; the browser pass
    // covers the real cascade.
    vi.spyOn(window, "getComputedStyle").mockReturnValue({
      getPropertyValue: (name: string) => (name === "--pele-bg" ? "#123456" : ""),
    } as unknown as CSSStyleDeclaration);
    const host = document.createElement("div");
    host.innerHTML =
      '<svg xmlns="http://www.w3.org/2000/svg" style="--_bg:var(--pele-bg,#fff)"><rect fill="var(--_bg)"/></svg>';
    const svg = host.querySelector("svg");
    if (!svg) throw new Error("no svg");
    const out = serializeDiagramSvg(svg);
    expect(out).toContain("--pele-bg: #123456");
    // the clone is a copy: the live diagram is untouched
    expect(svg.getAttribute("style")).not.toContain("--pele-bg: #123456");
  });
});
