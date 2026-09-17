// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphToolsPanel } from "./GraphToolsPanel";
import { initialGraphTools } from "../lib/graphTools";

function renderPanel(props: {
  selectedId?: string | null;
  focusedId?: string | null;
  onClear?: () => void;
}) {
  const onClear = props.onClear ?? vi.fn();
  render(
    <GraphToolsPanel
      tools={initialGraphTools}
      allVerbs={["refers_to"]}
      allKinds={["link"]}
      selectedId={props.selectedId ?? null}
      focusedId={props.focusedId ?? props.selectedId ?? null}
      selectedTitle={null}
      clusters={[]}
      onTools={vi.fn()}
      onFit={vi.fn()}
      onClear={onClear}
      onOpen={vi.fn()}
    />,
  );
  return onClear;
}

afterEach(cleanup);

describe("GraphToolsPanel", () => {
  it("keeps Clear enabled while a selection/hover focus exists", () => {
    renderPanel({ selectedId: "a", focusedId: "a" });
    expect(screen.getByRole("button", { name: "Clear" })).toHaveProperty("disabled", false);
    expect(screen.getByRole("button", { name: "Clear" }).getAttribute("title")).toBe(
      "Clear selection and re-fit",
    );
  });

  it("enables Clear for a hover-only focus", () => {
    renderPanel({ focusedId: "b" });
    expect(screen.getByRole("button", { name: "Clear" })).toHaveProperty("disabled", false);
  });

  it("disables Clear only when no focus exists", () => {
    renderPanel({});
    expect(screen.getByRole("button", { name: "Clear" })).toHaveProperty("disabled", true);
  });

  it("renders the Labels switch and the relation/edge multi-selects", () => {
    renderPanel({});
    expect(screen.getByRole("switch", { name: /Labels/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Visible relations/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Visible edge kinds/ })).toBeTruthy();
  });

  it("invokes onClear from the button", async () => {
    const onClear = renderPanel({ focusedId: "a" });
    await userEvent.click(screen.getByRole("button", { name: "Clear" }));
    expect(onClear).toHaveBeenCalledTimes(1);
  });
});
