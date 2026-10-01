---
theme: default
paginate: true
size: 16:9
title: "Deck fixture: the portable subset"
description: "Calibration deck exercising every construct the slide lane must render or flag."
---

<!-- A deck-wide directive comment (global) plus this front-matter block. -->

# Deck fixture: the portable subset

One deck that exercises the constructs this lane must render, and the defects
the advisory checks must flag. It is the calibration corpus for both the viewer
and `sdt context lint`.

<!-- _class: lead -->

## Directives, fragmented lists and notes

A spot directive above sets this slide's class. The list below is a _fragmented
list_ (bullet marker `*`), so the renderer marks up reveals.

- the first fragment
- the second fragment
- the third fragment

<!-- This comment is a speaker note: it is not a directive, so the engine
collects it as presenter notes rather than consuming it. -->

---

### Code with highlighting

```go
func split(md string) []string {
	return strings.Split(md, "\n---\n")
}
```

---

### A KaTeX formula

Inline math $E = mc^2$ and a display block:

$$
\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}
$$

---

### A relative-path image

![w:480 the fixture caption](assets/fixture.png)

The image uses Marpit's extended sizing keyword and a path relative to this
document.

---

<!-- _paginate: skip -->

## The end

A closing slide with a spot `paginate: skip` directive, so the counter stops
before it.
