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
  ENVELOPE_CANCEL,
  ENVELOPE_PERMISSION,
  ENVELOPE_PROMPT,
  ENVELOPE_STATE,
  PART_ERROR,
  PART_PERMISSION,
  PART_STOP,
  PART_TEXT,
  PART_THOUGHT,
  PART_TOOL_CALL,
  cancelCommand,
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
      for (const message of envelope.thread.messages) {
        for (const part of message.parts) kinds.add(part.kind);
      }
    }

    // A new part kind on the Go side fails here, rather than showing up in the thread
    // as a blank line.
    expect([...kinds].sort()).toEqual(
      [PART_TEXT, PART_THOUGHT, PART_TOOL_CALL, PART_PERMISSION].sort(),
    );
  });

  it("reads the scripted session with the field names the converter uses", () => {
    const envelope = fixture("session.json") as AgentStateEnvelope;
    expect(envelope.type).toBe(ENVELOPE_STATE);
    expect(envelope.sessionId).toBe("sess-1");

    const [user, assistant] = envelope.thread.messages;
    expect(user.role).toBe("user");
    expect(user.parts[0]).toEqual({ kind: PART_TEXT, text: "how big is the corpus?" });

    // The streaming rule, visible in the fixture: the chunks that arrive before the
    // tool call share one text part, and the one after it opens a new part.
    const kinds = assistant.parts.map((part) => part.kind);
    expect(kinds).toEqual([PART_TEXT, PART_THOUGHT, PART_TOOL_CALL, PART_TEXT, PART_PERMISSION]);
    expect(assistant.parts[0].text).toBe("Let me look at the notes.");
    expect(assistant.parts[3].text).toBe(" The corpus has 224 analyses.");

    const call = assistant.parts[2].toolCall;
    expect(call).toEqual({
      id: "call-1",
      title: "Read context/notes/a.md",
      kind: "read",
      status: "completed",
      output: "224 analyses",
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

    // And every kind the fixture set uses is one of the four the wire defines.
    const kinds = new Set(
      ["prompt", "cancel", "permission"].map(
        (name) => (fixture("commands", `${name}.json`) as { type: string }).type,
      ),
    );
    expect([...kinds].sort()).toEqual(
      [ENVELOPE_CANCEL, ENVELOPE_PERMISSION, ENVELOPE_PROMPT].sort(),
    );
  });
});
