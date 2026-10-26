/**
 * The agent panel's styling harness script.
 *
 * It loads the app's real stylesheet and the same mermaid mount the thread uses, on the
 * markup in agent-panel.html. Nothing here reaches the bridge, the runtime or an agent:
 * the page exists so a styling change has a render cell without a paid turn.
 */
import "@fontsource-variable/ibm-plex-sans";
import "@fontsource-variable/material-symbols-outlined";
import "../src/styles/tokens.css";
import "../src/styles/index.css";
import { renderMermaid } from "../src/lib/mermaidRender";

/** The fence the thread would receive; encoded the way `renderMarkdown` encodes `data-src`. */
const diagram = [
  "flowchart LR",
  "  reader[Reader] --> decide{Decision?}",
  "  decide -->|yes| turn[Four-part turn]",
  "  decide -->|no| line[Terse line]",
].join("\n");

const mount = document.querySelector<HTMLElement>("[data-harness-mermaid]");
if (mount) {
  const placeholder = document.createElement("div");
  placeholder.className = "md-mermaid";
  placeholder.dataset.src = encodeURIComponent(diagram);
  mount.append(placeholder);
  void renderMermaid(mount);
}
