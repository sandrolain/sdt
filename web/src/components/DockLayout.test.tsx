// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DockLayout, type DockPanelDef } from "./DockLayout";
import { loadLayout, saveLayout } from "../lib/layoutStore";

const PANELS: DockPanelDef[] = [
  { id: "tree", title: "Tree", icon: "account_tree", render: () => <div>tree body</div> },
  { id: "content", title: "Document", render: () => <div>content body</div> },
  { id: "meta", title: "Metadata", render: () => <div>meta body</div> },
];

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("DockLayout", () => {
  it("renders every panel", () => {
    render(<DockLayout storageKey="workspace" panels={PANELS} />);
    expect(screen.getByText("tree body")).toBeTruthy();
    expect(screen.getByText("content body")).toBeTruthy();
    expect(screen.getByText("meta body")).toBeTruthy();
  });

  it("collapses and reopens a panel, persisting visibility", async () => {
    render(<DockLayout storageKey="workspace" panels={PANELS} />);
    await userEvent.click(screen.getByTitle("Hide Tree"));
    expect(screen.queryByText("tree body")).toBeNull();
    expect(loadLayout("workspace:hidden")).toEqual(["tree"]);

    const reopen = screen.getByRole("button", { name: "Tree" });
    await userEvent.click(reopen);
    expect(screen.getByText("tree body")).toBeTruthy();
    expect(loadLayout("workspace:hidden")).toEqual([]);
  });

  it("restores a persisted hidden panel", () => {
    saveLayout("workspace:hidden", ["meta"]);
    render(<DockLayout storageKey="workspace" panels={PANELS} />);
    expect(screen.queryByText("meta body")).toBeNull();
    expect(screen.getByRole("button", { name: "Metadata" })).toBeTruthy();
  });
});
