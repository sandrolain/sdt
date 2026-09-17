// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { IDockviewPanelHeaderProps } from "dockview-react";
import { isTabClosable, tabContextMenuItems } from "../lib/workspaceTabs";
import { WorkspaceTab } from "./WorkspaceTab";

function fakeApi(id: string) {
  const disposable = { dispose: () => {} };
  return {
    id,
    title: id,
    isPinned: false,
    onDidTitleChange: () => disposable,
    onDidChangePinned: () => disposable,
    close: () => {},
  } as never;
}

function renderTab(id: string) {
  const props = {
    api: fakeApi(id),
    containerApi: {} as never,
    params: {},
    tabLocation: "header",
  } as IDockviewPanelHeaderProps;
  return render(<WorkspaceTab {...props} />);
}

afterEach(cleanup);

describe("isTabClosable", () => {
  it("keeps side and placeholder panels non-closable", () => {
    expect(isTabClosable("tree")).toBe(false);
    expect(isTabClosable("meta")).toBe(false);
    expect(isTabClosable("doc-courtesy")).toBe(false);
    expect(isTabClosable("doc:context/a.md")).toBe(true);
  });
});

describe("tabContextMenuItems", () => {
  it("offers no close entries for non-closable panels", () => {
    expect(tabContextMenuItems("tree")).toEqual([]);
    expect(tabContextMenuItems("doc:x.md")).toContain("close");
  });
});

describe("WorkspaceTab", () => {
  it("hides the close button for a side panel", () => {
    renderTab("tree");
    expect(screen.queryByLabelText("Close tab")).toBeNull();
  });

  it("keeps the close button for a document tab", () => {
    renderTab("doc:context/a.md");
    expect(screen.getByLabelText("Close tab")).toBeTruthy();
  });
});
