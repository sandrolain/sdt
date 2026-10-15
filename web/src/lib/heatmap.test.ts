import { describe, expect, it } from "vitest";
import { HEATMAP_WINDOW_WEEKS, heatmapLayout, heatmapOpacity, monthLabelOf } from "./heatmap";
import type { TreeEntry } from "./api";

const doc = (title: string): TreeEntry =>
  ({ path: `context/notes/${title}.md`, kind: "notes", title }) as TreeEntry;

function byDay(entries: Record<string, TreeEntry[]>): Map<string, TreeEntry[]> {
  return new Map(Object.entries(entries));
}

describe("heatmapLayout", () => {
  it("answers an empty grid for an empty corpus", () => {
    const grid = heatmapLayout(new Map());
    expect([grid.columns, grid.capped, grid.max]).toEqual([[], false, 0]);
  });

  it("puts the first day on its own weekday, Monday first", () => {
    // 2026-09-16 is a Wednesday.
    const grid = heatmapLayout(byDay({ "2026-09-16": [doc("a")] }));
    expect(grid.columns.length).toBe(1);
    const cells = grid.columns[0].cells;
    expect(cells[2]?.day).toBe("2026-09-16");
    expect(cells[0]).toBeNull();
    expect(cells[1]).toBeNull();
    expect(cells.slice(3).every((cell) => cell === null)).toBe(true);
  });

  it("starts a new column every Monday and labels the month when it changes", () => {
    const grid = heatmapLayout(
      byDay({
        "2026-09-28": [doc("a")], // a Monday
        "2026-09-30": [doc("b"), doc("c")],
        "2026-10-01": [doc("d")], // Thursday of the same week
        "2026-10-05": [doc("e")], // the next Monday: October's first column
      }),
    );
    expect(grid.columns.length).toBe(2);
    expect(grid.columns[0].monthLabel).toBe("Sep");
    expect(grid.columns[0].cells[0]?.count).toBe(1);
    expect(grid.columns[0].cells[2]?.count).toBe(2);
    expect(grid.columns[0].cells[3]?.count).toBe(1);
    expect(grid.columns[1].monthLabel).toBe("Oct");
    expect(grid.max).toBe(2);
  });

  it("caps a span longer than the window and says so", () => {
    const grid = heatmapLayout(
      byDay({
        "2020-01-06": [doc("old")],
        "2026-10-05": [doc("new")],
      }),
    );
    expect(grid.capped).toBe(true);
    expect(grid.first).not.toBe("2020-01-06");
    expect(grid.columns.length).toBeLessThanOrEqual(HEATMAP_WINDOW_WEEKS);
    // The recent document is in the window; the old one is not.
    const days = grid.columns.flatMap((c) => c.cells).map((c) => c?.day);
    expect(days).toContain("2026-10-05");
    expect(days).not.toContain("2020-01-06");
  });

  it("is deterministic and names the day's first documents", () => {
    const input = byDay({ "2026-10-05": [doc("a"), doc("b"), doc("c"), doc("d")] });
    const first = heatmapLayout(input);
    expect(heatmapLayout(input)).toEqual(first);
    const cell = first.columns[0].cells[0]!;
    expect(cell.day).toBe("2026-10-05");
    expect(cell.count).toBe(4);
    expect(cell.titles).toEqual(["a", "b", "c"]);
  });

  it("keeps the intensity on one scale, with an empty day dimmer than any", () => {
    expect(heatmapOpacity(0, 4)).toBeLessThan(heatmapOpacity(1, 4));
    expect(heatmapOpacity(1, 4)).toBeGreaterThan(0.25);
    expect(heatmapOpacity(4, 4)).toBe(1);
    expect(monthLabelOf("2026-01-15")).toBe("Jan");
  });
});
