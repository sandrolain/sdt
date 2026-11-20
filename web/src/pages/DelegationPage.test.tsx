// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { DelegationPage } from "./DelegationPage";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function renderDelegationTask(phase?: string, inbox: unknown[] = []) {
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
        inbox,
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

  it("shows grouped pending decisions and selects the associated task", async () => {
    await renderDelegationTask(undefined, [
      {
        task_id: "task-1",
        status: "intervention_required",
        revision: 4,
        updated_at: "2026-11-19T10:00:00Z",
        kinds: ["intervention", "permission"],
        pending_request_ids: ["request-a", "request-b"],
        next_action: "Review each pending permission and resolve it through the CLI",
      },
    ]);
    expect(screen.getByRole("heading", { name: "Principal inbox" })).toBeTruthy();
    const item = screen.getByRole("button", { name: /2 pending permission requests/ });
    fireEvent.click(item);
    expect(item.getAttribute("aria-pressed")).toBe("true");
    expect(item.textContent).toContain("request-a, request-b");
  });
});
