import { describe, expect, it } from "vitest";
import { annotateGroups, parseGroups, stripGroupTags, stripGroupTagsFromMarkdown } from "./groups";
import { nodeText, parseBoundaries } from "./boundaries";
import type { MindNode } from "./mindmap";

function node(content: string, children: MindNode[] = []): MindNode {
  return { content, children };
}

describe("stripGroupTags", () => {
  it("removes a trailing, leading or inner tag and tidies spacing", () => {
    expect(stripGroupTags("topic #group/auth")).toBe("topic");
    expect(stripGroupTags("#group/auth topic")).toBe("topic");
    expect(stripGroupTags("a #group/x b")).toBe("a b");
    expect(stripGroupTags("<strong>topic</strong> #group/auth-core")).toBe(
      "<strong>topic</strong>",
    );
  });

  it("leaves content without a tag untouched", () => {
    expect(stripGroupTags("plain topic")).toBe("plain topic");
  });
});

describe("stripGroupTagsFromMarkdown", () => {
  it("strips tags line-wise so boundary keys align with the stripped tree", () => {
    const md = "- alpha [B1] #group/core\n- beta [B1]\n  [B1]: G";
    const parsed = parseBoundaries(stripGroupTagsFromMarkdown(md));
    expect(parsed.mark.get("alpha")).toBe("1");
    expect(parsed.mark.get("beta")).toBe("1");
  });
});

describe("parseGroups", () => {
  it("collects memberships, multiple tags per node and ignores unrelated headings", () => {
    const md = [
      "- alpha #group/auth #group/core",
      "- beta #group/auth",
      "## gamma",
      "- delta #group/auth-core",
    ].join("\n");
    const parsed = parseGroups(md);
    expect(parsed.get("alpha")).toEqual(["auth", "core"]);
    expect(parsed.get("beta")).toEqual(["auth"]);
    expect(parsed.get("delta")).toEqual(["auth-core"]);
    expect(parsed.has("gamma")).toBe(false);
  });

  it("ignores an empty or malformed name", () => {
    expect([...parseGroups("topic #group/").keys()]).toEqual([]);
    expect([...parseGroups("topic #group//x").keys()]).toEqual([]);
  });
});

describe("annotateGroups", () => {
  it("groups non-consecutive members regardless of position", () => {
    const root = node("root", [
      node("a #group/g"),
      node("b"),
      node("c #group/g"),
      node("parent", [node("d #group/g")]),
    ]);
    const parsed = parseGroups(
      ["- a #group/g", "- b", "- c #group/g", "- parent", "  - d #group/g"].join("\n"),
    );
    const groups = annotateGroups(root, parsed);
    expect(groups).toHaveLength(1);
    expect(groups[0].id).toBe("g");
    expect(groups[0].nodes.map((n) => nodeText(n))).toEqual([
      "a #group/g",
      "c #group/g",
      "d #group/g",
    ]);
  });

  it("returns groups in first-seen order and matches tag-stripped tree content", () => {
    const root = node("root", [node("a #group/one"), node("b #group/two")]);
    const groups = annotateGroups(root, parseGroups("- a #group/one\n- b #group/two"));
    expect(groups.map((g) => g.id)).toEqual(["one", "two"]);
  });
});
