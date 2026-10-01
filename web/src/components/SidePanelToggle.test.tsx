// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SidePanelProvider } from "./SidePanelProvider";
import { SidePanelToggle } from "./SidePanelToggle";
import type { EdgePosition } from "../lib/workspaceLayout";

/** Minimal dockview stand-in: stateful collapse with a change listener. */
function fakeApi(initial: Partial<Record<EdgePosition, boolean>> = {}) {
  const state: Record<EdgePosition, boolean> = { left: !!initial.left, right: !!initial.right };
  const listeners = new Set<() => void>();
  const emit = () => listeners.forEach((l) => l());
  const group = (position: EdgePosition) => ({
    isCollapsed: () => state[position],
    collapse: vi.fn(() => {
      state[position] = true;
      emit();
    }),
    expand: vi.fn(() => {
      state[position] = false;
      emit();
    }),
    onDidCollapsedChange: (listener: () => void) => {
      listeners.add(listener);
      return { dispose: () => listeners.delete(listener) };
    },
  });
  const groups = { left: group("left"), right: group("right") };
  return {
    api: {
      getEdgeGroup: (position: EdgePosition) => groups[position],
      onDidLayoutChange: (listener: () => void) => {
        listeners.add(listener);
        return { dispose: () => listeners.delete(listener) };
      },
    },
    groups,
  };
}

afterEach(cleanup);

describe("SidePanelToggle", () => {
  it("renders nothing without the bridge", () => {
    const { container } = render(<SidePanelToggle position="left" label="Tree" />);
    expect(container.querySelector(".side-panel-toggle")).toBeNull();
  });

  it("collapses the panel and flips its label from the dockview state", async () => {
    const { api, groups } = fakeApi();
    render(
      <SidePanelProvider api={api}>
        <SidePanelToggle position="left" label="Tree" />
      </SidePanelProvider>,
    );
    const hide = screen.getByRole("button", { name: "Hide Tree" });
    expect(hide.getAttribute("data-collapsed")).toBeNull();
    await userEvent.click(hide);
    expect(groups.left.collapse).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.getByRole("button", { name: "Show Tree" })).toBeTruthy());
  });

  it("expands a collapsed panel", async () => {
    const { api, groups } = fakeApi({ right: true });
    render(
      <SidePanelProvider api={api}>
        <SidePanelToggle position="right" label="Info" />
      </SidePanelProvider>,
    );
    const show = screen.getByRole("button", { name: "Show Info" });
    expect(show.getAttribute("data-collapsed")).toBe("true");
    await userEvent.click(show);
    expect(groups.right.expand).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.getByRole("button", { name: "Hide Info" })).toBeTruthy());
  });

  it("maps the initial collapsed state independently per position", () => {
    const { api } = fakeApi({ left: true });
    render(
      <SidePanelProvider api={api}>
        <SidePanelToggle position="left" label="Tree" />
        <SidePanelToggle position="right" label="Info" />
      </SidePanelProvider>,
    );
    expect(screen.getByRole("button", { name: "Show Tree" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Hide Info" })).toBeTruthy();
  });
});
