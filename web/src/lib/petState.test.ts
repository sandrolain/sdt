import { describe, expect, it } from "vitest";
import type { AgentMessage, AgentThread } from "./agent";
import {
  ERROR_MS,
  MIN_STATE_MS,
  SLEEP_AFTER_MS,
  SUCCESS_MS,
  nextPetState,
  petPulse,
  petSignal,
  petStateFor,
  type PetFrame,
  type PetSignal,
} from "./petState";
import { petTimingFor } from "./petTimings";

/** thread builds the wire shape the panel receives, with only what a test cares about. */
function thread(messages: AgentMessage[], running?: boolean): AgentThread {
  return { sessionId: "sess-1", running, messages };
}

const user = (parts: AgentMessage["parts"], provisional = false): AgentMessage => ({
  id: "u",
  role: "user",
  provisional,
  parts,
});

const assistant = (parts: AgentMessage["parts"]): AgentMessage => ({
  id: "a",
  role: "assistant",
  parts,
});

const text = (value: string) => ({ kind: "text", text: value });
const thought = (value: string) => ({ kind: "thought", text: value });
const tool = (status?: string) => ({ kind: "tool_call", toolCall: { id: "t", status } });
const permission = (decision?: string) => ({
  kind: "permission",
  permission: { requestId: "p", sessionId: "sess-1", decision },
});

describe("petSignal", () => {
  it("reads the turn, the answering part and the answer's length", () => {
    const signal = petSignal(
      "active",
      thread([user([text("go")], true), assistant([thought("hm"), tool("in_progress")])], true),
      true,
    );
    expect(signal.running).toBe(true);
    expect(signal.lastKind).toBe("tool_call");
    expect(signal.toolStatus).toBe("in_progress");
    expect(signal.awaiting).toBe(false);
    expect(signal.textLength).toBe(0);
  });

  it("counts the answer's text, not the user's words", () => {
    const signal = petSignal(
      "active",
      thread([user([text("a long question")]), assistant([text("an "), text("answer")])], true),
      true,
    );
    expect(signal.textLength).toBe("an answer".length);
  });

  it("sees an unanswered card, and the answer that resolves it", () => {
    const waiting = petSignal("active", thread([assistant([permission()])], true), true);
    expect(waiting.awaiting).toBe(true);
    const answered = petSignal("active", thread([assistant([permission("allowed")])], true), true);
    expect(answered.awaiting).toBe(false);
  });

  it("reads a failed turn from the error part on the message that was sent", () => {
    const failed = petSignal(
      "active",
      thread([user([text("go"), { kind: "error", text: "no credits" }])]),
      false,
    );
    expect(failed.failed).toBe(true);
    const fine = petSignal("active", thread([user([text("go")]), assistant([text("ok")])]), false);
    expect(fine.failed).toBe(false);
  });

  it("is empty, not broken, for a thread that has not arrived", () => {
    const signal = petSignal("starting", undefined, false);
    expect(signal.lastKind).toBeNull();
    expect(signal.textLength).toBe(0);
    expect(signal.failed).toBe(false);
  });
});

describe("petStateFor", () => {
  const signal = (overrides: Partial<PetSignal> = {}): PetSignal => ({
    phase: "active",
    running: false,
    lastKind: null,
    awaiting: false,
    failed: false,
    textLength: 0,
    ...overrides,
  });

  it("shows the work a running turn is doing", () => {
    expect(petStateFor(signal({ running: true }), null)).toBe("thinking");
    expect(petStateFor(signal({ running: true, lastKind: "thought" }), null)).toBe("thinking");
    expect(
      petStateFor(
        signal({ running: true, lastKind: "tool_call", toolStatus: "in_progress" }),
        null,
      ),
    ).toBe("working");
    expect(petStateFor(signal({ running: true, lastKind: "text" }), null)).toBe("speaking");
  });

  it("listens while a card waits for the reader", () => {
    expect(
      petStateFor(signal({ running: true, lastKind: "permission", awaiting: true }), null),
    ).toBe("listening");
    // An answered card is behind the answer that follows it.
    expect(
      petStateFor(signal({ running: true, lastKind: "tool_call", toolStatus: "completed" }), null),
    ).toBe("thinking");
  });

  it("celebrates or grieves only on the falling edge of a turn", () => {
    const running = signal({ running: true });
    expect(petStateFor(signal({ running: false }), running)).toBe("success");
    expect(petStateFor(signal({ running: false, failed: true }), running)).toBe("error");
    // A thread that has been idle all along is not a turn that just ended.
    expect(petStateFor(signal({ running: false }), signal({ running: false }))).toBe("idle");
  });

  it("shows the panel's own phase when no turn is running", () => {
    expect(petStateFor(signal({ phase: "starting" }), null)).toBe("thinking");
    expect(petStateFor(signal({ phase: "restarting" }), null)).toBe("thinking");
    expect(petStateFor(signal({ phase: "loading" }), null)).toBe("thinking");
    expect(petStateFor(signal({ phase: "stopped" }), null)).toBe("sleeping");
    expect(petStateFor(signal({ phase: "error" }), null)).toBe("error");
    expect(petStateFor(signal({ phase: "active" }), null)).toBe("idle");
  });
});

describe("nextPetState", () => {
  const active = (overrides: Partial<PetSignal> = {}): PetSignal => ({
    phase: "active",
    running: true,
    lastKind: null,
    awaiting: false,
    failed: false,
    textLength: 0,
    ...overrides,
  });

  it("starts on the instantaneous state", () => {
    const frame = nextPetState(null, active(), 1_000);
    expect(frame.state).toBe("thinking");
    expect(frame.at).toBe(1_000);
    expect(frame.idleSince).toBe(0);
  });

  it("holds an ending for the variant's own timing", () => {
    const signal = active({ running: false });
    const frame: PetFrame = { signal, state: "success", at: 1_000, idleSince: 0 };
    // 2 s after the ending, the WebGL default (1.8 s) would have expired; Scriptor's 2.6 s
    // has not.
    expect(nextPetState(frame, signal, 3_000, petTimingFor("scriptor")).state).toBe("success");
    expect(nextPetState(frame, signal, 3_000, petTimingFor("dot")).state).toBe("idle");
    expect(nextPetState(frame, signal, 3_600, petTimingFor("scriptor")).state).toBe("idle");
  });

  it("holds a change for the flicker guard", () => {
    const first = nextPetState(null, active(), 0);
    const working = nextPetState(first, active({ lastKind: "text" }), 100);
    expect(working.state).toBe("thinking");
    expect(working.at).toBe(0);
    const late = nextPetState(working, active({ lastKind: "text" }), MIN_STATE_MS);
    expect(late.state).toBe("speaking");
    expect(late.at).toBe(MIN_STATE_MS);
  });

  it("applies an ending immediately and holds it for the library's duration", () => {
    const working = nextPetState(null, active({ lastKind: "text" }), 0);
    const ended = nextPetState(working, active({ running: false }), 10);
    expect(ended.state).toBe("success");
    expect(ended.at).toBe(10);

    const stillEnding = nextPetState(ended, active({ running: false }), 10 + SUCCESS_MS - 1);
    expect(stillEnding.state).toBe("success");
    const idle = nextPetState(stillEnding, active({ running: false }), 10 + SUCCESS_MS);
    expect(idle.state).toBe("idle");
  });

  it("holds a failure for its own duration", () => {
    const ended = nextPetState(
      nextPetState(null, active(), 0),
      active({ running: false, failed: true }),
      0,
    );
    expect(ended.state).toBe("error");
    expect(nextPetState(ended, active({ running: false, failed: true }), ERROR_MS - 1).state).toBe(
      "error",
    );
    expect(nextPetState(ended, active({ running: false, failed: true }), ERROR_MS).state).toBe(
      "idle",
    );
  });

  it("sleeps only after its rest, and wakes on the next signal", () => {
    const idle = nextPetState(null, active({ running: false }), 0);
    expect(idle.state).toBe("idle");
    expect(idle.idleSince).toBe(0);
    expect(nextPetState(idle, active({ running: false }), SLEEP_AFTER_MS - 1).state).toBe("idle");
    const asleep = nextPetState(idle, active({ running: false }), SLEEP_AFTER_MS);
    expect(asleep.state).toBe("sleeping");
    // The rest timer is not restarted by the frame that fell asleep.
    expect(asleep.idleSince).toBe(0);
    const awake = nextPetState(
      asleep,
      active({ lastKind: "thought" }),
      SLEEP_AFTER_MS + MIN_STATE_MS,
    );
    expect(awake.state).toBe("thinking");
  });

  it("takes its timing from the caller", () => {
    const first = nextPetState(null, active(), 0, { minMs: 1_000 });
    const early = nextPetState(first, active({ lastKind: "text" }), 999, { minMs: 1_000 });
    expect(early.state).toBe("thinking");
    expect(nextPetState(early, active({ lastKind: "text" }), 1_000, { minMs: 1_000 }).state).toBe(
      "speaking",
    );
  });
});

describe("petPulse", () => {
  const signal = (textLength: number, running = true): PetSignal => ({
    phase: "active",
    running,
    lastKind: "text",
    awaiting: false,
    failed: false,
    textLength,
  });

  it("fires only when an answer grew during a turn", () => {
    expect(petPulse(signal(10), signal(20))).toBe(0.5);
    expect(petPulse(signal(20), signal(20))).toBe(0);
    expect(petPulse(signal(10), signal(5))).toBe(0);
    expect(petPulse(null, signal(20))).toBe(0);
    expect(petPulse(signal(10, false), signal(20, false))).toBe(0);
  });
});

/** The frame type is exported for the strip's own clock; this keeps it from going stale. */
const frame: PetFrame | null = null;
void frame;
