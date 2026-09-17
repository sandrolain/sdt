// @vitest-environment jsdom
import { useRef } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { resetActiveSection, useActiveSection } from "./activeSection";
import { useActiveHeading } from "./useActiveHeading";

function rect(top: number): DOMRect {
  return { top, bottom: top, left: 0, right: 0, width: 0, height: 0, x: 0, y: top } as DOMRect;
}

function Probe() {
  const active = useActiveSection();
  return <span data-testid="active">{active.key ?? ""}</span>;
}

function Doc() {
  const ref = useRef<HTMLDivElement | null>(null);
  useActiveHeading(ref, "context/a.md", true);
  return (
    <div className="doc-rendered" ref={ref}>
      <h1>First</h1>
      <h2>Second</h2>
    </div>
  );
}

/** Stub container + heading geometry, then simulate a scroll. */
function layout(positions: number[]) {
  const root = document.querySelector(".doc-rendered") as HTMLElement;
  Object.defineProperty(root, "clientHeight", { value: 300, configurable: true });
  root.getBoundingClientRect = () => rect(0);
  const headings = Array.from(root.querySelectorAll<HTMLElement>("h1,h2"));
  headings.forEach((h, i) => {
    h.getBoundingClientRect = () => rect(positions[i] ?? 0);
  });
}

afterEach(() => {
  cleanup();
  resetActiveSection();
});

describe("useActiveHeading", () => {
  it("keeps a section selected when scrolling mid-section", () => {
    render(
      <>
        <Doc />
        <Probe />
      </>,
    );
    // First above the threshold, Second well below it
    layout([-120, 500]);
    act(() => window.dispatchEvent(new Event("scroll")));
    expect(screen.getByTestId("active").textContent).toBe("First");
  });

  it("advances to the next section once its heading passes the threshold", () => {
    render(
      <>
        <Doc />
        <Probe />
      </>,
    );
    layout([-500, 40]);
    act(() => window.dispatchEvent(new Event("scroll")));
    expect(screen.getByTestId("active").textContent).toBe("Second");
  });
});
