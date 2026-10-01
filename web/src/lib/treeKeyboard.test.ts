// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  isTreeFocusKey,
  isTypeaheadKey,
  nextStopIndex,
  stopIndex,
  stopText,
  syncTabStops,
  treeStops,
  useTreeKeyboard,
  TREE_FOCUS_KEY,
} from "./treeKeyboard";
import { requestTreeFocus, resetTreeFocusRequests, useTreeFocusRequests } from "./treeFocus";

/** A tree with an open folder, a closed folder and three entry rows. */
function mountTree(): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = `
    <details class="tree-folder" open>
      <summary>Analysis</summary>
      <a class="tree-entry" href="/docs/a">Alpha</a>
      <a class="tree-entry" href="/docs/b">Beta</a>
    </details>
    <details class="tree-folder">
      <summary>Plan</summary>
      <a class="tree-entry" href="/docs/hidden">Hidden</a>
    </details>
  `;
  document.body.appendChild(root);
  return root;
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("treeStops", () => {
  it("lists folder headers and reachable entries in DOM order", () => {
    const stops = treeStops(mountTree());
    expect(stops.map(stopText)).toEqual(["Analysis", "Alpha", "Beta", "Plan"]);
  });

  it("skips entries inside a collapsed folder but keeps its header", () => {
    const stops = treeStops(mountTree());
    expect(stops.some((s) => s.textContent === "Hidden")).toBe(false);
    expect(stops.some((s) => s.textContent === "Plan")).toBe(true);
  });

  it("includes an entry once its folder is open", () => {
    const root = mountTree();
    const folders = root.querySelectorAll("details");
    (folders[1] as HTMLDetailsElement).open = true;
    expect(treeStops(root).map(stopText)).toContain("Hidden");
  });

  it("skips a sub-folder inside a collapsed folder", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <details class="tree-folder" open>
        <summary>Notes</summary>
        <details class="tree-folder">
          <summary>viewer</summary>
          <a class="tree-entry" href="/docs/a">Nested</a>
        </details>
      </details>
    `;
    document.body.appendChild(root);
    // the closed sub-folder's own header stays reachable; its row does not
    expect(treeStops(root).map(stopText)).toEqual(["Notes", "viewer"]);
  });

  it("matches the row label, not the icon ligature", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <details class="tree-folder" open>
        <summary><span class="tree-folder__chevron">expand_more</span>
          <span class="tree-folder__label">Architecture</span>
        </summary>
        <a class="tree-entry" href="/docs/a">
          <span class="tree-entry__glyph">description</span>
          <span class="tree-entry__title">Stack</span>
        </a>
      </details>
    `;
    document.body.appendChild(root);
    const stops = treeStops(root);
    expect(stops.map(stopText)).toEqual(["Architecture", "Stack"]);
    // typing "arc" reaches the folder, not the icon text
    expect(nextStopIndex(stops, 1, "a", "arc")).toBe(0);
  });

  it("is empty without a container", () => {
    expect(treeStops(null)).toEqual([]);
  });
});

describe("nextStopIndex", () => {
  const stops = ["a", "b", "c"].map((t) => {
    const el = document.createElement("a");
    el.textContent = t;
    return el;
  }) as unknown as HTMLElement[];

  it("moves down and up, clamping at the ends", () => {
    expect(nextStopIndex(stops, 0, "ArrowDown", "")).toBe(1);
    expect(nextStopIndex(stops, 2, "ArrowDown", "")).toBe(2);
    expect(nextStopIndex(stops, 1, "ArrowUp", "")).toBe(0);
    expect(nextStopIndex(stops, 0, "ArrowUp", "")).toBe(0);
  });

  it("jumps to the first and last stop", () => {
    expect(nextStopIndex(stops, 1, "Home", "")).toBe(0);
    expect(nextStopIndex(stops, 0, "End", "")).toBe(2);
  });

  it("wraps around with a type-ahead prefix, searching from the current row", () => {
    expect(nextStopIndex(stops, 2, "a", "a")).toBe(0);
    expect(nextStopIndex(stops, 0, "b", "b")).toBe(1);
    expect(nextStopIndex(stops, 0, "z", "z")).toBeNull();
  });

  it("ignores activation and other keys", () => {
    expect(nextStopIndex(stops, 0, "Enter", "")).toBeNull();
    expect(nextStopIndex(stops, 0, "Tab", "")).toBeNull();
    expect(nextStopIndex([], 0, "ArrowDown", "")).toBeNull();
  });
});

describe("isTypeaheadKey", () => {
  function key(init: Partial<KeyboardEvent>): KeyboardEvent {
    return { key: "a", ctrlKey: false, metaKey: false, altKey: false, ...init } as KeyboardEvent;
  }

  it("accepts a printable character and refuses the rest", () => {
    expect(isTypeaheadKey(key({ key: "a" }))).toBe(true);
    expect(isTypeaheadKey(key({ key: "7" }))).toBe(true);
    expect(isTypeaheadKey(key({ key: " " }))).toBe(false);
    expect(isTypeaheadKey(key({ key: "Enter" }))).toBe(false);
    expect(isTypeaheadKey(key({ key: "a", metaKey: true }))).toBe(false);
    expect(isTypeaheadKey(key({ key: "a", ctrlKey: true }))).toBe(false);
  });
});

describe("syncTabStops and stopIndex", () => {
  it("keeps a single tab stop on the active row", () => {
    const root = mountTree();
    const stops = treeStops(root);
    syncTabStops(stops, 2);
    expect(stops.map((s) => s.tabIndex)).toEqual([-1, -1, 0, -1]);
    expect(stopIndex(stops, stops[2])).toBe(2);
    expect(stopIndex(stops, document.createElement("a"))).toBe(-1);
    expect(stopIndex(stops, null)).toBe(-1);
  });
});

describe("tree focus shortcut", () => {
  it("matches ⌘\\ / Ctrl+\\ only", () => {
    expect(TREE_FOCUS_KEY).toBe("\\");
    expect(isTreeFocusKey({ key: "\\", metaKey: true } as KeyboardEvent)).toBe(true);
    expect(isTreeFocusKey({ key: "\\", ctrlKey: true } as KeyboardEvent)).toBe(true);
    expect(isTreeFocusKey({ key: "\\" } as KeyboardEvent)).toBe(false);
    expect(isTreeFocusKey({ key: "k", metaKey: true } as KeyboardEvent)).toBe(false);
  });

  it("raises a request a mounted tree can answer", () => {
    resetTreeFocusRequests();
    const { result } = renderHook(() => useTreeFocusRequests());
    expect(result.current).toBe(0);
    act(() => requestTreeFocus());
    expect(result.current).toBe(1);
    act(() => requestTreeFocus());
    expect(result.current).toBe(2);
  });
});

describe("useTreeKeyboard", () => {
  function setup() {
    const root = mountTree();
    const ref = { current: root };
    const { rerender } = renderHook(
      ({ requests }: { requests: number }) => useTreeKeyboard(ref, requests),
      {
        initialProps: { requests: 0 },
      },
    );
    return { root, ref, rerender };
  }

  it("gives the tree exactly one tab stop", () => {
    const { root } = setup();
    const stops = treeStops(root);
    expect(stops.filter((s) => s.tabIndex === 0)).toHaveLength(1);
    expect(stops[0].tabIndex).toBe(0);
  });

  it("moves focus with the arrow keys and keeps the tab stop with it", () => {
    const { root } = setup();
    const stops = treeStops(root);
    stops[0].focus();
    root.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
    expect(document.activeElement).toBe(stops[1]);
    expect(stops.filter((s) => s.tabIndex === 0)).toEqual([stops[1]]);

    root.dispatchEvent(new KeyboardEvent("keydown", { key: "End", bubbles: true }));
    expect(document.activeElement).toBe(stops[stops.length - 1]);
  });

  it("jumps by type-ahead", () => {
    const { root } = setup();
    const stops = treeStops(root);
    stops[0].focus();
    root.dispatchEvent(new KeyboardEvent("keydown", { key: "b", bubbles: true }));
    expect(document.activeElement).toBe(stops[2]);
  });

  it("focuses the tree when a focus request arrives", () => {
    const root = mountTree();
    const ref = { current: root };
    const { rerender } = renderHook(
      ({ requests }: { requests: number }) => useTreeKeyboard(ref, requests),
      { initialProps: { requests: 0 } },
    );
    const stops = treeStops(root);
    // the first mount must not steal focus; the request must
    expect(document.activeElement).not.toBe(stops[0]);
    rerender({ requests: 1 });
    expect(document.activeElement).toBe(stops[0]);
  });

  it("ignores activation keys", () => {
    const { root } = setup();
    const stops = treeStops(root);
    stops[0].focus();
    const event = new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true });
    root.dispatchEvent(event);
    expect(document.activeElement).toBe(stops[0]);
    expect(event.defaultPrevented).toBe(false);
  });

  it("keeps one tab stop when rows are added after mount", async () => {
    const { root } = setup();
    const added = document.createElement("a");
    added.className = "tree-entry";
    added.textContent = "Late arrival";
    root.querySelector("details")?.appendChild(added);
    // the observer restores the model on the next microtask
    await new Promise((resolve) => setTimeout(resolve, 0));
    const stops = treeStops(root);
    expect(stops.filter((s) => s.tabIndex === 0)).toHaveLength(1);
  });

  it("detaches its listeners on unmount", () => {
    const root = mountTree();
    const remove = vi.spyOn(root, "removeEventListener");
    const { unmount } = renderHook(() => useTreeKeyboard({ current: root }, 0));
    unmount();
    expect(remove).toHaveBeenCalledWith("keydown", expect.any(Function));
  });
});

beforeEach(() => {
  resetTreeFocusRequests();
});
