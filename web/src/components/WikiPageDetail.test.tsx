// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { WikiPageDetail } from "./WikiPageDetail";

afterEach(cleanup);

describe("WikiPageDetail", () => {
  it("redirects a wiki page to the documents view", () => {
    render(
      <MemoryRouter initialEntries={["/wiki/topic.map"]}>
        <Routes>
          <Route path="/wiki/*" element={<WikiPageDetail />} />
          <Route path="/docs/*" element={<p>docs route</p>} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText("docs route")).toBeTruthy();
  });
});
