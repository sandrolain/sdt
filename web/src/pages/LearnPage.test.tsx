// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { LearnPage } from "./LearnPage";

const MATERIAL = {
  path: "context/renders/materials/s/x.it.md",
  title: "X — guida",
  lang: "it",
};

function mockFetch() {
  globalThis.fetch = vi.fn((url: string) => {
    if (url.startsWith("/api/learn/lesson")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            concept: "architecture/x",
            title: "X",
            objectives: ["one"],
            worked_example: { text: "example" },
            material: MATERIAL,
          }),
      });
    }
    if (url.startsWith("/api/learn/course")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            id: "syllabi/s",
            title: "S",
            members: [{ order: 1, id: "themes/t", title: "T", members: ["architecture/x"] }],
            materials: { "architecture/x": MATERIAL },
          }),
      });
    }
    return Promise.resolve({ ok: false, json: () => Promise.resolve({}) });
  }) as unknown as typeof fetch;
}

function renderAt(path: string, mode: "course" | "lesson") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path={`/learn/${mode}/*`} element={<LearnPage mode={mode} />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockFetch();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("LearnPage material links", () => {
  it("links the material from the lesson reader", async () => {
    renderAt("/learn/lesson/architecture/x", "lesson");
    const link = await screen.findByRole("link", { name: "X — guida" });
    expect(link.getAttribute("href")).toBe(`/docs/${MATERIAL.path}`);
  });

  it("links the material beside its concept in the course view", async () => {
    renderAt("/learn/course/syllabi/s", "course");
    const link = await screen.findByRole("link", { name: "X — guida" });
    expect(link.getAttribute("href")).toBe(`/docs/${MATERIAL.path}`);
  });

  it("renders no material section when the bundle carries none", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ concept: "architecture/x", title: "X", objectives: [] }),
      }),
    ) as unknown as typeof fetch;
    renderAt("/learn/lesson/architecture/x", "lesson");
    await screen.findByRole("heading", { name: "X" });
    expect(screen.queryByRole("heading", { name: "Material" })).toBeNull();
  });
});
