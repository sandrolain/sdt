/**
 * The map dialect's marker layer: the closed XMindMark subset the viewer
 * understands, read from a node's **own** text (plan D5). A marker is never
 * matched by re-deriving a key from another representation — the annotation is
 * produced where the node is created, which is the defect class that made the
 * old `[B]` overlay match zero rects at runtime.
 *
 * Grammar (markers abut the topic content; `\[\]` escapes a literal bracket):
 *
 * | Marker          | Meaning                                   |
 * |-----------------|-------------------------------------------|
 * | `[B]` `[B1]`    | boundary membership (id `0` / `1`)        |
 * | `[S]` `[S1]`    | summary membership (id `0` / `1`)          |
 * | `[3]` `[^3]`    | relationship source / target (`(Title)`)  |
 * | `[N:text]`      | note; repeatable, plain text              |
 * | `[L:url]`       | hyperlink                                 |
 * | `[F]`           | folded, children hidden on open           |
 * | `[!name]`       | SDT sticker (XMindMark has no sticker)    |
 * | `#group/name`   | SDT cross-cutting group (see `groups.ts`)|
 *
 * Markers are read wherever they appear in a node's text, so one written inside
 * an inline code span or a link label is still read: do not write markers there.
 *
 * A `[B<n>]: title` / `[S<n>]: title` line is a **declaration**, not a topic:
 * `extractMarkerTitles` lifts it out of the document before it is lexed.
 * Documented deviations from the XMindMark specification: no summary
 * sub-topics, no relationship on a title line, and boundary/summary ids are
 * document-global rather than indent-scoped.
 */

/** A `[n]` / `[^n]` pair end, with the optional `(Title)` after the target. */
export interface NodeRelation {
  id: string;
  title?: string;
  direction: "source" | "target";
}

/** Every marker one node carries. */
export interface NodeMarkers {
  /** `[B]` / `[B<n>]`; the first marker wins — a node joins one boundary. */
  boundary?: string;
  /** `[S]` / `[S<n>]`; the first marker wins — a node joins one summary. */
  summary?: string;
  /** `[n]` / `[^n]`; a node carries one relationship end. */
  relation?: NodeRelation;
  /** `[N:…]`, repeatable and rendered in the note tooltip. */
  notes: string[];
  /** `[L:url]`. */
  link?: string;
  /** `[F]`. */
  folded: boolean;
  /** `[!name]`, repeatable. */
  stickers: string[];
  /** `#group/name`, repeatable. */
  groups: string[];
}

/** Boundary and summary titles, keyed by id. */
export interface MarkerTitles {
  boundaries: Map<string, string>;
  summaries: Map<string, string>;
}

const MARKER_PATTERN = [
  "\\[\\^(?<relTarget>\\d+)\\](?:\\((?<relTitle>[^)]*)\\))?",
  "\\[(?<relSource>\\d+)\\](?:\\((?<relSourceTitle>[^)]*)\\))?",
  "\\[(?<wrapKind>[BS])(?<wrapId>\\d*)\\]",
  "\\[(?<fold>F)\\]",
  "\\[N:(?<note>[^\\]]*)\\]",
  "\\[L:(?<link>[^\\]]*)\\]",
  "\\[!(?<sticker>[A-Za-z0-9][\\w-]*)\\]",
  "#group\\/(?<group>[A-Za-z0-9][\\w-]*)",
].join("|");

/** A fresh regex per call: a shared global would carry `lastIndex` across nodes. */
function markerRegex(): RegExp {
  return new RegExp(MARKER_PATTERN, "g");
}

const TITLE_LINE_RE = /^[ \t]*\[([BS])(\d*)\]:[ \t]*(.*?)[ \t]*$/;

function applyMarker(markers: NodeMarkers, match: RegExpExecArray): void {
  const g = match.groups ?? {};
  if (g.relTarget !== undefined) {
    markers.relation = { id: g.relTarget, title: g.relTitle || undefined, direction: "target" };
  } else if (g.relSource !== undefined) {
    markers.relation = {
      id: g.relSource,
      title: g.relSourceTitle || undefined,
      direction: "source",
    };
  } else if (g.wrapKind !== undefined) {
    const id = g.wrapId || "0";
    const slot = g.wrapKind === "B" ? "boundary" : "summary";
    markers[slot] ??= id;
  } else if (g.fold !== undefined) {
    markers.folded = true;
  } else if (g.note !== undefined) {
    markers.notes.push(g.note.trim());
  } else if (g.link !== undefined) {
    markers.link ??= g.link.trim();
  } else if (g.sticker !== undefined) {
    markers.stickers.push(g.sticker);
  } else if (g.group !== undefined && !markers.groups.includes(g.group)) {
    markers.groups.push(g.group);
  }
}

/** Tidy the label left behind by removed markers and unescaped brackets. */
function tidy(label: string): string {
  return label
    .replace(/\\([[\]])/g, "$1")
    .replace(/[ \t]{2,}/g, " ")
    .trim();
}

/**
 * Read every marker out of one node's raw text and return the label with the
 * markers removed. Unrecognised bracket text (`[ ]`, `[x]`, a markdown
 * reference) stays in the label. A marker abuts the content, so the space in
 * front of it belongs to the marker and goes with it; a space after it stays.
 */
export function parseNodeMarkers(raw: string): { label: string; markers: NodeMarkers } {
  const markers: NodeMarkers = { notes: [], folded: false, stickers: [], groups: [] };
  const re = markerRegex();
  let label = "";
  let cursor = 0;
  for (let match = re.exec(raw); match; match = re.exec(raw)) {
    label += raw.slice(cursor, match.index);
    cursor = match.index + match[0].length;
    const after = raw[cursor];
    if (label.endsWith(" ") && after !== undefined && after !== " ") {
      label = label.slice(0, -1);
    }
    applyMarker(markers, match);
  }
  label += raw.slice(cursor);
  return { label: tidy(label), markers };
}

/**
 * Lift the `[B<n>]: title` / `[S<n>]: title` declaration lines out of the
 * markdown, so a title never becomes a topic, and return the titles they
 * declare. Markers on such a line are ignored (documented deviation).
 */
export function extractMarkerTitles(md: string): { titles: MarkerTitles; body: string } {
  const titles: MarkerTitles = { boundaries: new Map(), summaries: new Map() };
  const body = md
    .split("\n")
    .filter((line) => {
      const match = TITLE_LINE_RE.exec(line);
      if (!match) return true;
      (match[1] === "B" ? titles.boundaries : titles.summaries).set(match[2] || "0", match[3]);
      return false;
    })
    .join("\n");
  return { titles, body };
}
