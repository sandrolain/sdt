import { useEffect, useMemo, useRef, useState } from "react";
import { Icon } from "../lib/icon";
import { sanitizeSlides } from "../lib/slidesSanitize";
import { resolveSlideAssets } from "../lib/slidesAssets";
import { mermaidFencePlugin } from "../lib/slidesMermaid";
import { normalizeDeckSource } from "../lib/slidesSource";
import { SLIDES_THEME_NAME, slidesDefaultThemeCss, slidesThemeCss } from "../lib/slidesTheme";

interface SlidesViewProps {
  /** deck markdown, frontmatter already stripped by the caller */
  markdown: string;
  /**
   * The corpus frontmatter (the full `---…---` block). Marp reads deck-wide
   * directives (`theme`, `paginate`, `size`, `header`, `footer`, `style`, …)
   * from it, so it is prepended to the body before rendering; unknown corpus
   * keys are ignored by the directive parser.
   */
  frontmatter?: string;
  /** Corpus path of the deck, the base a relative image or background resolves against. */
  path?: string;
}

/**
 * Rendered deck: one fixed-aspect frame per slide, prev/next and keyboard
 * navigation, a counter, fullscreen and a speaker-notes panel.
 *
 * The engine (`@marp-team/marp-core`) is imported dynamically so it lands in a
 * lazy chunk, like MindmapView. Rendering is best-effort: if the engine fails
 * to load, the view states the degradation instead of showing a broken frame.
 */

interface DeckRender {
  slides: string[];
  css: string;
  notes: string[][];
}

/** Render a deck with marp-core, then sanitize and resolve each slide. */
async function renderDeck(markdown: string, path?: string): Promise<DeckRender> {
  const { Marp } = await import("@marp-team/marp-core");
  const marp = new Marp({ script: false, math: "katex", inlineSVG: false });
  marp.themeSet.add(slidesThemeCss());
  // The packed CSS of an unset or `theme: default` deck is the built-in look
  // unless the built-in name is overridden too; the corpus decks say
  // `theme: default`, so this is what makes them use the viewer's tokens.
  marp.themeSet.add(slidesDefaultThemeCss());
  // `pack(undefined)` resolves through `themeSet.default`, which Marp Core
  // pinned to the built-in theme at construction: repoint it at the override,
  // or a deck that names no theme still gets the GitHub look.
  const defaultTheme = marp.themeSet.get("default");
  if (defaultTheme) marp.themeSet.default = defaultTheme;
  // A mermaid fence becomes the viewer's placeholder; Pele renders it after
  // the slide mounts. The plugin wraps the fence rule Marp Core installed.
  marp.use(mermaidFencePlugin);
  const { html, css, comments } = marp.render(normalizeDeckSource(markdown), {
    htmlAsArray: true,
  });
  return {
    slides: sanitizeSlides(html).map((slide) => resolveSlideAssets(slide, path)),
    css,
    notes: comments,
  };
}

/** A note is worth showing when it is non-empty after trimming. */
function usableNotes(notes: string[][]): string[][] {
  return notes.map((page) => page.map((n) => n.trim()).filter((n) => n.length > 0));
}

export function SlidesView({ markdown, frontmatter, path }: SlidesViewProps) {
  // The engine's input is the deck frontmatter plus the body: the caller strips
  // the corpus frontmatter, and Marp reads deck-wide directives from it.
  const source = useMemo(() => `${frontmatter ?? ""}${markdown}`, [frontmatter, markdown]);
  // one state object keyed by the source: switching documents resets the
  // render, the error and the page together, without a setState-in-effect.
  const [state, setState] = useState<{
    source: string;
    deck: DeckRender | null;
    error: string | null;
  }>({ source, deck: null, error: null });
  const [current, setCurrent] = useState(0);
  const [notesOpen, setNotesOpen] = useState(false);
  const frameRef = useRef<HTMLDivElement | null>(null);
  const slideRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    let alive = true;
    renderDeck(source, path)
      .then((d) => {
        if (alive) setState({ source, deck: d, error: null });
      })
      .catch((e: unknown) => {
        if (alive)
          setState({ source, deck: null, error: e instanceof Error ? e.message : String(e) });
      });
    return () => {
      alive = false;
    };
  }, [source, path]);

  // derive the current render from the latest source: a stale state (from a
  // previous document) reads as "loading" rather than showing the old deck.
  const fresh = state.source === source;
  const deck = fresh ? state.deck : null;
  const error = fresh ? state.error : null;

  const notes = useMemo(() => (deck ? usableNotes(deck.notes) : []), [deck]);
  const total = deck?.slides.length ?? 0;
  // clamp during render so a document change (or a shorter deck) never leaves
  // the index past the end
  const page = total === 0 ? 0 : Math.max(0, Math.min(total - 1, fresh ? current : 0));

  const go = (next: number) => {
    if (total === 0) return;
    setCurrent(Math.max(0, Math.min(total - 1, next)));
  };

  // keyboard navigation: arrow keys, space, page up/down, home/end
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA")) return;
      switch (e.key) {
        case "ArrowRight":
        case "ArrowDown":
        case "PageDown":
        case " ":
          e.preventDefault();
          setCurrent((c) => Math.min(total - 1, c + 1));
          break;
        case "ArrowLeft":
        case "ArrowUp":
        case "PageUp":
          e.preventDefault();
          setCurrent((c) => Math.max(0, c - 1));
          break;
        case "Home":
          e.preventDefault();
          setCurrent(0);
          break;
        case "End":
          e.preventDefault();
          setCurrent(Math.max(0, total - 1));
          break;
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [total]);

  // Fit the 1280x720 slide into the frame. `scale(calc(100cqw / 1280))` is
  // invalid CSS (scale() takes a number, the calc yields a length), so the
  // ratio is measured from the frame and applied inline; a ResizeObserver
  // keeps it right through a window resize and fullscreen.
  useEffect(() => {
    const frame = frameRef.current;
    const slide = slideRef.current;
    if (!frame || !slide) return;
    const fit = () => {
      const scale = frame.clientWidth / 1280;
      if (scale > 0) slide.style.transform = `scale(${scale})`;
    };
    fit();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(fit);
    observer.observe(frame);
    return () => observer.disconnect();
  }, [deck]);

  // Render the current slide's Mermaid placeholders with Pele. The module is
  // imported lazily and only when the slide carries a diagram; a page change
  // gets a fresh DOM, so the effect re-runs. A theme flip restyles the SVG in
  // place and needs no second render.
  useEffect(() => {
    if (!deck) return;
    const frame = slideRef.current;
    if (!frame?.querySelector(".md-mermaid")) return;
    void import("../lib/mermaidRender").then(({ renderMermaid }) => {
      // The frame is the same element across pages; if it is still mounted,
      // render whatever placeholder it holds now (the function is idempotent).
      if (frame.isConnected) void renderMermaid(frame, { fit: "contain", maxHeight: 480 });
    });
  }, [deck, page]);

  const fullscreen = () => {
    const el = frameRef.current;
    if (!el) return;
    if (document.fullscreenElement) void document.exitFullscreen();
    else void el.requestFullscreen?.();
  };

  if (error) {
    return (
      <div className="slides-degraded" role="alert">
        <Icon name="error" />
        <p>This deck could not be rendered: {error}.</p>
        <p className="slides-degraded__hint">Read it in Code mode instead.</p>
      </div>
    );
  }

  if (!deck) {
    return <p className="content__empty">Loading slides…</p>;
  }

  if (total === 0) {
    return (
      <div className="slides-degraded" role="alert">
        <Icon name="warning" />
        <p>This deck has no slides.</p>
      </div>
    );
  }

  const paginated = /data-marpit-pagination="[^"]*"/.test(deck.slides[page]);

  return (
    <div className="slides-view" data-theme-name={SLIDES_THEME_NAME}>
      <style>{deck.css}</style>
      <div className="slides-toolbar" role="group" aria-label="Slide navigation">
        <button
          type="button"
          className="slides-toolbar__btn"
          aria-label="Previous slide"
          disabled={page === 0}
          onClick={() => go(page - 1)}
        >
          <Icon name="chevron_left" />
        </button>
        <span className="slides-toolbar__counter" aria-live="polite">
          {page + 1} / {total}
        </span>
        <button
          type="button"
          className="slides-toolbar__btn"
          aria-label="Next slide"
          disabled={page === total - 1}
          onClick={() => go(page + 1)}
        >
          <Icon name="chevron_right" />
        </button>
        <button
          type="button"
          className={`slides-toolbar__btn${notesOpen ? " is-active" : ""}`}
          aria-label={notesOpen ? "Hide speaker notes" : "Show speaker notes"}
          aria-pressed={notesOpen}
          disabled={notes.length === 0}
          onClick={() => setNotesOpen((v) => !v)}
        >
          <Icon name="sticky_note_2" />
        </button>
        <button
          type="button"
          className="slides-toolbar__btn"
          aria-label="Fullscreen"
          onClick={fullscreen}
        >
          <Icon name="fullscreen" />
        </button>
      </div>
      <div className="slides-frame" ref={frameRef}>
        <div
          className="slides-frame__slide marpit"
          ref={slideRef}
          data-slide={page + 1}
          data-paginated={paginated}
          role="group"
          aria-roledescription="slide"
          aria-label={`Slide ${page + 1} of ${total}`}
          dangerouslySetInnerHTML={{ __html: deck.slides[page] }}
        />
      </div>
      {notesOpen && (
        <aside className="slides-notes" aria-label="Speaker notes">
          {notes[page]?.length ? (
            <ul>
              {notes[page].map((n, i) => (
                <li key={i}>{n}</li>
              ))}
            </ul>
          ) : (
            <p className="content__empty">No speaker notes on this slide.</p>
          )}
        </aside>
      )}
    </div>
  );
}
