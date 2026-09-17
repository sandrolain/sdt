// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { resetKatexLoader, renderMath } from "./katexRender";

function mathNode(tex: string, display: "inline" | "block" = "inline"): HTMLElement {
  const span = document.createElement("span");
  span.className = display === "block" ? "md-math md-math--block" : "md-math";
  span.dataset.tex = tex;
  span.dataset.display = display;
  return span;
}

beforeEach(() => resetKatexLoader());
afterEach(() => {
  document.body.innerHTML = "";
  resetKatexLoader();
});

describe("renderMath", () => {
  it("renders inline math into KaTeX markup", async () => {
    const host = document.createElement("div");
    const node = mathNode("a^2 + b^2 = c^2");
    host.append(node);
    await renderMath(host);
    expect(node.querySelector(".katex")).toBeTruthy();
    expect(node.dataset.rendered).toBe("1");
    expect(node.title).toBe("a^2 + b^2 = c^2");
  });

  it("renders display math with the display class", async () => {
    const host = document.createElement("div");
    const node = mathNode("\\int_0^1 x\\,dx", "block");
    host.append(node);
    await renderMath(host);
    expect(node.querySelector(".katex-display")).toBeTruthy();
  });

  it("shows the source in place of invalid TeX instead of throwing", async () => {
    const host = document.createElement("div");
    const node = mathNode("\\frac{1}{");
    host.append(node);
    await renderMath(host);
    expect(node.classList.contains("md-math--error")).toBe(true);
    expect(node.querySelector(".md-math__error")).toBeTruthy();
    expect(node.querySelector(".md-math__source")?.textContent).toBe("\\frac{1}{");
  });

  it("ignores already-rendered nodes", async () => {
    const host = document.createElement("div");
    const node = mathNode("x");
    node.dataset.rendered = "1";
    host.append(node);
    await renderMath(host);
    expect(node.querySelector(".katex")).toBeNull();
  });

  it("is a no-op without math nodes or a root", async () => {
    await expect(renderMath(null)).resolves.toBeUndefined();
    await expect(renderMath(document.createElement("div"))).resolves.toBeUndefined();
  });
});
