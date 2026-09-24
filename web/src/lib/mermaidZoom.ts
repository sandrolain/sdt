import svgPanZoom from "svg-pan-zoom";

/** Imperative pan/zoom handle for one rendered mermaid diagram. */
export interface MermaidZoomHandle {
  zoomIn(): void;
  zoomOut(): void;
  reset(): void;
  destroy(): void;
}

interface Entry {
  api: ReturnType<typeof svgPanZoom>;
  svg: SVGSVGElement;
  controls: HTMLElement;
}

/** Pan/zoom options shared by the inline viewport and the fullscreen overlay. */
const ZOOM_OPTIONS = {
  minZoom: 0.5,
  maxZoom: 20,
  fit: true,
  center: true,
  controlIconsEnabled: false,
  dblClickZoomEnabled: false,
  mouseWheelZoomEnabled: true,
} as const;

const instances = new WeakMap<HTMLElement, Entry>();

/** Build a Material Symbols control button wired to `onClick`. */
function controlButton(icon: string, label: string, onClick: () => void): HTMLButtonElement {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "md-mermaid__control";
  button.setAttribute("aria-label", label);
  button.title = label;
  const glyph = document.createElement("span");
  glyph.className = "ms-icon";
  glyph.setAttribute("aria-hidden", "true");
  glyph.textContent = icon;
  button.append(glyph);
  button.addEventListener("click", onClick);
  return button;
}

function handleOf(entry: Entry): MermaidZoomHandle {
  return {
    zoomIn: () => entry.api.zoomIn(),
    zoomOut: () => entry.api.zoomOut(),
    reset: () => entry.api.reset(),
    destroy: () => entry.api.destroy(),
  };
}

/** Zoom in / out / reset / expand control cluster for one diagram. */
function buildControls(svg: SVGSVGElement, api: ReturnType<typeof svgPanZoom>): HTMLElement {
  const controls = document.createElement("div");
  controls.className = "md-mermaid__controls";
  controls.append(
    controlButton("zoom_in", "Zoom in", () => api.zoomIn()),
    controlButton("zoom_out", "Zoom out", () => api.zoomOut()),
    controlButton("fit_screen", "Reset view", () => api.reset()),
    controlButton("fullscreen", "Expand diagram", () => openFullscreen(svg)),
  );
  return controls;
}

/**
 * Attach GitHub-style pan/zoom and on-diagram controls to a rendered
 * `.md-mermaid` node (idempotent per SVG). Returns null when the node holds no
 * SVG yet.
 */
export function enhanceMermaid(node: HTMLElement): MermaidZoomHandle | null {
  const svg = node.querySelector<SVGSVGElement>("svg");
  if (!svg) return null;
  const existing = instances.get(node);
  if (existing && existing.svg === svg) return handleOf(existing);
  if (existing) destroyMermaidZoom(node);

  const api = svgPanZoom(svg, { ...ZOOM_OPTIONS });
  const controls = buildControls(svg, api);
  node.classList.add("md-mermaid--zoomable");
  node.append(controls);
  instances.set(node, { api, svg, controls });
  return handleOf(instances.get(node) as Entry);
}

/** Tear down the pan/zoom instance and controls bound to a node (no-op if none). */
export function destroyMermaidZoom(node: HTMLElement): void {
  const entry = instances.get(node);
  if (!entry) return;
  try {
    entry.api.destroy();
  } catch {
    // already detached from the DOM: nothing to release
  }
  entry.controls.remove();
  node.classList.remove("md-mermaid--zoomable");
  instances.delete(node);
}

/**
 * Open a fullscreen overlay with a pannable/zoomable clone of `svg`. Dismisses
 * on Escape, backdrop click or the close control.
 */
export function openFullscreen(svg: SVGSVGElement): void {
  const overlay = document.createElement("div");
  overlay.className = "mermaid-fullscreen";
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-modal", "true");
  overlay.setAttribute("aria-label", "Diagram");

  const stage = document.createElement("div");
  stage.className = "mermaid-fullscreen__stage";
  const clone = svg.cloneNode(true) as SVGSVGElement;
  clone.removeAttribute("width");
  clone.removeAttribute("height");
  stage.append(clone);
  overlay.append(stage);

  const controls = document.createElement("div");
  controls.className = "mermaid-fullscreen__controls";
  overlay.append(controls);

  const close = controlButton("close", "Close", () => dismiss());
  close.classList.add("mermaid-fullscreen__close");
  overlay.append(close);

  document.body.append(overlay);

  const api = svgPanZoom(clone, { ...ZOOM_OPTIONS });
  controls.append(
    controlButton("zoom_in", "Zoom in", () => api.zoomIn()),
    controlButton("zoom_out", "Zoom out", () => api.zoomOut()),
    controlButton("fit_screen", "Reset view", () => api.reset()),
  );

  function dismiss(): void {
    document.removeEventListener("keydown", onKey);
    try {
      api.destroy();
    } catch {
      // clone already removed
    }
    overlay.remove();
  }

  const onKey = (event: KeyboardEvent) => {
    if (event.key === "Escape") dismiss();
  };
  document.addEventListener("keydown", onKey);
  overlay.addEventListener("click", (event) => {
    if (event.target === overlay) dismiss();
  });

  close.focus();
}
