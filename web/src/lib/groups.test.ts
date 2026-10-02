import { describe, expect, it } from "vitest";
import { nodeText } from "./boundaries";
import { annotateGroups } from "./groups";
import { measureTree } from "./mapMetrics";
import { parseMapDocument } from "./mindmap";

describe("annotateGroups", () => {
  it("groups non-consecutive members regardless of position", () => {
    const root = parseMapDocument(
      ["# Root", "", "- a #group/g", "- b", "- c #group/g", "- parent", "  - d #group/g"].join(
        "\n",
      ),
    );
    const groups = annotateGroups(measureTree(root));
    expect(groups).toHaveLength(1);
    expect(groups[0].id).toBe("g");
    expect(groups[0].nodes.map((n) => nodeText(n))).toEqual(["a", "c", "d"]);
    expect(groups[0].nodes.map((n) => n.id)).toEqual(["n0.1", "n0.3", "n0.4.1"]);
  });

  it("returns groups in first-seen order and keeps several tags per node", () => {
    const tree = measureTree(
      parseMapDocument("# Root\n\n- a #group/one #group/core\n- b #group/two\n"),
    );
    const groups = annotateGroups(tree);
    expect(groups.map((g) => g.id)).toEqual(["one", "core", "two"]);
    expect(groups[0].nodes).toHaveLength(1);
  });

  it("strips the tag from the rendered label", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- a #group/g\n"));
    expect(nodeText(tree.children[0])).toBe("a");
    expect(annotateGroups(tree)[0].id).toBe("g");
  });

  it("ignores a malformed or empty group name", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- a #group/\n- b #group//x\n"));
    expect(annotateGroups(tree)).toHaveLength(0);
  });
});
