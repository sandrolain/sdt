// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentPart, AgentThread } from "../lib/agent";
import { petSignal } from "../lib/petState";
import { PetStrip } from "./PetStrip";

/**
 * The WebGL half is not testable in jsdom (there is no context), so the vendored canvas is
 * replaced by a recorder: the test asserts what the strip asks of it — state, still, paused,
 * pulse — not the pixels it would paint. The machine itself is no longer the strip's: the
 * panel owns `usePetState`, so these tests hand the state in and check that it is applied.
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

function thread(parts: AgentPart[], running: boolean): AgentThread {
  return {
    sessionId: "sess-1",
    running,
    messages: [{ id: "a", role: "assistant", parts }],
  };
}

/** signalOf is the panel's own reduction, so a test hands the strip what the panel would. */
function signalOf(parts: AgentPart[], running: boolean) {
  return petSignal("active", thread(parts, running), running);
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
  it("paints the canvas as decorative and says the state in words", () => {
    const { container } = render(
      <PetStrip
        phase="active"
        signal={signalOf([{ kind: "thought", text: "hm" }], true)}
        state="thinking"
        variant="dot"
      />,
    );
    // The hidden half is the canvas; the band's corner carries the state a reader can read.
    expect(container.querySelector(".agent-panel__pet-stage")?.getAttribute("aria-hidden")).toBe(
      "true",
    );
    expect(screen.getByText("thinking")).toBeTruthy();
    expect(screen.getByTestId("fake-pet").getAttribute("data-state")).toBe("thinking");
    expect(screen.getByTestId("fake-pet").getAttribute("data-variant")).toBe("dot");
  });

  it("keeps a resting pet moving, and stills it only for reduced motion", () => {
    const working = render(
      <PetStrip
        phase="active"
        signal={signalOf([{ kind: "text", text: "an answer" }], true)}
        state="speaking"
        variant="dot"
      />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-still")).toBe("false");
    expect(screen.getByText("answering")).toBeTruthy();
    working.unmount();

    cleanup();
    render(
      <PetStrip
        phase="active"
        signal={signalOf([{ kind: "text", text: "an answer" }], false)}
        state="idle"
        variant="dot"
      />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-still")).toBe("false");
    expect(screen.getByText("idle")).toBeTruthy();

    cleanup();
    vi.stubGlobal("matchMedia", () => ({
      matches: true,
      addEventListener: () => {},
      removeEventListener: () => {},
    }));
    render(<PetStrip phase="active" signal={signalOf([], false)} state="idle" variant="dot" />);
    expect(screen.getByTestId("fake-pet").getAttribute("data-still")).toBe("true");
    vi.unstubAllGlobals();
  });

  it("pulses when the answer grows during a turn", () => {
    const first = signalOf([{ kind: "text", text: "an" }], true);
    const { rerender } = render(
      <PetStrip phase="active" signal={first} state="speaking" variant="dot" />,
    );
    recorder.pulses = [];
    rerender(
      <PetStrip
        phase="active"
        signal={signalOf([{ kind: "text", text: "an answer" }], true)}
        state="speaking"
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
        signal={signalOf([{ kind: "text", text: "x" }], true)}
        state="working"
        variant="dot"
      />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-paused")).toBe("true");
    hidden.mockRestore();
  });

  it("follows a variant change", () => {
    const signal = signalOf([], false);
    const { rerender } = render(
      <PetStrip phase="active" signal={signal} state="idle" variant="dot" />,
    );
    expect(screen.getByTestId("fake-pet").getAttribute("data-variant")).toBe("dot");
    rerender(<PetStrip phase="active" signal={signal} state="idle" variant="trama" />);
    expect(screen.getByTestId("fake-pet").getAttribute("data-variant")).toBe("trama");
  });

  it("keeps the band and its word when the canvas cannot get a context", () => {
    const { container } = render(
      <PetStrip phase="active" signal={signalOf([], false)} state="idle" variant="dot" />,
    );
    expect(container.querySelector(".agent-panel__pet")).not.toBeNull();
    act(() => recorder.unavailable?.());
    // The canvas is gone; the state is not, because it is text in the band.
    expect(screen.queryByTestId("fake-pet")).toBeNull();
    expect(screen.getByText("idle")).toBeTruthy();
  });
});
