// @vitest-environment jsdom
import { useState } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { KindFilter } from "./KindFilter";
import { kindLabel } from "../lib/kinds";
import { kindSelectionLabel } from "../lib/kindFilter";

afterEach(cleanup);

function Harness() {
  const [selection, setSelection] = useState<Set<string>>(new Set());
  return <KindFilter selection={selection} onChange={setSelection} />;
}

describe("KindFilter", () => {
  it("labels the trigger with the selection", () => {
    expect(kindSelectionLabel(new Set())).toBe("All kinds");
    expect(kindSelectionLabel(new Set(["notes"]))).toBe(kindLabel("notes"));
    expect(kindSelectionLabel(new Set(["notes", "decision"]))).toBe("2 kinds");
  });

  it("selects several kinds at once and clears them in one press", async () => {
    render(<Harness />);
    const trigger = () => screen.getByRole("button", { name: "Filter by kind" }).textContent;
    expect(trigger()).toContain("All kinds");

    await userEvent.click(screen.getByRole("button", { name: "Filter by kind" }));
    await userEvent.click(screen.getByRole("checkbox", { name: kindLabel("notes") }));
    // The popover stays open while the selection grows: two kinds at once is the
    // whole point of the control, and closing on the first press would defeat it.
    await userEvent.click(screen.getByRole("checkbox", { name: kindLabel("decision") }));
    expect(
      (screen.getByRole("checkbox", { name: kindLabel("notes") }) as HTMLInputElement).checked,
    ).toBe(true);
    expect(
      (screen.getByRole("checkbox", { name: kindLabel("decision") }) as HTMLInputElement).checked,
    ).toBe(true);

    // The trigger is outside the accessible tree while the dialog is open (react-aria
    // marks the rest aria-hidden), so its label is read after closing.
    await userEvent.keyboard("{Escape}");
    expect(trigger()).toContain("2 kinds");

    await userEvent.click(screen.getByRole("button", { name: "Filter by kind" }));
    await userEvent.click(screen.getByRole("button", { name: "All" }));
    await userEvent.keyboard("{Escape}");
    expect(trigger()).toContain("All kinds");
  });
});
