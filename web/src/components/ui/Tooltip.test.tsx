// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipButton } from "./Tooltip";

afterEach(cleanup);

describe("TooltipButton", () => {
  it("exposes the accessible name and fires onPress", async () => {
    const onPress = vi.fn();
    render(
      <TooltipButton label="Collapse panel" tooltip="Collapse panel" onPress={onPress}>
        <span>icon</span>
      </TooltipButton>,
    );
    const button = screen.getByRole("button", { name: "Collapse panel" });
    await userEvent.click(button);
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it("shows the tooltip on hover", async () => {
    render(
      <TooltipButton label="Reset layout" tooltip="Reset layout">
        <span>icon</span>
      </TooltipButton>,
    );
    await userEvent.hover(screen.getByRole("button", { name: "Reset layout" }));
    expect(await screen.findByText("Reset layout")).toBeTruthy();
  });
});
