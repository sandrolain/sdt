// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";

// The engine throws, so the view must degrade visibly rather than show a
// broken frame. Mocked at the module boundary so the lazy `import()` inside
// `renderDeck` resolves to this stub.
vi.mock("@marp-team/marp-core", () => ({
  Marp: class {
    themeSet = { add: () => {} };
    render() {
      throw new Error("engine exploded");
    }
  },
}));

import { SlidesView } from "./SlidesView";

afterEach(() => cleanup());

describe("SlidesView degradation", () => {
  it("states the failure in the frame when the engine throws", async () => {
    render(<SlidesView markdown={"# One\n\n---\n\n# Two\n"} />);
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("could not be rendered");
    expect(screen.getByRole("alert").textContent).toContain("engine exploded");
  });
});
