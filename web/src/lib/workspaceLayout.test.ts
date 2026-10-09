import { describe, expect, it, vi } from "vitest";
import {
  addSidePanels,
  COLLAPSED_SIZE,
  ensureCenterGroup,
  ensureKindTabGroup,
  pruneEmptyTabGroups,
  SIDE_PANELS,
  type CenterApi,
  type CenterGroup,
  type EdgePosition,
  type SidePanelApi,
  type TabGroupApi,
} from "./workspaceLayout";
import { kindColor } from "./kinds";

function fakeApi(existing: { panels?: string[]; edges?: EdgePosition[] } = {}) {
  const panels = new Set(existing.panels ?? []);
  const edges = new Set(existing.edges ?? []);
  const addedPanels: string[] = [];
  const addedEdges: EdgePosition[] = [];
  const api: SidePanelApi = {
    getPanel: (id) => (panels.has(id) ? {} : undefined),
    getEdgeGroup: (position) => (edges.has(position) ? {} : undefined),
    addEdgeGroup: (position) => {
      edges.add(position);
      addedEdges.push(position);
      return {};
    },
    addPanel: (options) => {
      addedPanels.push(options.id);
      panels.add(options.id);
      return {};
    },
  };
  return { api, addedPanels, addedEdges };
}

describe("addSidePanels", () => {
  it("creates the edge groups and their panels in an empty workspace", () => {
    const { api, addedPanels, addedEdges } = fakeApi();
    addSidePanels(api);
    expect(addedEdges).toEqual(["left", "right"]);
    expect(addedPanels).toEqual([
      "tree",
      "meta-info",
      "meta-sections",
      "meta-links",
      "meta-related",
    ]);
  });

  it("re-adds a missing panel into an existing edge group", () => {
    const { api, addedPanels, addedEdges } = fakeApi({ panels: ["tree"], edges: ["left"] });
    addSidePanels(api);
    expect(addedEdges).toEqual(["right"]);
    expect(addedPanels).toEqual(["meta-info", "meta-sections", "meta-links", "meta-related"]);
  });

  it("never touches a complete workspace", () => {
    const { api, addedPanels, addedEdges } = fakeApi({
      panels: ["tree", "meta-info", "meta-sections", "meta-links", "meta-related"],
      edges: ["left", "right"],
    });
    addSidePanels(api);
    expect(addedEdges).toEqual([]);
    expect(addedPanels).toEqual([]);
  });

  it("uses the renamed titles and a collapsed strip size", () => {
    expect(SIDE_PANELS.map((p) => p.title)).toEqual([
      "Documents",
      "Info",
      "Sections",
      "Links",
      "Related",
    ]);
    expect(COLLAPSED_SIZE).toBeGreaterThan(0);
    expect(SIDE_PANELS.find((p) => p.id === "tree")?.initialSize).toBe(210);
    expect(SIDE_PANELS.find((p) => p.id === "meta-info")?.initialSize).toBe(260);
  });

  it("places each panel in its own edge group", () => {
    const spy = vi.fn(() => ({}));
    addSidePanels({
      getPanel: () => undefined,
      getEdgeGroup: () => undefined,
      addEdgeGroup: () => ({}),
      addPanel: spy,
    });
    expect(spy).toHaveBeenCalledWith(
      expect.objectContaining({ id: "tree", position: { referenceGroup: "edge-tree" } }),
    );
    expect(spy).toHaveBeenCalledWith(
      expect.objectContaining({ id: "meta-info", position: { referenceGroup: "edge-meta" } }),
    );
  });
});

describe("ensureCenterGroup", () => {
  function group(id: string, kind = "grid"): CenterGroup {
    return { id, api: { location: { type: kind } } };
  }

  function fakeCenter(panels: CenterApi["panels"] = [], groups: CenterGroup[] = []): CenterApi {
    return {
      panels,
      groups,
      addGroup: () => {
        const g = group("center-new");
        groups.push(g);
        return g;
      },
    };
  }

  it("reuses a cached group that is still present", () => {
    const center = group("center-1");
    const ref = { current: center };
    const api = fakeCenter([], [center]);
    expect(ensureCenterGroup(api, ref).id).toBe("center-1");
  });

  it("follows the group that already hosts a document", () => {
    const center = group("center-docs");
    const api = fakeCenter([{ id: "doc:context/a.md", group: center }], [group("edge"), center]);
    const ref: { current: CenterGroup | null } = { current: null };
    expect(ensureCenterGroup(api, ref).id).toBe("center-docs");
    expect(ref.current?.id).toBe("center-docs");
  });

  it("falls back to the placeholder group", () => {
    const center = group("center-courtesy");
    const api = fakeCenter([{ id: "doc-courtesy", group: center }], [center]);
    expect(ensureCenterGroup(api, { current: null }).id).toBe("center-courtesy");
  });

  it("uses any grid group then creates one", () => {
    const grid = group("grid-1");
    expect(
      ensureCenterGroup(fakeCenter([], [group("edge", "edge"), grid]), { current: null }).id,
    ).toBe("grid-1");
    expect(ensureCenterGroup(fakeCenter([], [group("edge", "edge")]), { current: null }).id).toBe(
      "center-new",
    );
  });
});

describe("ensureKindTabGroup", () => {
  function fakeTabApi() {
    const groups: { id: string; label: string; color?: string }[] = [];
    const added: { groupId: string; tabGroupId: string; panelId: string }[] = [];
    const membership = new Map<string, string>();
    let n = 0;
    const api: TabGroupApi = {
      getTabGroupForPanel: ({ panelId }) => {
        const id = membership.get(panelId);
        return id ? groups.find((g) => g.id === id) : undefined;
      },
      getTabGroups: () => groups,
      createTabGroup: ({ label, color }) => {
        const g = { id: `tg${++n}`, label: label ?? "", color };
        groups.push(g);
        return g;
      },
      addPanelToTabGroup: ({ groupId, tabGroupId, panelId }) => {
        membership.set(panelId, tabGroupId);
        added.push({ groupId, tabGroupId, panelId });
      },
    };
    return { api, groups, added };
  }

  const panel = (id: string, groupId = "center") => ({ id, group: { id: groupId } });

  it("creates a kind-coloured group and adds the panel", () => {
    const { api, groups, added } = fakeTabApi();
    ensureKindTabGroup(api, panel("doc:context/analysis/a.md"), "analysis");
    expect(groups).toEqual([{ id: "tg1", label: "Analyses", color: kindColor("analysis") }]);
    expect(added).toEqual([
      { groupId: "center", tabGroupId: "tg1", panelId: "doc:context/analysis/a.md" },
    ]);
  });

  it("reuses one group per kind", () => {
    const { api, groups, added } = fakeTabApi();
    ensureKindTabGroup(api, panel("doc:a"), "analysis");
    ensureKindTabGroup(api, panel("doc:b"), "analysis");
    expect(groups).toHaveLength(1);
    expect(added).toHaveLength(2);
  });

  it("is a no-op for a panel already in a tab group", () => {
    const { api, groups, added } = fakeTabApi();
    ensureKindTabGroup(api, panel("doc:a"), "wiki");
    const before = added.length;
    ensureKindTabGroup(api, panel("doc:a"), "wiki");
    expect(groups).toHaveLength(1);
    expect(added).toHaveLength(before);
  });

  it("groups an unknown kind under Other", () => {
    const { api, groups } = fakeTabApi();
    ensureKindTabGroup(api, panel("doc:x"), "other");
    expect(groups[0].label).toBe("Other");
  });
});

describe("pruneEmptyTabGroups", () => {
  it("dissolves only the empty tab groups", () => {
    const dissolved: { groupId: string; tabGroupId: string }[] = [];
    const api = {
      groups: [{ id: "center" }, { id: "edge" }],
      getTabGroups: ({ groupId }: { groupId: string }) =>
        groupId === "center"
          ? [
              { id: "tg-full", isEmpty: false },
              { id: "tg-empty", isEmpty: true },
            ]
          : [],
      dissolveTabGroup: (o: { groupId: string; tabGroupId: string }) => dissolved.push(o),
    };
    pruneEmptyTabGroups(api);
    expect(dissolved).toEqual([{ groupId: "center", tabGroupId: "tg-empty" }]);
  });

  it("does nothing when no tab group is empty", () => {
    const dissolve = vi.fn();
    pruneEmptyTabGroups({
      groups: [{ id: "center" }],
      getTabGroups: () => [{ id: "tg", isEmpty: false }],
      dissolveTabGroup: dissolve,
    });
    expect(dissolve).not.toHaveBeenCalled();
  });
});
