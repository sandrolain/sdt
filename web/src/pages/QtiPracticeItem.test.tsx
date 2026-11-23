// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QtiPracticeList } from "./QtiPracticeItem";

vi.mock("@longsightgroup/qti3-player-react", async () => {
  const React = await import("react");
  const Player = React.forwardRef((_props: unknown, ref: React.ForwardedRef<unknown>) => {
    React.useImperativeHandle(ref, () => ({
      scoreAttempt: () => ({ outcomes: { SCORE: 1 }, state: { responses: { RESPONSE: ["yes"] } } }),
    }));
    return React.createElement("div", { "data-testid": "qti-player" }, "QTI choices");
  });
  return { QtiAssessmentItemPlayer: Player };
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("QtiPracticeItem", () => {
  it("keeps recording behind an explicit action and sends the QTI response", async () => {
    const fetchMock = vi.fn((url: string, _init?: RequestInit) => {
      if (url.startsWith("/api/learn/qti")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ items: { "item-1": "<qti/>" } }),
        });
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ ok: true, grade: "correct" }),
      });
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    render(
      <QtiPracticeList
        lesson={{ concept: "architecture/x", items: [] }}
        items={[{ id: "item-1", objective: 1, type: "apply", prompt: "Choose one" }]}
      />,
    );

    expect(await screen.findByTestId("qti-player")).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Check answer" }));
    expect(await screen.findByRole("button", { name: "Record attempt" })).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Record attempt" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const request = fetchMock.mock.calls[1]?.[1];
    expect(request?.method).toBe("POST");
    expect(JSON.parse(String(request?.body))).toEqual({
      concept: "architecture/x",
      item: "item-1",
      responses: { RESPONSE: ["yes"] },
    });
    expect(await screen.findByText("Recorded by SDT as correct.")).toBeTruthy();
  });
});
