// @vitest-environment node
/**
 * The other half of the wire golden: the Go test writes
 * `internal/acpbridge/testdata/wire/*.json`, this one reads them.
 *
 * It is the net that keeps the two sides from drifting: a field renamed in Go fails
 * here, and the browser's own builders are compared *against the fixture* rather than
 * against a hand-written expectation, so an envelope the viewer would refuse cannot be
 * sent by the panel.
 */
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import {
  ACTION_ACCEPT,
  ENVELOPE_CANCEL,
  ENVELOPE_CONFIG,
  ENVELOPE_ELICITATION,
  ENVELOPE_PERMISSION,
  ENVELOPE_PROMPT,
  ENVELOPE_STATE,
  ENVELOPE_STATUS,
  PART_ELICITATION,
  PART_ERROR,
  PART_PERMISSION,
  PART_STOP,
  PART_TEXT,
  PART_THOUGHT,
  PART_TOOL_CALL,
  cancelCommand,
  configCommand,
  elicitationCommand,
  permissionCommand,
  promptCommand,
  type AgentStateEnvelope,
} from "./agent";

const WIRE_DIR = join(process.cwd(), "..", "internal", "acpbridge", "testdata", "wire");

function fixture(...path: string[]): unknown {
  return JSON.parse(readFileSync(join(WIRE_DIR, ...path), "utf8"));
}

describe("the wire the Go side writes", () => {
  it("carries one state envelope per session/update variant, and every part kind it can produce is one the panel renders", () => {
    const files = readdirSync(join(WIRE_DIR, "variants")).sort();
    // The analysis's mapping table has thirteen variants; the fold collapses them into
    // parts, and the permission card is the bridge asking rather than a variant.
    expect(files.length).toBeGreaterThanOrEqual(13);

    const kinds = new Set<string>();
    for (const file of files) {
      const envelope = fixture("variants", file) as AgentStateEnvelope;
      expect(envelope.type).toBe(ENVELOPE_STATE);
      expect(typeof envelope.sessionId).toBe("string");
      expect(Array.isArray(envelope.thread.messages)).toBe(true);
      // A single update is not a turn: only the thread's own running field says one is in
      // flight, and the field is absent rather than false when it is not.
      expect(envelope.thread.running).toBeUndefined();
      for (const message of envelope.thread.messages) {
        for (const part of message.parts) kinds.add(part.kind);
      }
    }

    // A new part kind on the Go side fails here, rather than showing up in the thread
    // as a blank line.
    expect([...kinds].sort()).toEqual(
      [PART_TEXT, PART_THOUGHT, PART_TOOL_CALL, PART_PERMISSION, PART_ELICITATION].sort(),
    );
  });

  it("reads the scripted session with the field names the converter uses", () => {
    const envelope = fixture("session.json") as AgentStateEnvelope;
    expect(envelope.type).toBe(ENVELOPE_STATE);
    expect(envelope.sessionId).toBe("sess-1");
    // The scripted session ends on a turn still in flight: the thread's one turn fact,
    // which the runtime's `isRunning` and the panel's pet both read.
    expect(envelope.thread.running).toBe(true);

    const [user, assistant] = envelope.thread.messages;
    expect(user.role).toBe("user");
    expect(user.parts[0]).toEqual({ kind: PART_TEXT, text: "how big is the corpus?" });

    // The streaming rule, visible in the fixture: the chunks that arrive before the
    // tool call share one text part, and the one after it opens a new part.
    const kinds = assistant.parts.map((part) => part.kind);
    expect(kinds).toEqual([
      PART_TEXT,
      PART_THOUGHT,
      PART_TOOL_CALL,
      PART_TEXT,
      PART_PERMISSION,
      PART_ELICITATION,
      PART_ELICITATION,
    ]);
    expect(assistant.parts[0].text).toBe("Let me look at the notes.");
    expect(assistant.parts[3].text).toBe(" The corpus has 224 analyses.");

    const call = assistant.parts[2].toolCall;
    expect(call).toEqual({
      id: "call-1",
      title: "Read context/notes/a.md",
      kind: "read",
      status: "completed",
      output: "224 analyses",
      content: "224 analyses",
    });

    const permission = assistant.parts[4].permission;
    expect(permission).toMatchObject({
      requestId: "perm-1",
      sessionId: "sess-1",
      toolCallId: "call-2",
      toolKind: "edit",
      title: "Write context/notes/x.md",
      decision: "allowed",
    });

    // The two shapes a question can end in: a form answered with its values, and a url
    // the protocol completed.
    expect(assistant.parts[5].elicitation).toMatchObject({
      requestId: "elicit-1",
      mode: "form",
      decision: "accepted",
      content: { branch: "main" },
    });
    expect(assistant.parts[6].elicitation).toMatchObject({
      requestId: "elicit-2",
      mode: "url",
      url: "https://example.com/auth",
      elicitationId: "e-2",
      completed: true,
    });

    expect(envelope.thread.plan?.entries?.[0]).toEqual({
      content: "read the notes",
      priority: "medium",
      status: "completed",
    });
  });

  it("carries a turn's outcome — the refusal, the explanatory stop and the message still waiting", () => {
    const envelope = fixture("session.json") as AgentStateEnvelope;
    const messages = envelope.thread.messages;
    const [refused, stopped, waiting] = messages.slice(-3);

    // The refusal is the agent's own sentence, on the message that was sent, and that
    // message is no longer waiting: an outcome answers it, badly.
    expect(refused.parts).toEqual([
      { kind: PART_TEXT, text: "how big is the agent bridge?" },
      { kind: PART_ERROR, text: "This request requires more credits, or fewer max_tokens" },
    ]);
    expect(refused.provisional).toBeUndefined();

    // An ending that explains the silence, in the protocol's own vocabulary.
    expect(stopped.parts[stopped.parts.length - 1]).toEqual({
      kind: PART_STOP,
      text: "max_tokens",
    });

    // A turn that has answered nothing yet: the panel's waiting state, and the only
    // message in the fixture that carries the flag.
    expect(waiting.provisional).toBe(true);
    expect(waiting.parts).toEqual([{ kind: PART_TEXT, text: "and the panel?" }]);
  });

  it("sends exactly the envelopes the browser's builders produce", () => {
    expect(promptCommand("how big is the corpus?", "sess-1")).toEqual(
      fixture("commands", "prompt.json"),
    );
    expect(cancelCommand("sess-1")).toEqual(fixture("commands", "cancel.json"));
    expect(permissionCommand("perm-1", true, "sess-1")).toEqual(
      fixture("commands", "permission.json"),
    );
    expect(configCommand("model", "opencode/big-pickle", "sess-1")).toEqual(
      fixture("commands", "config.json"),
    );
    expect(elicitationCommand("elicit-1", ACTION_ACCEPT, { branch: "main" }, "sess-1")).toEqual(
      fixture("commands", "elicitation.json"),
    );

    // Every kind the fixture set uses is one the wire defines, and the set is exactly the
    // vocabulary: a fifth kind on the Go side fails here rather than arriving unrecognised.
    const kinds = new Set(
      readdirSync(join(WIRE_DIR, "commands")).map(
        (name) => (fixture("commands", name) as { type: string }).type,
      ),
    );
    expect([...kinds].sort()).toEqual(
      [
        ENVELOPE_CANCEL,
        ENVELOPE_CONFIG,
        ENVELOPE_ELICITATION,
        ENVELOPE_PERMISSION,
        ENVELOPE_PROMPT,
      ].sort(),
    );
  });

  it("carries the agent's own config options, which is where the model list arrives", () => {
    const envelope = fixture("session.json") as AgentStateEnvelope;
    const config = envelope.thread.config as
      | { id: string; category?: string; currentValue?: string; options?: { value: string }[] }[]
      | undefined;
    // The open's own answer is the only place the list arrives: a real agent sends no
    // config update afterwards, so a bridge that dropped it would show an empty control.
    expect(config).toHaveLength(1);
    expect(config?.[0].id).toBe("model");
    expect(config?.[0].category).toBe("model");
    expect(config?.[0].currentValue).toBe("openrouter/meta/muse-spark-open");
    expect(config?.[0].options?.map((option) => option.value)).toEqual([
      "openrouter/meta/muse-spark-open",
      "opencode/deepseek-v4.1-flash-free",
      "opencode/big-pickle",
    ]);
  });
});

describe("the posture frame", () => {
  it("carries exactly the fields the panel reads, and no thread", () => {
    const frame = fixture("status.json") as Record<string, unknown>;
    expect(frame.type).toBe(ENVELOPE_STATUS);
    expect(Object.keys(frame).sort()).toEqual([
      "activated",
      "connected",
      "model",
      "sessionId",
      "status",
      "type",
    ]);
    expect(frame).not.toHaveProperty("thread");
    expect(frame).not.toHaveProperty("command");
  });
});
