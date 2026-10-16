import { describe, expect, it } from "vitest";
import { partLike, threadMessages } from "./agentThread";
import {
  PART_PERMISSION,
  PART_TEXT,
  PART_THOUGHT,
  PART_TOOL_CALL,
  type AgentMessage,
  type AgentPart,
  type AgentThread,
} from "./agent";

function thread(messages: AgentMessage[], plan?: AgentThread["plan"]): AgentThread {
  return { sessionId: "sess-1", messages, plan };
}

function message(role: string, parts: AgentPart[]): AgentMessage {
  return { id: `msg-${role}`, role, parts };
}

describe("the thread converter", () => {
  it("maps a text part to assistant-ui's text, and the role to its own", () => {
    const [converted] = threadMessages(
      thread([message("user", [{ kind: PART_TEXT, text: "how big?" }])]),
    );
    expect(converted.role).toBe("user");
    expect(converted.content).toEqual([{ type: "text", text: "how big?" }]);
    // Only an assistant message carries a status: the runtime refuses one elsewhere.
    expect("status" in converted).toBe(false);
  });

  it("maps the agent's reasoning to a reasoning part, not into its answer", () => {
    const [converted] = threadMessages(
      thread([
        message("assistant", [
          { kind: PART_THOUGHT, text: "checking the corpus" },
          { kind: PART_TEXT, text: "224 analyses." },
        ]),
      ]),
    );
    expect(converted.content).toEqual([
      { type: "reasoning", text: "checking the corpus" },
      { type: "text", text: "224 analyses." },
    ]);
  });

  it("maps the tool call in flight and completed, with its error state", () => {
    const inFlight = partLike({
      kind: PART_TOOL_CALL,
      toolCall: { id: "call-1", title: "Read a file", input: { path: "a.md" } },
    });
    expect(inFlight).toEqual({
      type: "tool-call",
      toolCallId: "call-1",
      toolName: "Read a file",
      argsText: JSON.stringify({ path: "a.md" }),
      result: undefined,
      isError: false,
      status: "running",
    });

    const failed = partLike({
      kind: PART_TOOL_CALL,
      toolCall: { id: "call-2", kind: "edit", status: "failed", output: "no" },
    });
    expect(failed).toMatchObject({
      toolName: "edit",
      result: "no",
      isError: true,
      status: "incomplete",
    });

    // A tool call with no call at all is still a part the runtime can render.
    expect(partLike({ kind: PART_TOOL_CALL })).toMatchObject({ type: "tool-call", toolCallId: "" });
  });

  it("maps a permission request to a data part, and the turn to requires-action until it is answered", () => {
    const card: AgentPart = {
      kind: PART_PERMISSION,
      permission: {
        requestId: "perm-1",
        sessionId: "sess-1",
        title: "Write context/notes/x.md",
        toolKind: "edit",
      },
    };
    const [pending] = threadMessages(thread([message("assistant", [card])]));
    expect(pending.content).toEqual([{ type: "data-permission", data: card.permission }]);
    expect(pending.status).toEqual({ type: "requires-action", reason: "tool-calls" });

    const answered = threadMessages(
      thread([
        message("assistant", [
          { ...card, permission: { ...card.permission!, decision: "allowed" } },
        ]),
      ]),
    );
    expect(answered[0].status).toEqual({ type: "complete" });
  });

  it("appends the plan as its own message under a stable id, and only when there is one", () => {
    const withPlan = threadMessages(
      thread([message("assistant", [{ kind: PART_TEXT, text: "done" }])], {
        entries: [{ content: "read the notes", status: "completed" }],
      }),
    );
    expect(withPlan).toHaveLength(2);
    expect(withPlan[1]).toEqual({
      id: "plan",
      role: "assistant",
      content: [
        {
          type: "data-plan",
          data: { entries: [{ content: "read the notes", status: "completed" }] },
        },
      ],
    });

    expect(threadMessages(thread([message("assistant", [])], { entries: [] }))).toHaveLength(1);
  });

  it("shows a part kind this build does not know rather than dropping it", () => {
    expect(partLike({ kind: "a_future_thing" })).toEqual({ type: "text", text: "a_future_thing" });
    expect(partLike({ kind: "a_future_thing", text: "its own text" })).toEqual({
      type: "text",
      text: "its own text",
    });
  });
});
