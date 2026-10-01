/**
 * Keyboard model for the documents tree (analysis N7): one tab stop, arrow and
 * Home/End movement, type-ahead, and the shortcut that pulls focus into the
 * tree. With ~800 links the tree was reachable only by tabbing through every
 * row, so a reader could not jump into it at all.
 */

import { useEffect, useRef, type RefObject } from "react";

/** Milliseconds a type-ahead prefix stays buffered. */
export const TYPEAHEAD_MS = 600;

/** The focusable stops of the tree, in DOM order: folder headers and entries. */
export const STOP_SELECTOR = ".tree-folder > summary, .tree-entry";

/**
 * The stops a keyboard can actually reach: a row inside a collapsed folder is
 * rendered but not focusable, so it is skipped.
 */
export function treeStops(container: HTMLElement | null): HTMLElement[] {
  if (!container) return [];
  return Array.from(container.querySelectorAll<HTMLElement>(STOP_SELECTOR)).filter(isReachable);
}

function isReachable(stop: HTMLElement): boolean {
  if (stop.getAttribute("aria-disabled") === "true" || (stop as HTMLInputElement).disabled) {
    return false;
  }
  // every folder above must be open. A summary's own folder does not count: it
  // is the row that opens it.
  for (let node = stop.parentElement; node; node = node.parentElement) {
    if (node.tagName !== "DETAILS" || (node as HTMLDetailsElement).open) continue;
    // the folder the row itself opens is not a barrier: a summary is the control
    // that opens it, so only the folders above it can hide it
    const isOwnFolder = node.firstElementChild === stop;
    if (!isOwnFolder) return false;
  }
  return true;
}

/**
 * The label a type-ahead matches on: the row's own text, not the icon ligature
 * that precedes it, so typing the first letters of a title finds it.
 */
export function stopText(stop: HTMLElement): string {
  const label = stop.querySelector(".tree-entry__title, .tree-folder__label");
  return ((label ?? stop).textContent ?? "").trim();
}

/**
 * The stop a key press should move to, or null when the key is not a movement
 * (activation keys stay native). `prefix` is the buffered type-ahead.
 */
export function nextStopIndex(
  stops: HTMLElement[],
  current: number,
  key: string,
  prefix: string,
): number | null {
  if (stops.length === 0) return null;
  if (prefix) {
    for (let i = 0; i < stops.length; i++) {
      const at = (current + 1 + i) % stops.length;
      if (stopText(stops[at]).toLowerCase().startsWith(prefix)) return at;
    }
    return null;
  }
  switch (key) {
    case "ArrowDown":
    case "ArrowRight":
      return Math.min(current + 1, stops.length - 1);
    case "ArrowUp":
    case "ArrowLeft":
      return Math.max(current - 1, 0);
    case "Home":
      return 0;
    case "End":
      return stops.length - 1;
    default:
      return null;
  }
}

/** Whether a key press starts (or extends) a type-ahead prefix. */
export function isTypeaheadKey(event: KeyboardEvent): boolean {
  if (event.ctrlKey || event.metaKey || event.altKey) return false;
  return event.key.length === 1 && event.key !== " " && event.key !== "\n";
}

/** The keyboard shortcut that focuses the tree, beside ⌘K for search. */
export const TREE_FOCUS_KEY = "\\";

/** Whether a keydown is the tree-focus shortcut (⌘\ / Ctrl+\). */
export function isTreeFocusKey(event: KeyboardEvent): boolean {
  return (event.metaKey === true || event.ctrlKey === true) && event.key === TREE_FOCUS_KEY;
}

/** Focus the first reachable stop of a tree container. */
export function focusFirstStop(stops: HTMLElement[]): void {
  stops[0]?.focus();
}

/**
 * Keep exactly one tab stop among the rows: the one the reader is on, else the
 * first. Without this the tree is ~800 tab presses.
 */
export function syncTabStops(stops: HTMLElement[], active: number): void {
  stops.forEach((stop, index) => {
    stop.tabIndex = index === active ? 0 : -1;
  });
}

/** Index of `element` among the stops, or -1 when it is not one. */
export function stopIndex(stops: HTMLElement[], element: Element | null): number {
  if (!element) return -1;
  return stops.indexOf(element as HTMLElement);
}

/**
 * Wire the keyboard model to a tree container: roving tab stop, arrow/Home/End
 * movement, type-ahead, and an answer to `requestTreeFocus`.
 */
export function useTreeKeyboard(
  container: RefObject<HTMLElement | null>,
  focusRequests: number,
  /** re-applies the model when the tree content changes (the entries it holds) */
  revision?: unknown,
): void {
  const activeRef = useRef(0);
  const bufferRef = useRef({ prefix: "", at: 0 });
  const handledRef = useRef(focusRequests);

  useEffect(() => {
    const root = container.current;
    if (!root) return;

    const apply = (index: number) => {
      const stops = treeStops(root);
      if (stops.length === 0) return;
      const at = Math.min(Math.max(index, 0), stops.length - 1);
      activeRef.current = at;
      syncTabStops(stops, at);
    };

    // a focus request pulls the keyboard into the tree; the first mount only
    // establishes the single tab stop, without stealing focus
    if (handledRef.current === focusRequests) {
      apply(0);
    } else {
      handledRef.current = focusRequests;
      apply(0);
      focusFirstStop(treeStops(root));
    }

    const onFocusIn = (event: Event) => {
      const stops = treeStops(root);
      const index = stopIndex(stops, event.target as Element);
      if (index >= 0) apply(index);
    };

    const onKeyDown = (event: KeyboardEvent) => {
      const stops = treeStops(root);
      if (stops.length === 0) return;
      const current = Math.max(stopIndex(stops, document.activeElement), 0);
      const buffer = bufferRef.current;
      if (isTypeaheadKey(event)) {
        buffer.prefix = buffer.prefix + event.key.toLowerCase();
        buffer.at = Date.now();
      } else if (buffer.prefix && Date.now() - buffer.at > TYPEAHEAD_MS) {
        buffer.prefix = "";
      }
      const next = nextStopIndex(stops, current, event.key, buffer.prefix);
      if (next === null) return;
      event.preventDefault();
      apply(next);
      stops[next].focus();
    };

    root.addEventListener("focusin", onFocusIn);
    root.addEventListener("keydown", onKeyDown);
    // rows arrive and folders open after mount (lazy entries, the reveal on
    // navigation), and a new link is natively tabbable: re-apply the single tab
    // stop whenever the tree's content changes
    const observer = new MutationObserver(() => apply(activeRef.current));
    observer.observe(root, { childList: true, subtree: true });
    return () => {
      root.removeEventListener("focusin", onFocusIn);
      root.removeEventListener("keydown", onKeyDown);
      observer.disconnect();
    };
    // `revision` re-runs the model once the entries arrive: on the first render
    // the container is still empty, and a ref alone would never re-run the effect
  }, [container, focusRequests, revision]);
}
