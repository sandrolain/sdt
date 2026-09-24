import { destroyMermaidZoom, enhanceMermaid } from "./mermaidZoom";

type MermaidModule = typeof import("mermaid");

let loader: Promise<MermaidModule> | null = null;

/** Lazy-load Mermaid (large dependency) only when a document has diagrams. */
function loadMermaid(): Promise<MermaidModule> {
  if (!loader) loader = import("mermaid");
  return loader;
}

/** Test/reset helper. */
export function resetMermaidLoader(): void {
  loader = null;
  lastSignature = "";
}

let lastSignature = "";

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

/** Catppuccin-token theme variables so diagrams match the app theme. */
function themeVariables(): Record<string, string> {
  return {
    background: cssVar("--bg-base") || "#1e1e2e",
    primaryColor: cssVar("--ctp-surface0") || "#313244",
    primaryTextColor: cssVar("--fg") || "#cdd6f4",
    primaryBorderColor: cssVar("--ctp-mauve") || "#cba6f7",
    lineColor: cssVar("--ctp-overlay0") || "#6c7086",
    textColor: cssVar("--fg") || "#cdd6f4",
    nodeBorder: cssVar("--ctp-mauve") || "#cba6f7",
    clusterBkg: cssVar("--bg-mantle") || "#181825",
    clusterBorder: cssVar("--border") || "#45475a",
    fontFamily: "inherit",
  };
}

function signature(): string {
  return `${cssVar("--bg-base")}|${cssVar("--fg")}`;
}

/** Reset rendered nodes so a theme change re-renders every diagram. */
function resetRendered(root: HTMLElement): void {
  for (const node of root.querySelectorAll<HTMLElement>(".md-mermaid[data-rendered]")) {
    destroyMermaidZoom(node);
    node.removeAttribute("data-rendered");
    node.classList.remove("md-mermaid--error");
    node.innerHTML = "";
  }
}

let renderSeq = 0;

/**
 * Render every un-rendered `.md-mermaid` placeholder inside `root`. Re-initializes
 * and re-renders when the resolved theme changes; a failing diagram falls back to
 * its source.
 */
export async function renderMermaid(root: HTMLElement | null): Promise<void> {
  if (!root) return;
  const nodes = Array.from(root.querySelectorAll<HTMLElement>(".md-mermaid"));
  if (nodes.length === 0) return;

  const current = signature();
  const mermaid = await loadMermaid();
  if (current !== lastSignature) {
    lastSignature = current;
    resetRendered(root);
  }
  mermaid.default.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: "base",
    themeVariables: themeVariables(),
  });

  for (const node of nodes) {
    if (node.dataset.rendered === "1") continue;
    const source = decodeURIComponent(node.dataset.src ?? "");
    node.dataset.rendered = "1";
    try {
      const { svg } = await mermaid.default.render(`sdt-mermaid-${renderSeq++}`, source);
      node.innerHTML = svg;
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
