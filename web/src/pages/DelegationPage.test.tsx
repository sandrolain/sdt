// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { DelegationPage } from "./DelegationPage";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function renderDelegationTask(phase?: string) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        tasks: [
          {
            id: "task-1",
            status: "running",
            role: "backend",
            created_at: "2026-11-19T10:00:00Z",
            plan: "context/plan/example.md",
            phase,
            result_ready: false,
            next_action: "Wait for the result",
          },
        ],
      }),
    }),
  );
  render(
    <MemoryRouter>
      <DelegationPage />
    </MemoryRouter>,
  );
  return screen.findByText("Plan scope");
}

describe("DelegationPage plan scope", () => {
  it("shows an explicit whole-plan scope when no phase is linked", async () => {
    const label = await renderDelegationTask();
    expect(label.parentElement?.querySelector("dd")?.textContent).toContain("Entire plan");
  });

  it("preserves the linked phase scope", async () => {
    const label = await renderDelegationTask("c2");
    expect(label.parentElement?.querySelector("dd")?.textContent).toContain("c2");
  });
});
