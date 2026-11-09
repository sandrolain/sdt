// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { SlidesView } from "./SlidesView";

/**
 * A material deck puts its machine header comment first and Marp's frontmatter
 * after it (decision 0038). Marp only reads frontmatter at the start, so
 * without normalization the deck opens on a blank slide and shows the
 * frontmatter keys as content. This reads the real derived file, not a copy.
 */
const DECK_PATH = "context/renders/materials/software-architect/bounded-context.it.slide.md";
const DECK_FILE = resolve(dirname(fileURLToPath(import.meta.url)), "../../../", DECK_PATH);

afterEach(() => cleanup());

describe("SlidesView material deck", () => {
  it("starts on the title slide, not on the material header", { timeout: 20000 }, async () => {
    const markdown = readFileSync(DECK_FILE, "utf8");
    render(<SlidesView markdown={markdown} path={DECK_PATH} />);
    await waitFor(() => expect(screen.getByText("1 / 14")).toBeTruthy());
    const frame = document.querySelector(".slides-frame__slide");
    expect(frame?.querySelector("h1")?.textContent).toContain("Il bounded context");
    expect(frame?.textContent).not.toContain("theme: default");
    expect(frame?.textContent).not.toContain("paginate: true");
  });
});
