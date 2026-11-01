// @vitest-environment jsdom
import { beforeAll, describe, expect, it, vi } from "vitest";
import { render } from "pele";
import { renderMarkdown } from "./markdown";
import { DIAGRAM_SANITIZE_CONFIG, sanitizeDiagramSvg } from "./diagramSanitize";

beforeAll(() => {
  // jsdom has no canvas: stub it so Pele falls back to its metrics table
  // silently instead of logging "getContext() not implemented" per render.
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
});

function rendered(source: string): string {
  return sanitizeDiagramSvg(render(source).svg);
}

describe("sanitizeDiagramSvg", () => {
  it("keeps the SVG, its shapes and the CSS-variable theme bindings", () => {
    const clean = rendered("flowchart LR\n  A[Start] --> B{Choice}");
    const host = document.createElement("div");
    host.innerHTML = clean;
    expect(host.querySelector("svg")).toBeTruthy();
    expect(host.querySelectorAll("rect, polygon").length).toBeGreaterThan(0);
    expect(host.querySelector("text")?.textContent).toBe("Start");
    expect(clean).toContain("var(--pele-");
  });

  it("drops a script injected into a label", () => {
    const clean = sanitizeDiagramSvg(
      '<svg xmlns="http://www.w3.org/2000/svg"><text><script>alert(1)</script>ok</text></svg>',
    );
    expect(clean).not.toContain("<script");
    expect(clean).toContain("ok");
  });

  it("drops event handlers and javascript: links", () => {
    const clean = sanitizeDiagramSvg(
      '<svg xmlns="http://www.w3.org/2000/svg"><a href="javascript:alert(1)"><text onload="alert(2)">x</text></a></svg>',
    );
    expect(clean).not.toContain("onload");
    expect(clean).not.toContain("javascript:");
  });

  it("does not widen the global allowlist", () => {
    expect(renderMarkdown("<svg><text>x</text></svg>")).not.toContain("<svg");
    expect(DIAGRAM_SANITIZE_CONFIG.ALLOWED_TAGS).toContain("svg");
  });
});
