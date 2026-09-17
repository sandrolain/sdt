// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import {
  clearActiveSection,
  resetActiveSection,
  setActiveSection,
  useActiveSection,
} from "./activeSection";

function Probe() {
  const active = useActiveSection();
  return <span data-testid="active">{`${active.path}|${active.key ?? ""}`}</span>;
}

afterEach(() => {
  cleanup();
  resetActiveSection();
});

describe("activeSection store", () => {
  it("publishes the active heading for a path", () => {
    render(<Probe />);
    act(() => setActiveSection("context/a.md", "Summary"));
    expect(screen.getByTestId("active").textContent).toBe("context/a.md|Summary");
  });

  it("only clears when the owner path matches", () => {
    render(<Probe />);
    act(() => setActiveSection("context/a.md", "Summary"));
    act(() => clearActiveSection("context/other.md"));
    expect(screen.getByTestId("active").textContent).toBe("context/a.md|Summary");
    act(() => clearActiveSection("context/a.md"));
    expect(screen.getByTestId("active").textContent).toBe("|");
  });
});
