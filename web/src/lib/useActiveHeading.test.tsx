// @vitest-environment jsdom
import { useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { resetActiveSection, useActiveSection } from "./activeSection";
import { useActiveHeading } from "./useActiveHeading";

type IOCallback = (entries: Array<{ target: Element; isIntersecting: boolean }>) => void;

class IntersectionObserverStub {
  static instances: IntersectionObserverStub[] = [];
  elements: Element[] = [];
  constructor(callback: IOCallback) {
    this.callback = callback;
    IntersectionObserverStub.instances.push(this);
  }
  private callback: IOCallback;
  observe(element: Element) {
    this.elements.push(element);
  }
  unobserve() {}
  disconnect() {}
  trigger() {
    this.callback(this.elements.map((target) => ({ target, isIntersecting: true })));
  }
}

function Probe() {
  const active = useActiveSection();
  return <span data-testid="active">{`${active.path}|${active.key ?? ""}`}</span>;
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

beforeEach(() => {
  IntersectionObserverStub.instances = [];
  vi.stubGlobal("IntersectionObserver", IntersectionObserverStub);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  resetActiveSection();
});

describe("useActiveHeading", () => {
  it("publishes the first heading in view", () => {
    render(
      <>
        <Doc />
        <Probe />
      </>,
    );
    const observer = IntersectionObserverStub.instances[0];
    act(() => observer.trigger());
    expect(screen.getByTestId("active").textContent).toBe("context/a.md|First");
  });
});
