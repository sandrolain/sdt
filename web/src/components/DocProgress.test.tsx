// @vitest-environment jsdom
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DocProgress } from "./DocProgress";
import { resetActiveSection, setActiveSection } from "../lib/activeSection";

const BODY = `
  <h2 data-heading="Context">Context</h2>
  <p>one</p>
  <h3 data-heading="Detail">Detail</h3>
  <p>two</p>
  <h2 data-heading="Findings">Findings</h2>
  <p>three</p>
`;

/** A scroller with real-ish metrics: jsdom reports zero for every box. */
function scroller(scrollHeight = 3000, clientHeight = 600) {
  const el = document.createElement("div");
  el.className = "doc-rendered";
  el.innerHTML = BODY;
  Object.defineProperty(el, "scrollHeight", { value: scrollHeight, configurable: true });
  Object.defineProperty(el, "clientHeight", { value: clientHeight, configurable: true });
  document.body.appendChild(el);
  return el;
}

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
  resetActiveSection();
});

function renderProgress(container: HTMLElement) {
  return render(<DocProgress container={container} path="context/notes/x.md" />);
}

describe("DocProgress", () => {
  it("reports no progress at the top and disables back-to-top", () => {
    const el = scroller();
    renderProgress(el);
    const bar = screen.getByRole("progressbar", { name: "Reading progress" });
    expect(bar.getAttribute("aria-valuenow")).toBe("0");
    expect(screen.getByRole("button", { name: "Back to top" })).toHaveProperty("disabled", true);
  });

  it("tracks the scroll ratio", async () => {
    const el = scroller();
    renderProgress(el);
    el.scrollTop = 1200; // of 2400 scrollable
    await act(async () => {
      el.dispatchEvent(new Event("scroll"));
      await new Promise((resolve) => requestAnimationFrame(resolve));
    });
    await waitFor(() =>
      expect(
        screen.getByRole("progressbar", { name: "Reading progress" }).getAttribute("aria-valuenow"),
      ).toBe("50"),
    );
  });

  it("scrolls back to the top", async () => {
    const el = scroller();
    const scrollTo = vi.fn();
    el.scrollTo = scrollTo;
    renderProgress(el);
    el.scrollTop = 900;
    await act(async () => {
      el.dispatchEvent(new Event("scroll"));
      await new Promise((resolve) => requestAnimationFrame(resolve));
    });
    await userEvent.click(screen.getByRole("button", { name: "Back to top" }));
    expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "smooth" });
  });

  it("exposes a compact outline and marks the heading in view", async () => {
    const el = scroller();
    setActiveSection("context/notes/x.md", "Detail");
    renderProgress(el);
    await userEvent.click(screen.getByRole("button", { name: "Show outline" }));
    const outline = screen.getByRole("navigation", { name: "Document outline" });
    const items = Array.from(outline.querySelectorAll("button")).map((b) => b.textContent);
    expect(items).toEqual(["Context", "Detail", "Findings"]);
    const active = outline.querySelector(".doc-outline__item.is-active");
    expect(active?.textContent).toBe("Detail");
    expect(active?.getAttribute("aria-current")).toBe("location");
  });

  it("scrolls to a heading when the outline entry is used", async () => {
    const el = scroller();
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    renderProgress(el);
    await userEvent.click(screen.getByRole("button", { name: "Show outline" }));
    await userEvent.click(screen.getByRole("button", { name: "Findings" }));
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("hides the outline toggle for a document with a single heading", () => {
    const el = scroller();
    el.innerHTML = "<h2 data-heading='Only'>Only</h2><p>text</p>";
    renderProgress(el);
    expect(screen.queryByRole("button", { name: "Show outline" })).toBeNull();
  });
});
