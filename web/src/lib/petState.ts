/**
 * The pet's state, derived from the panel's own truth.
 *
 * The panel holds a whole-thread snapshot and a phase, not an event stream: a
 * state→state mapper is the honest shape (the library's event adapter expects `request`,
 * `tool_call`, `token`, `done`). Everything here is pure and time is a parameter, so the
 * behaviour is tested without a clock, a canvas or a browser.
 *
 * Two rules matter more than the rest:
 *
 * - `success` comes only from the falling edge of `running`, the thread's own turn fact.
 *   Nothing here invents a turn's end from silence: a timer that guessed would celebrate a
 *   turn that is still running, which this project has already decided against.
 * - An ending is held for the library's own durations, so a still pet (reduced motion) sees
 *   the same celebration a moving one does.
 */
import type { AgentThread } from "./agent";
import type { PetState } from "./pet/types";
import { useEffect, useRef, useState } from "react";

/** The panel's phase, structurally: `AgentPanel` owns the vocabulary. */
export type PetPhase = "loading" | "starting" | "active" | "restarting" | "stopped" | "error";

/** The library's flicker guard: a turn's states change no faster than this. */
export const MIN_STATE_MS = 350;
/** Its rest timer: an idle pet falls asleep after this long. */
export const SLEEP_AFTER_MS = 60_000;
/** The library's one-shot durations for the two endings. */
export const SUCCESS_MS = 1_800;
export const ERROR_MS = 1_300;

/** What the pet reacts to, reduced from the phase and the thread. */
export interface PetSignal {
  phase: PetPhase;
  /** A turn is in flight, from the thread's own field. */
  running: boolean;
  /** The last part of the last assistant message, or null when the agent said nothing. */
  lastKind: string | null;
  /** That part's tool status, when it is a tool call. */
  toolStatus?: string;
  /** The last part is a permission card nobody has answered yet. */
  awaiting: boolean;
  /** The last turn ended with an error part. */
  failed: boolean;
  /** The answer's text length: its growth is the pulse. */
  textLength: number;
}

/** One frame of the pet's life: the signal it was derived from and the state it settled on. */
export interface PetFrame {
  signal: PetSignal;
  state: PetState;
  /** When the current state was applied. */
  at: number;
  /** When the pet became idle; the rest timer runs from here. */
  idleSince: number;
}

/** Timing, overridable so a test can drive an ending without waiting for it. */
export interface PetTiming {
  minMs?: number;
  sleepAfterMs?: number;
  successMs?: number;
  errorMs?: number;
}

/**
 * petSignal reduces the panel's state to the few facts the pet reacts to. It reads only
 * what the wire carries: the phase, `running`, and the parts.
 */
export function petSignal(
  phase: PetPhase,
  thread: AgentThread | undefined,
  running: boolean,
): PetSignal {
  const messages = thread?.messages ?? [];
  const assistant = [...messages].reverse().find((message) => message.role === "assistant");
  const last = assistant?.parts?.[assistant.parts.length - 1];
  const lastUser = [...messages].reverse().find((message) => message.role === "user");
  let textLength = 0;
  for (const part of assistant?.parts ?? []) {
    if (part.kind === "text") textLength += part.text?.length ?? 0;
  }
  return {
    phase,
    running,
    lastKind: last?.kind ?? null,
    toolStatus: last?.toolCall?.status,
    awaiting: last?.kind === "permission" && !last.permission?.decision,
    failed: Boolean(lastUser?.parts?.some((part) => part.kind === "error")),
    textLength,
  };
}

/** A tool call in a terminal state is not work in progress. */
function toolFinished(status: string | undefined): boolean {
  return status === "completed" || status === "failed";
}

/**
 * petStateFor is the instantaneous state: what the pet would show if nothing had to hold.
 * `previous` is used for one thing only — the falling edge of a turn, which is the only
 * honest source of a success or a failure.
 */
export function petStateFor(signal: PetSignal, previous: PetSignal | null): PetState {
  if (previous?.running && !signal.running) {
    return signal.failed ? "error" : "success";
  }
  if (signal.running) {
    if (signal.awaiting) return "listening";
    if (signal.lastKind === "tool_call" && !toolFinished(signal.toolStatus)) return "working";
    if (signal.lastKind === "thought") return "thinking";
    if (signal.lastKind === "text") return "speaking";
    return "thinking";
  }
  switch (signal.phase) {
    case "error":
      return "error";
    case "stopped":
      return "sleeping";
    case "loading":
    case "starting":
    case "restarting":
      return "thinking";
    default:
      return "idle";
  }
}

/**
 * nextPetState advances one frame: the instantaneous state, with the library's timing laid
 * over it. A state is held for `MIN_STATE_MS`; the two endings are immediate and then held
 * for their own durations; an idle pet falls asleep once its rest has run out.
 */
export function nextPetState(
  previous: PetFrame | null,
  signal: PetSignal,
  now: number,
  timing: PetTiming = {},
): PetFrame {
  const minMs = timing.minMs ?? MIN_STATE_MS;
  const sleepAfterMs = timing.sleepAfterMs ?? SLEEP_AFTER_MS;
  const target = petStateFor(signal, previous?.signal ?? null);

  if (!previous) {
    return { signal, state: target, at: now, idleSince: target === "idle" ? now : 0 };
  }

  if (target === previous.state) {
    if (previous.state === "idle" && now - previous.idleSince >= sleepAfterMs) {
      return { signal, state: "sleeping", at: now, idleSince: previous.idleSince };
    }
    // Same state, newer signal: the frame keeps its clock and the answer's length follows.
    return { signal, state: previous.state, at: previous.at, idleSince: previous.idleSince };
  }

  const hold =
    previous.state === "success"
      ? (timing.successMs ?? SUCCESS_MS)
      : previous.state === "error"
        ? (timing.errorMs ?? ERROR_MS)
        : minMs;
  // An ending is immediate, like the library's own one-shot states; every other change
  // waits out the flicker guard.
  const immediate = target === "success" || target === "error";
  if (!immediate && now - previous.at < hold) return previous;

  return { signal, state: target, at: now, idleSince: target === "idle" ? now : 0 };
}

/**
 * petPulse is the energy an answer's growth deserves: a token arrived while a turn is in
 * flight. Anything else is silence, so a frame that only repeated itself does not pulse.
 */
export function petPulse(previous: PetSignal | null, signal: PetSignal): number {
  if (!previous?.running || !signal.running) return 0;
  return signal.textLength > previous.textLength ? 0.5 : 0;
}

/**
 * The clock, read only from the hook's effect: the machine takes time as a parameter, so
 * nothing reads "now" during render. A module-scope arrow is the documented escape hatch.
 */
const clock = () => performance.now();

/** dueAt is the next moment the frame could change on its own, or null when the next change
 *  needs a new signal. `held` is a frame the machine refused to leave yet: the flicker guard
 *  is what expires. */
function dueAt(frame: PetFrame, held: boolean, timing: PetTiming = {}): number | null {
  if (held) return frame.at + (timing.minMs ?? MIN_STATE_MS);
  if (frame.state === "success") return frame.at + (timing.successMs ?? SUCCESS_MS);
  if (frame.state === "error") return frame.at + (timing.errorMs ?? ERROR_MS);
  if (frame.state === "idle") return frame.idleSince + (timing.sleepAfterMs ?? SLEEP_AFTER_MS);
  return null;
}

/**
 * stateWord is the state in the reader's words, with the process's own phase winning: a
 * stopped agent is stopped, not asleep. It is the panel's single vocabulary for the state,
 * so the band, the header and the tests cannot drift apart.
 */
export function stateWord(phase: PetPhase, state: PetState): string {
  switch (phase) {
    case "loading":
    case "starting":
      return "starting";
    case "restarting":
      return "restarting";
    case "stopped":
      return "stopped";
    case "error":
      return "failed";
    default:
      break;
  }
  switch (state) {
    case "listening":
      return "waiting for you";
    case "thinking":
      return "thinking";
    case "working":
      return "running a command";
    case "speaking":
      return "answering";
    case "success":
      return "done";
    case "error":
      return "failed";
    case "sleeping":
      return "asleep";
    default:
      return "idle";
  }
}

/**
 * usePetState runs the machine for one signal and hands back the state to render.
 *
 * It lives here rather than in `PetStrip` because the panel also says the state in words,
 * including while the pet itself is off: two clocks would let the band and the word
 * disagree about whether the agent is working.
 */
export function usePetState(signal: PetSignal, timing: PetTiming = {}): PetState {
  const frame = useRef<PetFrame | null>(null);
  const [state, setState] = useState<PetState>("idle");

  // The machine runs once per signal and schedules the one moment it could move again: a
  // held change, the end of a celebration, or the rest before sleep. Nothing here polls — a
  // state with no pending change sets no timer.
  useEffect(() => {
    let timer = 0;
    const step = () => {
      const now = clock();
      const before = frame.current;
      const next = nextPetState(before, signal, now, timing);
      if (next !== before) {
        frame.current = next;
        setState(next.state);
      }
      window.clearTimeout(timer);
      const due = dueAt(next, next === before, timing);
      if (due !== null) timer = window.setTimeout(step, Math.max(0, due - now));
    };
    step();
    return () => window.clearTimeout(timer);
    // The timing is read field by field: a caller may build the object inline, and an
    // object identity in the dependency list would restart the clock on every render.
  }, [signal, timing.minMs, timing.sleepAfterMs, timing.successMs, timing.errorMs]);

  return state;
}
