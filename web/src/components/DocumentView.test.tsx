// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { DocumentView } from "./DocumentView";

vi.mock("./MindmapView", () => ({
  MindmapView: ({ title }: { title?: string }) => (
    <div role="img" aria-label={`Mindmap: ${title ?? ""}`} />
  ),
}));

function renderView(
  props: Partial<Parameters<typeof DocumentView>[0]> & { path: string },
  initial = "/docs/x",
) {
  render(
    <MemoryRouter initialEntries={[initial]}>
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

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("DocumentView", () => {
  it("defaults ordinary markdown to Render mode", () => {
    renderView({ path: "context/wiki/alpha.md" });
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
    expect(screen.getByText(/Hello/)).toBeTruthy();
    // the leading `# Title` is the document title, not repeated in the body
    expect(screen.queryByRole("heading", { name: "Title" })).toBeNull();
  });

  it("defaults map documents to Map mode with a map icon and renders the mindmap", async () => {
    renderView({ path: "context/wiki/topic.map.md", markdown: "# Top\n\n- item\n" });
    expect(screen.getByRole("button", { name: "Map", pressed: true })).toBeTruthy();
    expect(screen.getByRole("img", { name: "Map document" })).toBeTruthy();
    expect(await screen.findByRole("img", { name: "Mindmap: topic.map" })).toBeTruthy();
  });

  it("offers Map mode only for .map.md documents", () => {
    renderView({ path: "context/wiki/alpha.md" }, "/docs/x?view=map");
    expect(screen.queryByRole("button", { name: "Map" })).toBeNull();
    expect(screen.getByRole("button", { name: "Render", pressed: true })).toBeTruthy();
  });

  it("copies a rendered code block from its copy button", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    renderView({ path: "context/wiki/alpha.md", markdown: "```js\nconst x = 1;\n```" });
    await userEvent.click(screen.getByRole("button", { name: "Copy code" }));
    expect(writeText).toHaveBeenCalledWith("const x = 1;");
    expect(screen.getByRole("button", { name: "Copied" })).toBeTruthy();
  });

  it("switches to Code mode showing highlighted raw markdown", async () => {
    renderView({ path: "context/wiki/alpha.md" });
    await userEvent.click(screen.getByRole("button", { name: "Code" }));
    expect(screen.getByRole("button", { name: "Code", pressed: true })).toBeTruthy();
    expect(screen.getByText(/Hello/)).toBeTruthy();
  });

  it("renders exactly one pressed mode button (exclusive group)", () => {
    renderView({ path: "context/wiki/alpha.md" });
    expect(screen.getAllByRole("button", { pressed: true })).toHaveLength(1);
  });

  it("renders a mindmap for plain prose in Map mode", async () => {
    renderView({ path: "context/wiki/prose.md", isMap: true, markdown: "just prose" });
    expect(await screen.findByRole("img", { name: "Mindmap: prose" })).toBeTruthy();
  });
});
