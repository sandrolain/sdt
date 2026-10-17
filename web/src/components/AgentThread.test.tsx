// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AgentThread } from "./AgentThread";
import {
  PART_TEXT,
  PART_THOUGHT,
  PART_TOOL_CALL,
  type AgentPart,
  type AgentThread as AgentThreadState,
} from "../lib/agent";

// The parse is counted, not the component: this test file owns the claim that an unchanged
// part is not re-parsed when a later frame arrives.
vi.mock("../lib/markdown", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/markdown")>();
  return { ...actual, renderMarkdown: vi.fn(actual.renderMarkdown) };
});
import { renderMarkdown } from "../lib/markdown";

/** threadOf is one session with one assistant message carrying these parts. */
function threadOf(parts: AgentPart[]): AgentThreadState {
  return { sessionId: "sess-1", messages: [{ id: "msg-1", role: "assistant", parts }] };
}

function show(parts: AgentPart[]) {
  return render(<AgentThread thread={threadOf(parts)} onAnswer={() => undefined} />);
}

/** parses counts the parses of one exact string, so a second part cannot pass for it. */
const parses = (text: string) =>
  vi.mocked(renderMarkdown).mock.calls.filter(([md]) => md === text).length;

afterEach(() => {
  cleanup();
  vi.mocked(renderMarkdown).mockClear();
});

describe("AgentThread", () => {
  it("renders the agent's prose as the markdown it is", () => {
    show([{ kind: PART_TEXT, text: "a **bold** claim and a [link](https://example.com)" }]);

    // The corpus renderer's HTML, not the characters the agent sent.
    expect(document.querySelector(".agent-panel__markdown strong")?.textContent).toBe("bold");
    expect(
      document.querySelector<HTMLAnchorElement>(".agent-panel__markdown a")?.getAttribute("href"),
    ).toBe("https://example.com");
  });

  it("parses a part's prose once, however many frames arrive", () => {
    const view = show([{ kind: PART_TEXT, text: "unique prose markdown" }]);
    expect(parses("unique prose markdown")).toBe(1);

    // A later frame: the same message, with a second one after it. A part whose text has not
    // changed keeps the HTML it already has rather than being parsed again.
    view.rerender(
      <AgentThread
        thread={{
          sessionId: "sess-1",
          messages: [
            {
              id: "msg-1",
              role: "assistant",
              parts: [{ kind: PART_TEXT, text: "unique prose markdown" }],
            },
            {
              id: "msg-2",
              role: "assistant",
              parts: [{ kind: PART_TEXT, text: "the later answer" }],
            },
          ],
        }}
        onAnswer={() => undefined}
      />,
    );

    expect(parses("unique prose markdown")).toBe(1);
    expect(parses("the later answer")).toBe(1);
  });

  it("sanitises what the agent writes instead of running it", () => {
    show([{ kind: PART_TEXT, text: '<img src=x onerror="alert(1)">' }]);

    expect(document.querySelector("[onerror]")).toBeNull();
    expect(document.querySelector("script")).toBeNull();
  });

  it("reads as the command and its outcome, with the protocol's record behind a disclosure", () => {
    show([
      {
        kind: PART_TOOL_CALL,
        toolCall: {
          id: "t1",
          title: "bash: go test ./...",
          kind: "execute",
          status: "completed",
          input: { command: "go test ./..." },
          output: { exitCode: 0 },
          content: "the tests passed",
        },
      },
    ]);

    // The command once, with the state as a word rather than only as a colour.
    expect(screen.getByText("bash: go test ./...")).toBeTruthy();
    expect(screen.getByText("Completed")).toBeTruthy();

    // The body is the tool's own output, not the protocol's record of it.
    const body = document.querySelector(".agent-panel__tool-result");
    expect(body?.textContent).toBe("the tests passed");

    const envelope = document.querySelector(".agent-panel__tool-envelope");
    expect(envelope?.querySelector("summary")?.textContent).toBe("Protocol envelope");
    expect(envelope?.textContent).toContain('{"command":"go test ./..."}');
    expect(envelope?.textContent).toContain('{"exitCode":0}');
  });

  it("names the state of a call that failed and of one still running", () => {
    show([
      {
        kind: PART_TOOL_CALL,
        toolCall: { id: "t1", title: "edit", status: "failed", output: "no" },
      },
    ]);
    expect(screen.getByText("Failed")).toBeTruthy();

    cleanup();
    show([{ kind: PART_TOOL_CALL, toolCall: { id: "t2", title: "read", status: "in_progress" } }]);
    expect(screen.getByText("Running")).toBeTruthy();
  });

  it("shows a call with no output at all as the command alone", () => {
    show([{ kind: PART_TOOL_CALL, toolCall: { id: "t1", title: "read a file" } }]);

    expect(screen.getByText("read a file")).toBeTruthy();
    expect(document.querySelector(".agent-panel__tool-result")).toBeNull();
    expect(document.querySelector(".agent-panel__tool-envelope")).toBeNull();
  });

  it("keeps the agent's thinking collapsed behind its own word", () => {
    show([{ kind: PART_THOUGHT, text: "weighing the options" }]);

    expect(screen.getByText("Reasoning")).toBeTruthy();
    expect(screen.getByText("weighing the options")).toBeTruthy();
  });
});
