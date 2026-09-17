// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderMermaid, resetMermaidLoader } from "./mermaidRender";

const mocks = vi.hoisted(() => ({
  initialize: vi.fn(),
  render: vi.fn(),
}));

vi.mock("mermaid", () => ({
  default: {
    initialize: (...args: unknown[]) => mocks.initialize(...args),
    render: (...args: unknown[]) => mocks.render(...args),
  },
}));

function mermaidNode(source: string): HTMLElement {
  const div = document.createElement("div");
  div.className = "md-mermaid";
  div.dataset.src = encodeURIComponent(source);
  return div;
}

beforeEach(() => {
  mocks.initialize.mockClear();
  mocks.render.mockReset();
  mocks.render.mockResolvedValue({ svg: '<svg data-testid="diagram"></svg>' });
  document.documentElement.style.setProperty("--bg-base", "#111111");
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
    const node = mermaidNode("flowchart LR\n  A --> B");
    host.append(node);
    await renderMermaid(host);
    expect(mocks.initialize).toHaveBeenCalled();
    expect(node.dataset.rendered).toBe("1");
    expect(node.querySelector("svg")).toBeTruthy();
    expect(node.querySelector(".md-mermaid__download")).toBeTruthy();
  });

  it("falls back to the source when rendering fails", async () => {
    mocks.render.mockRejectedValueOnce(new Error("boom"));
    const host = document.createElement("div");
    const node = mermaidNode("broken diagram");
    host.append(node);
    await renderMermaid(host);
    expect(node.classList.contains("md-mermaid--error")).toBe(true);
    expect(node.querySelector(".md-mermaid__source")?.textContent).toBe("broken diagram");
  });

  it("skips already-rendered nodes within the same theme", async () => {
    const host = document.createElement("div");
    const first = mermaidNode("graph TD\n  A");
    host.append(first);
    await renderMermaid(host); // establishes the theme signature
    mocks.render.mockClear();
    const second = mermaidNode("graph TD\n  B");
    second.dataset.rendered = "1";
    host.append(second);
    await renderMermaid(host);
    expect(mocks.render).not.toHaveBeenCalled();
  });

  it("re-renders every diagram when the resolved theme changes", async () => {
    const host = document.createElement("div");
    const node = mermaidNode("graph TD\n  A");
    host.append(node);
    await renderMermaid(host);
    expect(mocks.render).toHaveBeenCalledTimes(1);
    document.documentElement.style.setProperty("--bg-base", "#222222");
    await renderMermaid(host);
    expect(mocks.render).toHaveBeenCalledTimes(2);
  });

  it("is a no-op without diagrams or a root", async () => {
    await expect(renderMermaid(null)).resolves.toBeUndefined();
    await expect(renderMermaid(document.createElement("div"))).resolves.toBeUndefined();
    expect(mocks.render).not.toHaveBeenCalled();
  });
});
