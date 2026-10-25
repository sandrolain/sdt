/**
 * The delegated controls the corpus renderer puts inside rendered markdown.
 *
 * The renderer emits a wrap toggle and a copy button inside every code block, and the
 * document page answers them by delegation on its rendered container. The agent panel
 * renders the same markup, so it delegates to the same function: a second copy of these
 * handlers is how the two surfaces start disagreeing about what "copy" copies.
 *
 * Anchor buttons are not handled here: copying a section link is a document-link action
 * (the router owns the fragment), and a chat message has no section to link to.
 */
import { codeBlockText } from "./codeLines";

/** Copy the sibling `<code>` text and flash a success glyph on the button. */
function copyCodeBlock(button: Element): void {
  const code = button.closest(".md-code-block")?.querySelector("code");
  if (!code || !navigator.clipboard) return;
  void navigator.clipboard.writeText(codeBlockText(code)).then(() => {
    const glyph = button.querySelector(".ms-icon");
    const previous = glyph?.textContent ?? "content_copy";
    if (glyph) glyph.textContent = "check";
    button.setAttribute("aria-label", "Copied");
    window.setTimeout(() => {
      if (glyph) glyph.textContent = previous;
      button.setAttribute("aria-label", "Copy code");
    }, 1200);
  });
}

/**
 * markdownControlAction answers one of the renderer's controls and reports whether it
 * did, so a caller that also routes links can stop before touching the click.
 */
export function markdownControlAction(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null;
  const wrapButton = element?.closest?.(".md-code__wrap");
  if (wrapButton) {
    wrapButton
      .closest(".md-code-block")
      ?.querySelector("pre.md-code")
      ?.classList.toggle("is-wrapped");
    return true;
  }
  const copyButton = element?.closest?.(".md-code__copy");
  if (copyButton) {
    copyCodeBlock(copyButton);
    return true;
  }
  return false;
}
