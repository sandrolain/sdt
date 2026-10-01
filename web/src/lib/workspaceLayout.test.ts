import { describe, expect, it, vi } from "vitest";
import {
  addSidePanels,
  COLLAPSED_SIZE,
  ensureCenterGroup,
  SIDE_PANELS,
  type CenterApi,
  type CenterGroup,
  type EdgePosition,
  type SidePanelApi,
} from "./workspaceLayout";

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
    expect(addedPanels).toEqual(["tree", "meta"]);
  });

  it("re-adds a missing panel into an existing edge group", () => {
    const { api, addedPanels, addedEdges } = fakeApi({ panels: ["tree"], edges: ["left"] });
    addSidePanels(api);
    expect(addedEdges).toEqual(["right"]);
    expect(addedPanels).toEqual(["meta"]);
  });

  it("never touches a complete workspace", () => {
    const { api, addedPanels, addedEdges } = fakeApi({
      panels: ["tree", "meta"],
      edges: ["left", "right"],
    });
    addSidePanels(api);
    expect(addedEdges).toEqual([]);
    expect(addedPanels).toEqual([]);
  });

  it("uses the renamed titles and a collapsed strip size", () => {
    expect(SIDE_PANELS.map((p) => p.title)).toEqual(["Documents", "Info"]);
    expect(COLLAPSED_SIZE).toBeGreaterThan(0);
    expect(SIDE_PANELS.find((p) => p.id === "tree")?.initialSize).toBe(210);
    expect(SIDE_PANELS.find((p) => p.id === "meta")?.initialSize).toBe(260);
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
      expect.objectContaining({ id: "meta", position: { referenceGroup: "edge-meta" } }),
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
