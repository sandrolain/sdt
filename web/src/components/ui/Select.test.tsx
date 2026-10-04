// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Select } from "./Select";

const options = [
  { id: "flat", label: "Flat" },
  { id: "type", label: "By type" },
];

afterEach(cleanup);

describe("Select", () => {
  it("renders a visible label without changing the accessible name", () => {
    render(
      <Select
        ariaLabel="Grouping"
        label="Group"
        options={options}
        value="flat"
        onChange={vi.fn()}
      />,
    );
    expect(document.querySelector(".ui-field__label")?.textContent).toBe("Group");
    // the aria-label is what names the control; the visible label is inert
    const button = screen.getByRole("button", { name: /Grouping/ });
    expect(button.getAttribute("aria-label")).toBe("Grouping");
  });

  it("renders no label element when the prop is absent", () => {
    render(<Select ariaLabel="Grouping" options={options} value="flat" onChange={vi.fn()} />);
    expect(document.querySelector(".ui-field__label")).toBeNull();
  });
});
