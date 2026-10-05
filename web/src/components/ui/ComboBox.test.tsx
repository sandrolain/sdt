// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ComboBox } from "./ComboBox";

const options = [
  { id: "caveman-communication", label: "Caveman communication" },
  { id: "caveman-ecosystem", label: "Caveman ecosystem" },
  { id: "sdt-dev-lifecycle", label: "SDT dev lifecycle" },
];

afterEach(cleanup);

describe("ComboBox", () => {
  it("names the input with the aria label", () => {
    render(<ComboBox ariaLabel="Path from" options={options} />);
    expect(screen.getByRole("combobox", { name: "Path from" })).toBeTruthy();
  });

  it("filters the options by the typed text", async () => {
    render(<ComboBox ariaLabel="Path from" options={options} />);
    const input = screen.getByRole("combobox", { name: "Path from" });
    await userEvent.click(input);
    await userEvent.keyboard("cav");
    expect(await screen.findByRole("option", { name: "Caveman communication" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Caveman ecosystem" })).toBeTruthy();
    expect(screen.queryByRole("option", { name: "SDT dev lifecycle" })).toBeNull();
  });

  it("reports the selected option", async () => {
    const onSelectionChange = vi.fn();
    render(
      <ComboBox ariaLabel="Path from" options={options} onSelectionChange={onSelectionChange} />,
    );
    const input = screen.getByRole("combobox", { name: "Path from" });
    await userEvent.click(input);
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.click(await screen.findByRole("option", { name: "Caveman ecosystem" }));
    expect(onSelectionChange).toHaveBeenCalledWith("caveman-ecosystem");
  });

  it("selects the first match with the keyboard", async () => {
    const onSelectionChange = vi.fn();
    render(
      <ComboBox ariaLabel="Path from" options={options} onSelectionChange={onSelectionChange} />,
    );
    const input = screen.getByRole("combobox", { name: "Path from" });
    await userEvent.click(input);
    await userEvent.keyboard("cav");
    await userEvent.keyboard("{ArrowDown}{Enter}");
    expect(onSelectionChange).toHaveBeenCalled();
    expect(String(onSelectionChange.mock.calls.at(-1)?.[0])).toMatch(/^caveman-/);
  });

  it("renders a visible label without changing the accessible name", () => {
    render(<ComboBox ariaLabel="Path from" label="From" options={options} />);
    expect(document.querySelector(".ui-field__label")?.textContent).toBe("From");
    expect(screen.getByRole("combobox", { name: "Path from" })).toBeTruthy();
  });
});
