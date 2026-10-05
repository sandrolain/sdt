/**
 * Markdown-lite node body for the shared JSON Canvas view.
 *
 * Adapted from `context/refs/react/jc-vite/src/JsonCanvas.jsx` (MIT): headings
 * `#`-`###`, `-`/`*` lists, bold, italic, inline code and a styled wiki-link
 * span. Rendered as React elements — never `dangerouslySetInnerHTML`.
 */
import type { ReactNode } from "react";

function inline(s: string): ReactNode[] {
  return s.split(/(\*\*[^*]+\*\*|`[^`]+`|\[\[[^\]]+\]\]|\*[^*\s][^*]*\*)/g).map((t, i) => {
    if (t.startsWith("**")) return <b key={i}>{t.slice(2, -2)}</b>;
    if (t.startsWith("`")) return <code key={i}>{t.slice(1, -1)}</code>;
    if (t.startsWith("[["))
      return (
        <span key={i} className="jc-wl">
          {t.slice(2, -2)}
        </span>
      );
    if (t.startsWith("*") && t.length > 2) return <i key={i}>{t.slice(1, -1)}</i>;
    return t;
  });
}

/** Markdown-lite renderer for a `text` node body. */
export function Md({ text }: { text: string }) {
  return (
    <>
      {text.split("\n").map((l, i) => {
        const h = /^(#{1,3})\s+(.*)/.exec(l);
        if (h)
          return (
            <div key={i} className={`jc-h${h[1].length}`}>
              {inline(h[2])}
            </div>
          );
        if (/^\s*[-*]\s/.test(l))
          return (
            <div key={i} className="jc-li">
              {inline(l.replace(/^\s*[-*]\s/, ""))}
            </div>
          );
        return <div key={i}>{l.trim() ? inline(l) : <br />}</div>;
      })}
    </>
  );
}
