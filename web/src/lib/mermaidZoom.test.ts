// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import svgPanZoom from "svg-pan-zoom";
import { destroyMermaidZoom, enhanceMermaid, openFullscreen } from "./mermaidZoom";

interface Api {
  zoomIn: ReturnType<typeof vi.fn>;
  zoomOut: ReturnType<typeof vi.fn>;
  reset: ReturnType<typeof vi.fn>;
  destroy: ReturnType<typeof vi.fn>;
}

const panZoom = vi.hoisted(() => ({ instances: [] as unknown[] }));

vi.mock("svg-pan-zoom", () => ({
  default: vi.fn(() => {
    const api = {
      zoomIn: vi.fn(),
      zoomOut: vi.fn(),
      reset: vi.fn(),
      destroy: vi.fn(),
    };
    panZoom.instances.push(api);
    return api;
  }),
}));

function apiAt(index: number): Api {
  return panZoom.instances[index] as Api;
}

function svgNode(): HTMLElement {
  const node = document.createElement("div");
  node.className = "md-mermaid";
  node.innerHTML = "<svg><g></g></svg>";
  return node;
}

beforeEach(() => {
  panZoom.instances.length = 0;
  vi.mocked(svgPanZoom).mockClear();
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("enhanceMermaid", () => {
  it("attaches four controls and is idempotent per node", () => {
    const node = svgNode();
    const handle = enhanceMermaid(node);
    expect(handle).not.toBeNull();
    expect(node.classList.contains("md-mermaid--zoomable")).toBe(true);
    expect(node.querySelectorAll(".md-mermaid__control")).toHaveLength(4);
    enhanceMermaid(node);
    expect(svgPanZoom).toHaveBeenCalledTimes(1);
  });

  it("returns null when the node has no SVG", () => {
    const node = document.createElement("div");
    node.className = "md-mermaid";
    expect(enhanceMermaid(node)).toBeNull();
  });

  it("wires the zoom controls to the underlying instance", () => {
    const node = svgNode();
    enhanceMermaid(node);
    const api = apiAt(0);
    const byLabel = (label: string) =>
      node.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`) as HTMLButtonElement;
    byLabel("Zoom in").click();
    byLabel("Zoom out").click();
    byLabel("Reset view").click();
    expect(api.zoomIn).toHaveBeenCalledOnce();
    expect(api.zoomOut).toHaveBeenCalledOnce();
    expect(api.reset).toHaveBeenCalledOnce();
  });

  it("destroys the instance and removes controls", () => {
    const node = svgNode();
    enhanceMermaid(node);
    destroyMermaidZoom(node);
    expect(apiAt(0).destroy).toHaveBeenCalledOnce();
    expect(node.querySelector(".md-mermaid__controls")).toBeNull();
    expect(node.classList.contains("md-mermaid--zoomable")).toBe(false);
  });
});

describe("openFullscreen", () => {
  it("opens an overlay with controls and closes on Escape", () => {
    const svg = svgNode().querySelector("svg") as SVGSVGElement;
    openFullscreen(svg);
    const overlay = document.querySelector(".mermaid-fullscreen");
    expect(overlay).toBeTruthy();
    expect(overlay?.querySelectorAll(".md-mermaid__control").length).toBeGreaterThan(0);
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(document.querySelector(".mermaid-fullscreen")).toBeNull();
    expect(apiAt(0).destroy).toHaveBeenCalledOnce();
  });

  it("closes when the close control is clicked", () => {
    const svg = svgNode().querySelector("svg") as SVGSVGElement;
    openFullscreen(svg);
    const close = document.querySelector<HTMLButtonElement>(
      '.mermaid-fullscreen button[aria-label="Close"]',
    );
    close?.click();
    expect(document.querySelector(".mermaid-fullscreen")).toBeNull();
  });
});
