/**
 * Label-layer CSS for the ported graph engine.
 *
 * Ported faithfully from `context/refs/react/graph-react/src/KnowledgeGraph.jsx`
 * (LABEL_CSS/EDGE_LABEL_CSS 224-233). The DOM label layer is the medium of the
 * engine, so these raw values are graph identity (phase 9), not theme surfaces;
 * the Catppuccin theming phase may re-pick them but must keep them raw here.
 */

export const LABEL_CSS =
  "position:absolute;left:0;top:0;white-space:nowrap;pointer-events:none;" +
  "font:400 11.5px/16px system-ui,-apple-system,'Segoe UI',sans-serif;" +
  "color:rgba(226,232,240,.92);text-shadow:0 0 5px #05080f,0 0 2px #05080f,0 1px 2px #05080f;" +
  "will-change:transform;display:none;";

export const EDGE_LABEL_CSS =
  "position:absolute;left:0;top:0;white-space:nowrap;pointer-events:none;display:none;" +
  "font:500 10.5px/14px system-ui,-apple-system,'Segoe UI',sans-serif;" +
  "padding:1px 6px;border-radius:8px;background:rgba(8,12,22,.82);" +
  "border:1px solid rgba(148,163,184,.25);will-change:transform;";
