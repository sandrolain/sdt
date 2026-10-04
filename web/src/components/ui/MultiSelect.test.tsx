// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MultiSelect, type UiOptionSection } from "./MultiSelect";

const options = [
  { id: "completed", label: "Completed" },
  { id: "draft", label: "Draft" },
];

/** The trigger value: the hidden native select mirrors the same option labels,
 *  so the summary is read from the trigger rather than from the document. */
const trigger = () => screen.getByRole("button", { name: "Visible states" });
const triggerText = () => trigger().querySelector(".ui-select__value")?.textContent;

const sections: UiOptionSection[] = [
  { id: "open", label: "Open", options: [{ id: "draft", label: "Draft" }] },
  { id: "concluded", label: "Concluded", options: [{ id: "completed", label: "Completed" }] },
];

afterEach(cleanup);

describe("MultiSelect", () => {
  it("summarises the selection and reports every change", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <MultiSelect
        ariaLabel="Visible states"
        options={options}
        selected={[]}
        onChange={onChange}
      />,
    );
    // empty selection falls back to the placeholder
    expect(triggerText()).toBe("Any");

    rerender(
      <MultiSelect
        ariaLabel="Visible states"
        options={options}
        selected={["completed"]}
        onChange={onChange}
      />,
    );
    expect(triggerText()).toBe("Completed");

    rerender(
      <MultiSelect
        ariaLabel="Visible states"
        options={options}
        selected={["completed", "draft"]}
        onChange={onChange}
        allLabel="All"
      />,
    );
    // everything selected reads as the all label, not as a count
    expect(triggerText()).toBe("All");

    await userEvent.click(trigger());
    // clicking a selected option deselects it: the payload is what stays selected
    await userEvent.click(await screen.findByRole("option", { name: "Draft" }));
    expect(onChange).toHaveBeenCalled();
    expect(onChange.mock.calls[0][0]).toEqual(["completed"]);
  });

  it("renders the options in labelled groups and uses emptyLabel when nothing is selected", async () => {
    render(
      <MultiSelect
        ariaLabel="Visible states"
        sections={sections}
        selected={[]}
        onChange={() => {}}
        placeholder="All"
        emptyLabel="None"
      />,
    );
    expect(triggerText()).toBe("None");

    await userEvent.click(trigger());
    expect(await screen.findByRole("option", { name: "Completed" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Draft" })).toBeTruthy();
    // the group headings are rendered, in order
    expect(screen.getByText("Open")).toBeTruthy();
    expect(screen.getByText("Concluded")).toBeTruthy();
  });

  it("applies a preset and toggles a whole section", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <MultiSelect
        ariaLabel="Visible states"
        sections={sections}
        presets={[{ id: "all", label: "All", ids: ["draft", "completed"] }]}
        selected={[]}
        onChange={onChange}
      />,
    );
    await userEvent.click(trigger());
    await userEvent.click(await screen.findByRole("button", { name: "All" }));
    expect(onChange).toHaveBeenLastCalledWith(["draft", "completed"]);

    onChange.mockClear();
    await userEvent.click(screen.getByRole("button", { name: "Select all Open" }));
    expect(onChange).toHaveBeenLastCalledWith(["draft"]);

    // every option of the section already selected: the action clears it
    onChange.mockClear();
    rerender(
      <MultiSelect
        ariaLabel="Visible states"
        sections={sections}
        presets={[{ id: "all", label: "All", ids: ["draft", "completed"] }]}
        selected={["draft", "completed"]}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Clear Open" }));
    expect(onChange).toHaveBeenLastCalledWith(["completed"]);
  });

  it("marks a selected option with a check glyph", async () => {
    render(
      <MultiSelect
        ariaLabel="Visible states"
        sections={sections}
        selected={["completed"]}
        onChange={() => {}}
      />,
    );
    await userEvent.click(trigger());
    const selected = await screen.findByRole("option", { name: "Completed" });
    expect(selected.querySelector(".ui-select__item-check")).toBeTruthy();
    const unselected = screen.getByRole("option", { name: "Draft" });
    expect(unselected.querySelector(".ui-select__item-check")).toBeNull();
  });

  it("renders a visible label without changing the accessible name", () => {
    render(
      <MultiSelect
        ariaLabel="Visible states"
        label="States"
        options={options}
        selected={[]}
        onChange={() => {}}
      />,
    );
    expect(document.querySelector(".ui-field__label")?.textContent).toBe("States");
    expect(trigger()).toBeTruthy();
  });
});
