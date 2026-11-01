import { sanitizeDiagramSvg } from "./diagramSanitize";
import { enhanceMermaid } from "./mermaidZoom";

type PeleModule = typeof import("pele");

let loader: Promise<PeleModule> | null = null;

/** Lazy-load Pele (the diagram renderer) only when a document has diagrams. */
function loadPele(): Promise<PeleModule> {
  if (!loader) loader = import("pele");
  return loader;
}

/** Test/reset helper. */
export function resetMermaidLoader(): void {
  loader = null;
}

/**
 * Serialize a rendered diagram for download with its theme resolved. Pele's
 * SVG paints through `var(--pele-…)` names declared in `tokens.css`; a file
 * outside the app has no such declarations, so every colour would fall back to
 * Pele's own defaults. The resolved values are copied onto the clone, which
 * keeps the downloaded file looking like the diagram on screen.
 */
export function serializeDiagramSvg(svg: SVGSVGElement): string {
  const clone = svg.cloneNode(true) as SVGSVGElement;
  const computed = getComputedStyle(svg);
  const names = new Set<string>();
  for (const match of clone.outerHTML.matchAll(/var\((--pele-[\w-]+)/g)) names.add(match[1]);
  for (const name of names) {
    const value = computed.getPropertyValue(name).trim();
    if (value !== "") clone.style.setProperty(name, value);
  }
  return new XMLSerializer().serializeToString(clone);
}

/** The document's own font, so Pele measures text with the font it draws. */
function documentFont(): string | undefined {
  const font = getComputedStyle(document.documentElement).fontFamily.trim();
  return font === "" ? undefined : font;
}

/**
 * Render every un-rendered `.md-mermaid` placeholder inside `root`. Pele is
 * synchronous and themes through CSS variables, so a theme flip restyles the
 * diagrams in place — nothing is re-rendered. A failing diagram falls back to
 * its source.
 */
export async function renderMermaid(root: HTMLElement | null): Promise<void> {
  if (!root) return;
  const nodes = Array.from(root.querySelectorAll<HTMLElement>(".md-mermaid"));
  if (nodes.length === 0) return;

  const pele = await loadPele();
  const fontFamily = documentFont();

  for (const node of nodes) {
    if (node.dataset.rendered === "1") continue;
    const source = decodeURIComponent(node.dataset.src ?? "");
    node.dataset.rendered = "1";
    try {
      const { svg } = pele.render(source, fontFamily ? { fontFamily } : undefined);
      node.innerHTML = sanitizeDiagramSvg(svg);
      if (!node.querySelector(".md-mermaid__download")) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "md-mermaid__download";
        button.setAttribute("aria-label", "Download diagram SVG");
        button.title = "Download SVG";
        const icon = document.createElement("span");
        icon.className = "ms-icon";
        icon.setAttribute("aria-hidden", "true");
        icon.textContent = "download";
        button.append(icon);
        node.append(button);
      }
      enhanceMermaid(node);
    } catch {
      node.classList.add("md-mermaid--error");
      const pre = document.createElement("pre");
      pre.className = "md-mermaid__source";
      pre.textContent = source;
      node.replaceChildren(pre);
    }
  }
}
