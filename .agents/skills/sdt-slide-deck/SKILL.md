---
name: sdt-slide-deck
description: >
  Use this skill to author, edit or review a slide deck: writing a
  `.slide.md`/Marp deck, structuring a presentation, choosing a theme or
  layout, or checking that a deck renders and exports cleanly — even when the
  request does not mention "slide", "deck", "Marp" or "presentation"
  explicitly. It routes to the project's slide-deck instruction files; it does
  not replace them.
---

<!-- sdt:begin:skills/sdt-slide-deck/SKILL -->

# SDT slide-deck skill

Read the project's deck instruction files before authoring or reviewing a deck;
this skill routes to them and never restates them.

1. Read `context/instructions/slides.md` **before authoring any deck** (the
   format-agnostic doctrine).
2. Read `context/instructions/slides-marp.md` for the Marp / Marpit Markdown
   dialect, the portability split and the deck document-management convention.
3. Keep framework, tool and rendering specifics out of doctrine: follow the
   project's files where they exist.

If a deck is rendered by a tool, verify the rendered result before claiming it is
correct; do not describe what you expect in place of looking at it.

<!-- sdt:end:skills/sdt-slide-deck/SKILL -->
