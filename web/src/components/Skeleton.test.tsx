// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { SkeletonLines } from "./Skeleton";

afterEach(cleanup);

describe("SkeletonLines", () => {
  it("renders the requested number of shimmer lines with a status label", () => {
    render(<SkeletonLines count={3} label="Loading tree" />);
    const status = screen.getByRole("status", { name: "Loading tree" });
    expect(status.querySelectorAll(".skeleton__line")).toHaveLength(3);
  });

  it("defaults to four lines", () => {
    render(<SkeletonLines />);
    expect(screen.getByRole("status").querySelectorAll(".skeleton__line")).toHaveLength(4);
  });
});
