import { describe, expect, it } from "vitest";
import { partLike, threadMessages } from "./agentThread";
import {
  PART_ELICITATION,
  PART_ERROR,
  PART_PERMISSION,
  PART_STOP,
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

  it("states a turn in flight once, at the end of the thread, not on the reader's message", () => {
    // A refusal: the agent's own sentence, as the data part the panel renders.
    expect(partLike({ kind: PART_ERROR, text: "the provider refused it" })).toEqual({
      type: "data-error",
      data: { text: "the provider refused it" },
    });
    expect(partLike({ kind: PART_STOP, text: "max_tokens" })).toEqual({
      type: "data-stop",
      data: { reason: "max_tokens" },
    });

    // The wait is the thread's own fact, not a part of the reader's message: the message is
    // over, the turn is not, so the state goes last and the reader's text stands alone.
    const sent = {
      ...message("user", [{ kind: PART_TEXT, text: "how big?" }]),
      provisional: true,
    };
    const [only] = threadMessages(thread([sent]));
    expect(only.content).toEqual([{ type: "text", text: "how big?" }]);

    const running = { ...thread([sent]), running: true };
    const whileRunning = threadMessages(running);
    expect(whileRunning.at(-1)).toEqual({
      id: "waiting",
      role: "assistant",
      content: [{ type: "data-waiting", data: {} }],
    });
    expect(whileRunning.at(-2)?.content).toEqual([{ type: "text", text: "how big?" }]);
  });

  it("drops a part with nothing to say, and a message left with nothing", () => {
    const messages = threadMessages(
      thread([
        // A streamed answer opens with an empty chunk: a bubble with no text is not a state.
        message("assistant", [
          { kind: PART_TEXT, text: "" },
          { kind: PART_THOUGHT, text: "   " },
          { kind: PART_TEXT, text: "ecco" },
        ]),
        message("assistant", [{ kind: PART_TEXT, text: "\n" }]),
      ]),
    );
    expect(messages).toHaveLength(1);
    expect(messages[0].content).toEqual([{ type: "text", text: "ecco" }]);
  });

  it("maps a question to the data part the panel answers in place", () => {
    const question = {
      requestId: "elicit-1",
      sessionId: "sess-1",
      mode: "form",
      message: "Which branch?",
      schema: { type: "object", properties: { branch: { type: "string" } } },
    };
    expect(partLike({ kind: PART_ELICITATION, elicitation: question })).toEqual({
      type: "data-elicitation",
      data: question,
    });
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
      artifact: { input: { path: "a.md" }, output: undefined, status: undefined },
    });

    // The tool's own output wins over the raw record: the reader asked for the command's
    // result, and the protocol's envelope is kept beside it for the disclosure.
    expect(
      partLike({
        kind: PART_TOOL_CALL,
        toolCall: {
          id: "call-3",
          kind: "execute",
          status: "completed",
          content: "the tests passed",
          output: { exitCode: 0 },
        },
      }),
    ).toMatchObject({
      result: "the tests passed",
      artifact: { input: undefined, output: { exitCode: 0 }, status: "completed" },
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

    expect(
      threadMessages(
        thread([message("assistant", [{ kind: PART_TEXT, text: "done" }])], { entries: [] }),
      ).map((converted) => converted.id),
    ).toEqual(["msg-assistant"]);
  });

  it("marks an unanswered question as waiting on the reader, and an answered one as done", () => {
    const question: AgentPart = {
      kind: PART_ELICITATION,
      elicitation: {
        requestId: "elicit-1",
        sessionId: "sess-1",
        mode: "form",
        message: "Which branch?",
      },
    };
    const [pending] = threadMessages(thread([message("assistant", [question])]));
    expect(pending.status).toEqual({ type: "requires-action", reason: "tool-calls" });

    const [answered] = threadMessages(
      thread([
        message("assistant", [
          {
            ...question,
            elicitation: { ...question.elicitation!, decision: "accepted", content: {} },
          },
        ]),
      ]),
    );
    expect(answered.status).toEqual({ type: "complete" });
  });

  it("shows a part kind this build does not know rather than dropping it", () => {
    expect(partLike({ kind: "a_future_thing" })).toEqual({ type: "text", text: "a_future_thing" });
    expect(partLike({ kind: "a_future_thing", text: "its own text" })).toEqual({
      type: "text",
      text: "its own text",
    });
  });
});
