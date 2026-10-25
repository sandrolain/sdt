// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { markdownControlAction } from "./markdownActions";

/** block is the markup the corpus renderer emits for one fenced code block. */
function block(text = "const x = 1;"): HTMLElement {
  document.body.innerHTML = [
    '<div class="md-code-block">',
    '<button type="button" class="md-code__wrap" aria-label="Toggle line wrapping">',
    '<span class="ms-icon">wrap_text</span></button>',
    '<button type="button" class="md-code__copy" aria-label="Copy code">',
    '<span class="ms-icon">content_copy</span></button>',
    `<pre class="md-code"><code>${text}</code></pre>`,
    "</div>",
  ].join("");
  return document.body;
}

afterEach(() => {
  document.body.innerHTML = "";
  Reflect.deleteProperty(navigator, "clipboard");
  vi.restoreAllMocks();
});

describe("markdownControlAction", () => {
  it("toggles the wrapping of the code block it belongs to", () => {
    const root = block();
    const wrap = root.querySelector(".md-code__wrap");
    const pre = root.querySelector("pre.md-code");

    expect(markdownControlAction(wrap)).toBe(true);
    expect(pre?.classList.contains("is-wrapped")).toBe(true);
    markdownControlAction(wrap);
    expect(pre?.classList.contains("is-wrapped")).toBe(false);
  });

  it("copies the block's code from its copy button", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    const root = block("const a = 1;\nconst b = 2;");

    expect(markdownControlAction(root.querySelector(".md-code__copy"))).toBe(true);
    expect(writeText).toHaveBeenCalledWith("const a = 1;\nconst b = 2;");
  });

  it("answers a click that is not one of its controls", () => {
    const root = block();
    expect(markdownControlAction(root.querySelector("code"))).toBe(false);
    expect(markdownControlAction(null)).toBe(false);
  });
});
