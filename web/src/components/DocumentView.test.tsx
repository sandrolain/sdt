// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { DocumentView } from "./DocumentView";

function renderView(props: Partial<Parameters<typeof DocumentView>[0]> & { path: string }) {
  render(
    <MemoryRouter initialEntries={["/docs/x"]}>
      <Routes>
        <Route
          path="/docs/*"
          element={
            <DocumentView
              path={props.path}
              markdown={props.markdown ?? "# Title\n\nHello **tokens**."}
              frontmatter={props.frontmatter ?? "---\nkind: wiki\n---\n"}
              isMap={props.isMap}
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(cleanup);

describe("DocumentView", () => {
  it("defaults ordinary markdown to Render mode", () => {
    renderView({ path: "context/wiki/alpha.md" });
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Title" })).toBeTruthy();
  });

  it("defaults map documents to Map mode with a badge", () => {
    renderView({ path: "context/wiki/topic.map.md", markdown: "# Top\n\n- item\n" });
    expect(screen.getByRole("button", { name: "Map", pressed: true })).toBeTruthy();
    expect(screen.getByText("map")).toBeTruthy();
    expect(screen.getByText("Top")).toBeTruthy();
  });

  it("switches to Code mode showing highlighted raw markdown", async () => {
    renderView({ path: "context/wiki/alpha.md" });
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    expect(screen.getByRole("button", { name: "Code", pressed: true })).toBeTruthy();
    expect(screen.getByText(/Hello/)).toBeTruthy();
  });

  it("shows an empty outline state for plain prose in Map mode", () => {
    renderView({ path: "context/wiki/prose.md", isMap: true, markdown: "just prose" });
    expect(screen.getByText("No headings or lists to map.")).toBeTruthy();
  });
});
