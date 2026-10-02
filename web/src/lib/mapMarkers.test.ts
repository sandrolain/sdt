import { describe, expect, it } from "vitest";
import { extractMarkerTitles, parseNodeMarkers } from "./mapMarkers";
import { parseMapDocument } from "./mindmap";

describe("parseNodeMarkers", () => {
  it("reads every marker and strips it from the label", () => {
    const { label, markers } = parseNodeMarkers(
      "alpha [B1][S2][!star][3][N:remember this][L:https://example.com][F]",
    );
    expect(label).toBe("alpha");
    expect(markers).toEqual({
      notes: ["remember this"],
      folded: true,
      stickers: ["star"],
      groups: [],
      boundary: "1",
      summary: "2",
      link: "https://example.com",
      relation: { id: "3", title: undefined, direction: "source" },
    });
  });

  it("reads a relationship target with its title", () => {
    const { label, markers } = parseNodeMarkers("beta [^7](Cool)");
    expect(label).toBe("beta");
    expect(markers.relation).toEqual({ id: "7", title: "Cool", direction: "target" });
  });

  it("records both ends of a pair independently", () => {
    const source = parseNodeMarkers("Spring [1]");
    const target = parseNodeMarkers("Autumn [^1](Cool)");
    expect(source.markers.relation).toMatchObject({ id: "1", direction: "source" });
    expect(target.markers.relation).toMatchObject({ id: "1", direction: "target" });
    // An unpaired end is still recorded: the viewer drops the edge, the lint warns.
    expect(parseNodeMarkers("lonely [4]").markers.relation?.direction).toBe("source");
  });

  it("keeps notes, stickers and groups repeatable", () => {
    const { label, markers } = parseNodeMarkers(
      "topic [N:first][N:second] [!idea][!risk] #group/auth #group/core #group/auth",
    );
    expect(label).toBe("topic");
    expect(markers.notes).toEqual(["first", "second"]);
    expect(markers.stickers).toEqual(["idea", "risk"]);
    expect(markers.groups).toEqual(["auth", "core"]);
  });

  it("keeps the first boundary and summary, and the first link", () => {
    const { markers } = parseNodeMarkers("x [B1][B2][S1][S2][L:a][L:b]");
    expect(markers.boundary).toBe("1");
    expect(markers.summary).toBe("1");
    expect(markers.link).toBe("a");
  });

  it("treats an unnumbered boundary or summary as id 0", () => {
    expect(parseNodeMarkers("x [B]").markers.boundary).toBe("0");
    expect(parseNodeMarkers("x [S]").markers.summary).toBe("0");
  });

  it("keeps an unknown sticker name as a chip and a malformed one in the label", () => {
    const { label, markers } = parseNodeMarkers(
      "compare [ ] with [x], flag [!wibble], odd [!bad?] here",
    );
    expect(markers.stickers).toEqual(["wibble"]);
    expect(label).toBe("compare [ ] with [x], flag, odd [!bad?] here");
  });

  it("reads a marker wherever it sits, including inside a link label (documented)", () => {
    const { label, markers } = parseNodeMarkers("see [docs [B1]](https://example.com)");
    expect(markers.boundary).toBe("1");
    expect(label).toBe("see [docs](https://example.com)");
  });

  it("unescapes an escaped bracket pair and tidies the spacing left behind", () => {
    const { label, markers } = parseNodeMarkers("literal \\[\\] and [B1]   padded");
    expect(markers.boundary).toBe("1");
    expect(label).toBe("literal [] and padded");
  });

  it("is stateless across calls (no shared lastIndex)", () => {
    const raw = "alpha [B1]";
    expect(parseNodeMarkers(raw).markers.boundary).toBe("1");
    expect(parseNodeMarkers(raw).markers.boundary).toBe("1");
    expect(parseNodeMarkers("beta [S1]").markers.boundary).toBeUndefined();
  });
});

describe("extractMarkerTitles", () => {
  it("lifts title lines out of the body and keys them by id", () => {
    const md = ["- a [B1]", "- b [B1]", "  [B1]: Wrap", "  [S]: Sum", "- c [S]", ""].join("\n");
    const { titles, body } = extractMarkerTitles(md);
    expect(titles.boundaries.get("1")).toBe("Wrap");
    expect(titles.summaries.get("0")).toBe("Sum");
    expect(body).not.toContain("[B1]: Wrap");
    expect(body).not.toContain("[S]: Sum");
    expect(body).toContain("- a [B1]");
  });

  it("ignores a list item that merely starts with a bracket", () => {
    const { titles, body } = extractMarkerTitles("- [B1] not a title\n");
    expect(titles.boundaries.size).toBe(0);
    expect(body).toBe("- [B1] not a title\n");
  });

  it("drops a marker written on a title line (documented deviation)", () => {
    const { titles, body } = extractMarkerTitles("  [B1]: Wrap [1]\n");
    expect(titles.boundaries.get("1")).toBe("Wrap [1]");
    expect(body.trim()).toBe("");
  });
});

describe("the tree carries the markers", () => {
  it("annotates each node from its own text and strips the token from the label", () => {
    // The 2026-09-29 defect: the old parser keyed markers on a markdown line
    // while the overlay matched the node label, so nothing ever matched.
    const root = parseMapDocument(
      [
        "# Root",
        "",
        "- gamma [!flag]",
        "",
        "## alpha [B1]",
        "",
        "## beta [B1]",
        "",
        "[B1]: Wrap",
      ].join("\n"),
    );
    const [gamma, alpha, beta] = root.children;
    expect(alpha.content).toBe("alpha");
    expect(alpha.payload?.markers?.boundary).toBe("1");
    expect(beta.payload?.markers?.boundary).toBe("1");
    expect(root.payload?.titles?.boundaries.get("1")).toBe("Wrap");
    expect(gamma.payload?.markers?.stickers).toEqual(["flag"]);
    expect(gamma.content).toBe("gamma");
  });

  it("never turns a boundary or summary title into a node", () => {
    const root = parseMapDocument(
      ["# Root", "", "- a [B1]", "- b [B1]", "  [B1]: Wrap", "  [S1]: Sum", "- c [S1]"].join("\n"),
    );
    const labels = root.children.map((c) => c.content);
    expect(labels).toEqual(["a", "b", "c"]);
    expect(labels.join(" ")).not.toContain("Wrap");
    expect(labels.join(" ")).not.toContain("Sum");
  });
});
