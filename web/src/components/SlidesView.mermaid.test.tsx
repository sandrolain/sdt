// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";

const mocks = vi.hoisted(() => ({ render: vi.fn() }));

vi.mock("pele", () => ({
  render: (...args: unknown[]) => mocks.render(...args),
}));

vi.mock("svg-pan-zoom", () => ({
  default: vi.fn(() => ({
    zoomIn: vi.fn(),
    zoomOut: vi.fn(),
    reset: vi.fn(),
    destroy: vi.fn(),
  })),
}));

import { SlidesView } from "./SlidesView";

const DECK = `# One

\`\`\`mermaid
flowchart LR
  A-->B
\`\`\`

---

# Two

\`\`\`mermaid
flowchart LR
  C-->D
\`\`\`
`;

beforeEach(() => {
  mocks.render.mockReset();
  mocks.render.mockReturnValue({
    svg: '<svg xmlns="http://www.w3.org/2000/svg" data-testid="diagram"></svg>',
    width: 100,
    height: 50,
    type: "flowchart",
    mermaidType: "flowchart",
    links: [],
  });
});

afterEach(() => cleanup());

describe("SlidesView mermaid", () => {
  it("renders the current slide's diagram and the next slide's", async () => {
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 2")).toBeTruthy());
    await waitFor(() =>
      expect(document.querySelector('.slides-frame__slide [data-testid="diagram"]')).toBeTruthy(),
    );
    expect(
      document
        .querySelector(".slides-frame__slide .md-mermaid")
        ?.classList.contains("md-mermaid--contain"),
    ).toBe(true);

    window.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight" }));
    await waitFor(() => expect(screen.getByText("2 / 2")).toBeTruthy());
    await waitFor(() =>
      expect(document.querySelector('.slides-frame__slide [data-testid="diagram"]')).toBeTruthy(),
    );
    expect(mocks.render).toHaveBeenCalledTimes(2);
  });

  it("falls back to the source when Pele cannot parse the diagram", async () => {
    mocks.render.mockImplementationOnce(() => {
      throw new Error("bad diagram");
    });
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 2")).toBeTruthy());
    await waitFor(() =>
      expect(document.querySelector(".slides-frame__slide .md-mermaid__source")).toBeTruthy(),
    );
  });
});
