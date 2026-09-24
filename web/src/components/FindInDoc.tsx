import { useEffect, useRef, useState } from "react";
import {
  applyFindHighlights,
  clearFindHighlights,
  collectFindRanges,
  scrollFindCurrent,
} from "../lib/findInDoc";
import { closeFind, setFindQuery, useFindInDoc } from "../lib/findInDocStore";
import type { DocumentMode } from "../lib/documentModes";
import { Icon } from "../lib/icon";
import { TooltipButton } from "./ui/Tooltip";

interface FindInDocProps {
  /** the document view article; the findable container is resolved by mode */
  root: HTMLElement | null;
  mode: DocumentMode;
}

/** Find bar over the open document body: highlights and steps through matches. */
export function FindInDoc({ root, mode }: FindInDocProps) {
  const { open, query } = useFindInDoc();
  const [current, setCurrent] = useState(0);
  const [count, setCount] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const target = (() => {
    if (!root || !open) return null;
    if (mode === "render") return root.querySelector<HTMLElement>(".doc-rendered");
    if (mode === "code") return root.querySelector<HTMLElement>(".doc-code");
    return null;
  })();

  // Sync the DOM with the query and active match. Collection always runs on a
  // freshly cleared container so the wrapped nodes match the live DOM; a
  // single effect keyed on [query, current] avoids stale-node re-wrapping.
  useEffect(() => {
    if (!target) {
      // Mirror the external DOM match count into state for the counter.
      // oxlint-disable-next-line react-hooks/set-state-in-effect
      setCount(0);
      return;
    }
    clearFindHighlights(target);
    const found = collectFindRanges(target, query);
    // Mirror the external DOM match count into state for the counter.
    // oxlint-disable-next-line react-hooks/set-state-in-effect
    setCount(found.length);
    if (found.length > 0) {
      const safe = clamp(current, found.length);
      applyFindHighlights(target, found, safe);
      scrollFindCurrent(target);
    }
    return () => clearFindHighlights(target);
  }, [target, open, query, current]);

  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  const step = (dir: 1 | -1) => {
    if (count === 0) return;
    setCurrent((c) => (c + dir + count) % count);
  };

  return (
    <div className="find-doc" role="search" aria-label="Find in document">
      <Icon name="search" className="find-doc__icon" />
      <input
        ref={inputRef}
        className="find-doc__input"
        type="text"
        value={query}
        placeholder="Find in document…"
        aria-label="Find in document"
        onChange={(e) => setFindQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            closeFind();
          } else if (e.key === "Enter") {
            e.preventDefault();
            step(e.shiftKey ? -1 : 1);
          }
        }}
      />
      <span className="find-doc__count" data-testid="find-count">
        {count === 0 ? "0/0" : `${current + 1}/${count}`}
      </span>
      <TooltipButton
        className="find-doc__button"
        label="Previous match"
        tooltip="Previous match (Shift+Enter)"
        onPress={() => step(-1)}
      >
        <Icon name="keyboard_arrow_up" />
      </TooltipButton>
      <TooltipButton
        className="find-doc__button"
        label="Next match"
        tooltip="Next match (Enter)"
        onPress={() => step(1)}
      >
        <Icon name="keyboard_arrow_down" />
      </TooltipButton>
      <TooltipButton
        className="find-doc__button"
        label="Close find"
        tooltip="Close (Esc)"
        onPress={closeFind}
      >
        <Icon name="close" />
      </TooltipButton>
    </div>
  );
}

function clamp(value: number, max: number): number {
  if (max <= 0) return 0;
  if (value < 0) return 0;
  if (value >= max) return max - 1;
  return value;
}
