import { describe, expect, it, vi } from "vitest";
import { addSidePanels, SIDE_PANELS, type SidePanelApi } from "./workspaceLayout";

function fakeApi(existing: string[] = []) {
  const added: string[] = [];
  const api: SidePanelApi = {
    getPanel: (id) => (existing.includes(id) ? {} : undefined),
    addPanel: (options) => {
      added.push(options.id);
      return {};
    },
  };
  return { api, added };
}

describe("addSidePanels", () => {
  it("adds both side panels to an empty workspace", () => {
    const { api, added } = fakeApi();
    addSidePanels(api);
    expect(added).toEqual(["tree", "meta"]);
  });

  it("re-adds only the missing panel", () => {
    const { api, added } = fakeApi(["tree"]);
    addSidePanels(api);
    expect(added).toEqual(["meta"]);
  });

  it("never closes or reorders existing panels", () => {
    const { api, added } = fakeApi(["tree", "meta"]);
    addSidePanels(api);
    expect(added).toEqual([]);
  });

  it("uses narrower default widths", () => {
    const tree = SIDE_PANELS.find((p) => p.id === "tree");
    const meta = SIDE_PANELS.find((p) => p.id === "meta");
    expect(tree?.initialWidth).toBe(210);
    expect(meta?.initialWidth).toBe(260);
  });
});

describe("no-op sanity", () => {
  it("keeps the injected api untouched when complete", () => {
    const spy = vi.fn();
    addSidePanels({ getPanel: () => ({}), addPanel: spy });
    expect(spy).not.toHaveBeenCalled();
  });
});
