// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentThread } from "../lib/agent";
import { PetStrip } from "./PetStrip";

/**
 * The WebGL half is not testable in jsdom (there is no context), so the vendored canvas is
 * replaced by a recorder: the test asserts what the strip asks of it — state, still, paused,
 * pulse — not the pixels it would paint.
 */
const recorder = vi.hoisted(() => ({
  states: [] as (string | undefined)[],
  still: [] as boolean[],
  paused: [] as boolean[],
  variants: [] as (string | undefined)[],
  pulses: [] as number[],
  unavailable: undefined as (() => void) | undefined,
}));

vi.mock("../lib/pet", async () => {
  const React = await import("react");
  const PetCanvas = React.forwardRef(function FakePet(
    props: {
      variant?: string;
      state?: string;
      still?: boolean;
      paused?: boolean;
      onUnavailable?: () => void;
    },
    ref: React.Ref<unknown>,
  ) {
    recorder.states.push(props.state);
    recorder.still.push(Boolean(props.still));
    recorder.paused.push(Boolean(props.paused));
    recorder.variants.push(props.variant);
    recorder.unavailable = props.onUnavailable;
    React.useImperativeHandle(ref, () => ({
      state: props.state ?? "idle",
      setState: () => {},
      pulse: (amount = 0.6) => recorder.pulses.push(amount),
      lookAt: () => {},
    }));
    return React.createElement("div", {
      "data-testid": "fake-pet",
      "data-state": props.state,
      "data-still": String(Boolean(props.still)),
      "data-paused": String(Boolean(props.paused)),
      "data-variant": props.variant,
    });
  });
  return { PetCanvas };
});

function thread(parts: AgentThread["messages"][number]["parts"], running: boolean): AgentThread {
  return {
    sessionId: "sess-1",
    running,
    messages: [{ id: "a", role: "assistant", parts }],
  };
}

beforeEach(() => {
  recorder.states = [];
  recorder.still = [];
  recorder.paused = [];
  recorder.variants = [];
  recorder.pulses = [];
  recorder.unavailable = undefined;
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("PetStrip", () => {
  it("paints the band as decorative and shows the work the turn is doing", () => {
    const { container } = render(
      <PetStrip
        phase="active"
        thread={thread([{ kind: "thought", text: "hm" }], true)}
        variant="dot"
      />,
    );
    const band = container.querySelector(".agent-panel__pet");
    expect(band?.getAttribute("aria-hidden")).toBe("true");
    expect(screen.getByTestId("fake-pet").getAttribute("data-state")).toBe("thinking");
    expect(screen.getByTestId("fake-pet").getAttribute("data-variant")).toBe("dot");
  });

  it("shows a running turn moving and a resting one still", () => {
    const running = render(
      <PetStrip
        phase="active"
        thread={thread([{ kind: "text", text: "an answer" }], true)}
        variant="dot"
      />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-state")).toBe("speaking");
    expect(screen.getByTestId("fake-pet").getAttribute("data-still")).toBe("false");
    running.unmount();

    render(
      <PetStrip
        phase="active"
        thread={thread([{ kind: "text", text: "an answer" }], false)}
        variant="dot"
      />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-state")).toBe("idle");
    expect(screen.getByTestId("fake-pet").getAttribute("data-still")).toBe("true");
  });

  it("pulses when the answer grows during a turn", () => {
    const parts = { kind: "text", text: "an" } as const;
    const { rerender } = render(
      <PetStrip phase="active" thread={thread([parts], true)} variant="dot" />,
    );
    recorder.pulses = [];
    rerender(
      <PetStrip
        phase="active"
        thread={thread([{ kind: "text", text: "an answer" }], true)}
        variant="dot"
      />,
    );
    expect(recorder.pulses).toEqual([0.5]);
  });

  it("pauses the canvas while the document is hidden", () => {
    const hidden = vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    render(
      <PetStrip
        phase="active"
        thread={thread([{ kind: "text", text: "x" }], true)}
        variant="dot"
      />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-paused")).toBe("true");
    hidden.mockRestore();
  });

  it("follows a variant change", () => {
    const { rerender } = render(<PetStrip phase="active" variant="dot" />);
    expect(screen.getByTestId("fake-pet").getAttribute("data-variant")).toBe("dot");
    rerender(<PetStrip phase="active" variant="trama" />);
    expect(screen.getByTestId("fake-pet").getAttribute("data-variant")).toBe("trama");
  });

  it("collapses the band when the canvas cannot get a context", () => {
    const { container } = render(<PetStrip phase="active" variant="dot" />);
    expect(container.querySelector(".agent-panel__pet")).not.toBeNull();
    act(() => recorder.unavailable?.());
    expect(screen.queryByTestId("fake-pet")).toBeNull();
    expect(container.querySelector(".agent-panel__pet")).toBeNull();
  });
});
