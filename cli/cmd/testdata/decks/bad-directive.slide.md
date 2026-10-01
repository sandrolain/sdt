---
theme: default
---

# A directive typo

<!-- _classs: lead -->

The directive above is misspelled, so Marpit silently ignores it and this slide
gets no lead styling.

<!-- layout: two-columns -->

A `layout:` key is not a Marp directive; it is silently ignored too.

---

## A missing image

![w:400 the caption](assets/does-not-exist.png)

The relative asset path above does not resolve, so the slide ships with a broken
image.
