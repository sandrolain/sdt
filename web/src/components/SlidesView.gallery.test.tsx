// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SlidesView } from "./SlidesView";

/**
 * The gallery is a corpus document, so it is read from disk rather than
 * duplicated here: the deck a reader opens in the Notes tree is the deck this
 * test renders. Assertions are structural (containers, resolved URLs, engine
 * markers), never prose, so editing the deck's wording does not break them.
 */
const DECK_PATH = "context/notes/20261109-201359-marp-feature-gallery.slide.md";
const DECK_FILE = resolve(dirname(fileURLToPath(import.meta.url)), "../../../", DECK_PATH);

function splitDeck(content: string): { frontmatter: string; markdown: string } {
  if (!content.startsWith("---")) return { frontmatter: "", markdown: content };
  const close = content.indexOf("\n---", 3);
  if (close < 0) return { frontmatter: "", markdown: content };
  const end = content.indexOf("\n", close + 4);
  const cut = end < 0 ? content.length : end + 1;
  return { frontmatter: content.slice(0, cut), markdown: content.slice(cut) };
}

const { frontmatter, markdown } = splitDeck(readFileSync(DECK_FILE, "utf8"));

function deckFrame(): HTMLElement {
  const frame = document.querySelector<HTMLElement>(".slides-frame__slide");
  if (!frame) throw new Error("slide frame not rendered");
  return frame;
}

function press(key: string, times = 1): void {
  for (let i = 0; i < times; i += 1) {
    window.dispatchEvent(new KeyboardEvent("keydown", { key }));
  }
}

afterEach(() => cleanup());

describe("SlidesView gallery", () => {
  it(
    "renders the corpus gallery with the token theme and Marp's containers",
    { timeout: 20000 },
    async () => {
      render(<SlidesView markdown={markdown} frontmatter={frontmatter} path={DECK_PATH} />);
      await waitFor(() => expect(screen.getByText("1 / 14")).toBeTruthy());

      expect(deckFrame().classList.contains("marpit")).toBe(true);
      expect(deckFrame().querySelector("header")?.textContent).toContain("Marp feature gallery");
      expect(deckFrame().querySelector("footer")?.textContent).toContain("verification deck");

      const css = document.querySelector(".slides-view style")?.textContent ?? "";
      expect(css).toContain("var(--bg-base)");
      // the deck's own `style:` directive survives into the packed stylesheet
      expect(css).toContain("letter-spacing");
    },
  );

  it("exposes the deck's speaker notes", async () => {
    const user = userEvent.setup();
    render(<SlidesView markdown={markdown} frontmatter={frontmatter} path={DECK_PATH} />);
    await waitFor(() => expect(screen.getByText("1 / 14")).toBeTruthy());
    await user.click(screen.getByLabelText("Show speaker notes"));
    expect(screen.getByText(/smoke test/)).toBeTruthy();
  });

  it("resolves the inline image and the background through /api/file", async () => {
    render(<SlidesView markdown={markdown} frontmatter={frontmatter} path={DECK_PATH} />);
    await waitFor(() => expect(screen.getByText("1 / 14")).toBeTruthy());

    press("ArrowRight", 6);
    await waitFor(() => expect(screen.getByText("7 / 14")).toBeTruthy());
    expect(deckFrame().querySelector("img")?.getAttribute("src")).toBe(
      "/api/file?path=context%2Fassets%2Fthe-spec-is-the-code.jpg",
    );

    press("ArrowRight");
    await waitFor(() => expect(screen.getByText("8 / 14")).toBeTruthy());
    expect(deckFrame().querySelector("section")?.getAttribute("style")).toContain(
      "/api/file?path=context%2Fassets%2Fthe-spec-is-the-code.jpg",
    );
  });

  it("keeps the KaTeX slide's math and the fragmented list markers", async () => {
    render(<SlidesView markdown={markdown} frontmatter={frontmatter} path={DECK_PATH} />);
    await waitFor(() => expect(screen.getByText("1 / 14")).toBeTruthy());

    press("ArrowRight", 4);
    await waitFor(() => expect(screen.getByText("5 / 14")).toBeTruthy());
    expect(deckFrame().querySelector(".katex")).toBeTruthy();

    press("Home");
    await waitFor(() => expect(screen.getByText("1 / 14")).toBeTruthy());
    press("ArrowRight", 2);
    await waitFor(() => expect(screen.getByText("3 / 14")).toBeTruthy());
    expect(deckFrame().querySelector("[data-marpit-fragment]")).toBeTruthy();
  });
});
