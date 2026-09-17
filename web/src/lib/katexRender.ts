type KatexModule = typeof import("katex");

let loader: Promise<KatexModule> | null = null;

/** Lazy-load KaTeX and its stylesheet (dynamic chunks, only when math exists). */
function loadKatex(): Promise<KatexModule> {
  if (!loader) {
    loader = Promise.all([import("katex"), import("katex/dist/katex.min.css")]).then(
      ([katex]) => katex,
    );
  }
  return loader;
}

/** Test/reset helper. */
export function resetKatexLoader(): void {
  loader = null;
}

/** Replace a failed formula with an error marker plus its raw TeX. */
function showMathError(node: HTMLElement, tex: string): void {
  const marker = document.createElement("span");
  marker.className = "md-math__error";
  marker.textContent = "math error";
  const source = document.createElement("code");
  source.className = "md-math__source";
  source.textContent = tex;
  node.replaceChildren(marker, source);
}

/**
 * Render every un-rendered `.md-math` node inside `root` with KaTeX. Errors are
 * shown in place (throwOnError:false) and the raw TeX stays available as a
 * tooltip; the source is restored if rendering throws outright.
 */
export async function renderMath(root: HTMLElement | null): Promise<void> {
  if (!root) return;
  const nodes = Array.from(root.querySelectorAll<HTMLElement>(".md-math:not([data-rendered])"));
  if (nodes.length === 0) return;
  const katex = await loadKatex();
  for (const node of nodes) {
    const tex = node.dataset.tex ?? "";
    node.dataset.rendered = "1";
    node.title = tex;
    try {
      katex.default.render(tex, node, {
        displayMode: node.dataset.display === "block",
        throwOnError: false,
        trust: false,
        strict: "ignore",
      });
      if (node.querySelector(".katex-error")) {
        node.classList.add("md-math--error");
        showMathError(node, tex);
      }
    } catch {
      node.classList.add("md-math--error");
      showMathError(node, tex);
    }
  }
}
