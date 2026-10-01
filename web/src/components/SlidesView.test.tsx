// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SlidesView } from "./SlidesView";

const DECK = `---\ntheme: default\n---\n\n# One\n\nfirst\n\n---\n\n## Two\n\n<!-- a note -->\n\nsecond\n\n---\n\n## Three\n\nthird\n`;

afterEach(() => cleanup());

describe("SlidesView", () => {
  it("renders the deck's slides and a counter", async () => {
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
});
