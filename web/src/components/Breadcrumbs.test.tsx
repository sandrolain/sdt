// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Breadcrumbs } from "./Breadcrumbs";

afterEach(cleanup);

function renderCrumbs(path: string, title?: string) {
  render(
    <MemoryRouter>
      <Breadcrumbs path={path} title={title} />
    </MemoryRouter>,
  );
}

describe("Breadcrumbs", () => {
  it("renders root, folders and the document title", () => {
    renderCrumbs("context/wiki/backend/auth.md", "Auth wiki page");
    expect(screen.getByRole("link", { name: /Corpus/ }).getAttribute("href")).toBe("/docs");
    expect(screen.getByText("wiki")).toBeTruthy();
    expect(screen.getByText("backend")).toBeTruthy();
    expect(screen.getByText("Auth wiki page")).toBeTruthy();
  });

  it("derives the leaf title from the path when none is given", () => {
    renderCrumbs("context/plan/20260915-195559-viewer-fixes.md");
    expect(screen.getByText("Viewer fixes")).toBeTruthy();
    expect(screen.getByText("plan")).toBeTruthy();
  });

  it("handles a root-level document with no folders", () => {
    renderCrumbs("context/notes.md", "Notes");
    expect(screen.getByText("Notes")).toBeTruthy();
    expect(screen.queryByText("notes")).toBeNull();
  });
});
