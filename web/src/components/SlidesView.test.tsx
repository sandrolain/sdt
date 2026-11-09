// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SlidesView } from "./SlidesView";

const DECK = `---\ntheme: default\n---\n\n# One\n\nfirst\n\n---\n\n## Two\n\n<!-- a note -->\n\nsecond\n\n---\n\n## Three\n\nthird\n`;

afterEach(() => cleanup());

describe("SlidesView", () => {
  // The first render compiles the Marp deck; under the full parallel suite it can
  // exceed the 5s default, so this cell gets its own budget rather than a flaky gate.
  it("renders the deck's slides and a counter", { timeout: 20000 }, async () => {
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
    expect(screen.getByText("One")).toBeTruthy();
  });

  it("navigates with the next/prev buttons", async () => {
    const user = userEvent.setup();
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
    await user.click(screen.getByLabelText("Next slide"));
    expect(screen.getByText("2 / 3")).toBeTruthy();
    await user.click(screen.getByLabelText("Previous slide"));
    expect(screen.getByText("1 / 3")).toBeTruthy();
  });

  it("navigates with the keyboard", async () => {
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight" }));
    await waitFor(() => expect(screen.getByText("2 / 3")).toBeTruthy());
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "End" }));
    await waitFor(() => expect(screen.getByText("3 / 3")).toBeTruthy());
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Home" }));
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
  });

  it("shows speaker notes for the current slide", async () => {
    const user = userEvent.setup();
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
    // the note lives on slide 2
    await user.click(screen.getByLabelText("Next slide"));
    await user.click(screen.getByLabelText("Show speaker notes"));
    expect(screen.getByText("a note")).toBeTruthy();
  });

  it("defaults to the first slide and disables Previous there", async () => {
    render(<SlidesView markdown={DECK} />);
    await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
    expect((screen.getByLabelText("Previous slide") as HTMLButtonElement).disabled).toBe(true);
  });

  it("still shows a frame for a deck that renders to one empty slide", async () => {
    // marp-core always yields at least one section; a blank deck is one empty
    // slide, not a crash (the "no slides" defect is a lint signal, not a view
    // state).
    render(<SlidesView markdown={""} />);
    await waitFor(() => expect(screen.getByText("1 / 1")).toBeTruthy());
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("scopes the section under marpit and injects the token theme", async () => {
    // The packed CSS is all `div.marpit > section`; without the container class
    // no rule matches and the deck renders unstyled.
    render(<SlidesView markdown={"---\ntheme: default\n---\n\n# One\n"} />);
    await waitFor(() => expect(screen.getByText("1 / 1")).toBeTruthy());
    const frame = document.querySelector(".slides-frame__slide");
    expect(frame?.classList.contains("marpit")).toBe(true);
    const css = document.querySelector(".slides-view style")?.textContent ?? "";
    expect(css).toContain("div.marpit > section");
    // `theme: default` must select the viewer's tokens, not Marp's built-in look.
    expect(css).toContain("var(--bg-base)");
  });

  it("resolves a relative image and background through /api/file", async () => {
    // Marp emits both relative to the deck; the SPA would resolve them against
    // the route, so the view rewrites them after sanitization.
    render(
      <SlidesView
        markdown={
          "---\ntheme: default\n---\n\n# One\n\n![w:200](../assets/pic.jpg)\n\n![bg](../assets/bg.jpg)\n"
        }
        path="context/notes/deck.slide.md"
      />,
    );
    await waitFor(() => expect(screen.getByText("1 / 1")).toBeTruthy());
    const frame = document.querySelector(".slides-frame__slide");
    expect(frame?.querySelector("img")?.getAttribute("src")).toBe(
      "/api/file?path=context%2Fassets%2Fpic.jpg",
    );
    expect(frame?.querySelector("section")?.getAttribute("style")).toContain(
      "/api/file?path=context%2Fassets%2Fbg.jpg",
    );
  });

  it("applies deck-wide directives from the frontmatter", async () => {
    // The caller strips the corpus frontmatter; the deck-wide directives live
    // there, so the view must prepend it before rendering.
    render(
      <SlidesView
        markdown={"# One\n\nbody\n"}
        frontmatter={"---\ntheme: default\npaginate: true\nheader: Head\nfooter: Foot\n---\n"}
      />,
    );
    await waitFor(() => expect(screen.getByText("1 / 1")).toBeTruthy());
    const frame = document.querySelector(".slides-frame__slide");
    expect(frame?.querySelector("header")?.textContent).toBe("Head");
    expect(frame?.querySelector("footer")?.textContent).toBe("Foot");
    expect(frame?.getAttribute("data-paginated")).toBe("true");
  });

  it("uses the token theme for a deck that names no theme", async () => {
    // `pack(undefined)` goes through themeSet.default; Marp Core pins it to
    // the built-in theme, so the view repoints it at the token override.
    render(<SlidesView markdown={"# One\n"} />);
    await waitFor(() => expect(screen.getByText("1 / 1")).toBeTruthy());
    const css = document.querySelector(".slides-view style")?.textContent ?? "";
    expect(css).toContain("var(--bg-base)");
    expect(css).not.toContain("--fgColor-default");
  });

  it("scales the 1280px slide to the frame width", async () => {
    // `scale(calc(100cqw / 1280))` is invalid CSS; the ratio is measured and
    // applied inline, and the observer keeps it through resizes.
    const callbacks: Array<() => void> = [];
    class TestResizeObserver {
      constructor(callback: () => void) {
        callbacks.push(callback);
      }
      observe() {}
      disconnect() {}
      unobserve() {}
    }
    vi.stubGlobal("ResizeObserver", TestResizeObserver);
    try {
      render(<SlidesView markdown={DECK} />);
      await waitFor(() => expect(screen.getByText("1 / 3")).toBeTruthy());
      const frame = document.querySelector<HTMLElement>(".slides-frame");
      const slide = document.querySelector<HTMLElement>(".slides-frame__slide");
      if (!frame || !slide) throw new Error("frame not rendered");
      Object.defineProperty(frame, "clientWidth", { value: 640, configurable: true });
      for (const callback of callbacks) callback();
      expect(slide.style.transform).toBe("scale(0.5)");
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
