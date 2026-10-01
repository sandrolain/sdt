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

/** The rendered size of a viewBox scaled to `availableWidth`; never upscales
 *  past the diagram's natural size. Zero when the viewBox or width is unset. */
export function fittedSvgSize(
  viewBox: { width: number; height: number },
  availableWidth: number,
): { width: number; height: number } {
  if (viewBox.width <= 0 || viewBox.height <= 0 || availableWidth <= 0) {
    return { width: 0, height: 0 };
  }
  const scale = Math.min(1, availableWidth / viewBox.width);
  return { width: viewBox.width * scale, height: viewBox.height * scale };
}

/** Read an SVG's viewBox dimensions from the attribute (jsdom-safe). */
function viewBoxOf(svg: SVGSVGElement): { width: number; height: number } {
  const parts = (svg.getAttribute("viewBox") ?? "")
    .split(/[\s,]+/)
    .filter(Boolean)
    .map(Number);
  if (parts.length === 4 && parts.every((n) => Number.isFinite(n))) {
    return { width: parts[2], height: parts[3] };
  }
  const base = svg.viewBox?.baseVal;
  return { width: base?.width ?? 0, height: base?.height ?? 0 };
}

/**
 * Give the SVG a definite box matching its viewBox aspect at the wrapper's
 * content width before svg-pan-zoom measures it. Mermaid emits
 * `width="100%"` with no height, so a `height: auto` SVG resolves against the
 * flex line and svg-pan-zoom measures the wrong (short) viewport, which clips
 * tall diagrams. Sizing the box here makes the wrapper grow to the diagram.
 */
function sizeSvgToViewBox(node: HTMLElement, svg: SVGSVGElement): void {
  const styles = getComputedStyle(node);
  const padding = parseFloat(styles.paddingLeft || "0") + parseFloat(styles.paddingRight || "0");
  const available = node.clientWidth - padding;
  const size = fittedSvgSize(viewBoxOf(svg), available);
  if (size.width > 0 && size.height > 0) {
    // inline style beats the `height: auto` rule, which would otherwise
    // re-collapse the SVG once svg-pan-zoom removes the viewBox attribute
    svg.style.width = `${size.width}px`;
    svg.style.height = `${size.height}px`;
    svg.setAttribute("width", String(size.width));
    svg.setAttribute("height", String(size.height));
  }
}

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

  sizeSvgToViewBox(node, svg);
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
