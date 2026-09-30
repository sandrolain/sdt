import { useCallback, useEffect, useMemo, useState } from "react";
import { setActiveSection, useActiveSection } from "../lib/activeSection";
import { Icon } from "../lib/icon";
import { TooltipButton } from "./ui/Tooltip";

interface DocProgressProps {
  /** the rendered-document scroller (`.doc-rendered`) */
  container: HTMLElement | null;
  /** corpus path of the document, for the shared active-section store */
  path: string;
}

interface OutlineItem {
  id: string;
  text: string;
  depth: number;
}

/**
 * Reading aids that must stay reachable whatever the panel width (analysis R3):
 * a progress bar, a back-to-top action and a compact in-document outline. The
 * metadata panel holds the full Sections list, but it is hidden below 900 px,
 * where the outline used to disappear with it.
 */
export function DocProgress({ container, path }: DocProgressProps) {
  const [scrolled, setScrolled] = useState(0);
  const [outlineOpen, setOutlineOpen] = useState(false);
  const { key: activeHeading } = useActiveSection();

  useEffect(() => {
    if (!container) return;
    let frame = 0;
    const measure = () => {
      if (frame) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        const max = container.scrollHeight - container.clientHeight;
        setScrolled(max > 0 ? Math.min(1, container.scrollTop / max) : 0);
      });
    };
    measure();
    container.addEventListener("scroll", measure, { passive: true });
    window.addEventListener("resize", measure);
    return () => {
      container.removeEventListener("scroll", measure);
      window.removeEventListener("resize", measure);
      if (frame) cancelAnimationFrame(frame);
    };
  }, [container]);

  const outline = useMemo<OutlineItem[]>(() => {
    if (!container) return [];
    return Array.from(
      container.querySelectorAll<HTMLElement>("h2[data-heading], h3[data-heading]"),
    ).map((el) => ({
      id: el.dataset.heading ?? "",
      text: (el.dataset.heading ?? el.textContent ?? "").trim(),
      depth: Number(el.tagName.slice(1)),
    }));
  }, [container]);

  const jump = useCallback(
    (id: string) => {
      if (!container) return;
      const target = Array.from(container.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6")).find(
        (h) => (h.dataset.heading ?? h.textContent ?? "").trim() === id,
      );
      target?.scrollIntoView({ behavior: "smooth", block: "start" });
      if (id) setActiveSection(path, id);
    },
    [container, path],
  );

  const percent = Math.round(scrolled * 100);
  return (
    <div className="doc-progress">
      <div
        className="doc-progress__bar"
        role="progressbar"
        aria-label="Reading progress"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
      >
        <span className="doc-progress__fill" style={{ width: `${percent}%` }} />
      </div>
      <div className="doc-progress__actions">
        {outline.length > 1 && (
          <TooltipButton
            className="doc-progress__button"
            label={outlineOpen ? "Hide outline" : "Show outline"}
            tooltip={outlineOpen ? "Hide outline" : "Show outline"}
            aria-expanded={outlineOpen}
            onPress={() => setOutlineOpen((open) => !open)}
          >
            <Icon name="list" />
          </TooltipButton>
        )}
        <TooltipButton
          className="doc-progress__button"
          label="Back to top"
          tooltip="Back to top"
          isDisabled={percent === 0}
          onPress={() => {
            container?.scrollTo({ top: 0, behavior: "smooth" });
          }}
        >
          <Icon name="keyboard_arrow_up" />
        </TooltipButton>
      </div>
      {outlineOpen && (
        <nav className="doc-outline" aria-label="Document outline">
          {outline.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`doc-outline__item doc-outline__item--h${item.depth}${
                item.id === activeHeading ? " is-active" : ""
              }`}
              aria-current={item.id === activeHeading ? "location" : undefined}
              onClick={() => jump(item.id)}
            >
              {item.text}
            </button>
          ))}
        </nav>
      )}
    </div>
  );
}
