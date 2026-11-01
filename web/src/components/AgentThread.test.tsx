// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AgentThread } from "./AgentThread";
import {
  ACTION_ACCEPT,
  ACTION_CANCEL,
  ACTION_DECLINE,
  PART_ELICITATION,
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

// The renderer is the browser's job; this file owns the wiring claim, so the module is
// replaced and the mount is observed instead of a diagram being drawn in jsdom.
vi.mock("../lib/mermaidRender", () => ({ renderMermaid: vi.fn(() => Promise.resolve()) }));
import { renderMermaid } from "../lib/mermaidRender";

/** threadOf is one session with one assistant message carrying these parts. */
function threadOf(parts: AgentPart[]): AgentThreadState {
  return { sessionId: "sess-1", messages: [{ id: "msg-1", role: "assistant", parts }] };
}

function show(parts: AgentPart[]) {
  return render(
    <AgentThread
      onSend={async () => undefined}
      onCancel={() => undefined}
      thread={threadOf(parts)}
      onAnswer={() => undefined}
      onElicitation={() => undefined}
    />,
  );
}

/** openChain opens the turn's own disclosure: a finished turn leaves it closed, so a test
 *  that reads what the turn ran has to open it first — the reader does the same. */
function openChain() {
  const head = document.querySelector<HTMLButtonElement>(".agent-panel__chain-head");
  if (head && head.getAttribute("aria-expanded") === "false") fireEvent.click(head);
  return head;
}

/** parses counts the parses of one exact string, so a second part cannot pass for it. */
const parses = (text: string) =>
  vi.mocked(renderMarkdown).mock.calls.filter(([md]) => md === text).length;

afterEach(() => {
  cleanup();
  vi.mocked(renderMarkdown).mockClear();
  vi.mocked(renderMermaid).mockClear();
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

  it("keeps the newlines of what was typed or written", () => {
    show([{ kind: PART_TEXT, text: "prima riga\nseconda riga" }]);

    // The chat renders through the corpus renderer, with its `breaks` option on: a single
    // newline is the reader's own line break, not a space.
    const html = document.querySelector(".agent-panel__markdown")?.innerHTML ?? "";
    expect(html).toContain("prima riga<br>seconda riga");
  });

  it("gives a fenced block the document renderer's chrome, and its copy control works", async () => {
    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    show([{ kind: PART_TEXT, text: "```js\nconst x = 1;\n```" }]);

    // The renderer's block, not a bare <pre>: this is what the panel's stylesheet targets.
    expect(document.querySelector(".agent-panel__markdown .md-code-block")).toBeTruthy();
    const copy = document.querySelector<HTMLButtonElement>(".md-code__copy");
    expect(copy).toBeTruthy();
    copy?.click();
    expect(writeText).toHaveBeenCalledWith("const x = 1;");
    Reflect.deleteProperty(navigator, "clipboard");
  });

  it("mounts the document's mermaid renderer on a diagram fence", async () => {
    show([{ kind: PART_TEXT, text: "```mermaid\nflowchart LR\n  A --> B\n```" }]);

    await waitFor(() => expect(vi.mocked(renderMermaid)).toHaveBeenCalled());
    const host = vi.mocked(renderMermaid).mock.calls.at(-1)?.[0];
    expect(host?.querySelector(".md-mermaid")).toBeTruthy();
  });

  it("parses a part's prose once, however many frames arrive", () => {
    const view = show([{ kind: PART_TEXT, text: "unique prose markdown" }]);
    expect(parses("unique prose markdown")).toBe(1);

    // A later frame: the same message, with a second one after it. A part whose text has not
    // changed keeps the HTML it already has rather than being parsed again.
    view.rerender(
      <AgentThread
        onSend={async () => undefined}
        onCancel={() => undefined}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
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

  it("keeps a command's output collapsed behind the size it states", () => {
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

    openChain();

    // The command line is the agent's own command, highlighted — not the bold ACP title.
    const command = document.querySelector(".agent-panel__tool-command");
    expect(command?.textContent).toBe("go test ./...");
    expect(command?.classList.contains("hljs")).toBe(true);
    expect(document.querySelector(".agent-panel__tool-name")).toBeNull();
    expect(screen.getAllByText("Completed").length).toBeGreaterThan(0);

    // The output is the disclosure's body, closed until the reader opens it, and the summary
    // says what opening it costs — the reader's own unit, not a character count.
    const output = document.querySelector<HTMLDetailsElement>(".agent-panel__tool-output");
    expect(output?.open).toBe(false);
    expect(output?.querySelector("summary")?.textContent).toBe("Output · 1 line");
    expect(output?.querySelector(".agent-panel__tool-result")?.textContent).toBe(
      "the tests passed",
    );

    // The protocol's record stays where it was: a second disclosure, after the output.
    const envelope = document.querySelector(".agent-panel__tool-envelope");
    expect(envelope?.querySelector("summary")?.textContent).toBe("Protocol envelope");
    expect(envelope?.textContent).toContain('{"command":"go test ./..."}');
    expect(envelope?.textContent).toContain('{"exitCode":0}');
  });

  it("highlights a JSON output as JSON, inside the closed body", () => {
    show([
      {
        kind: PART_TOOL_CALL,
        toolCall: {
          id: "t1",
          title: "bash: curl",
          status: "completed",
          output: '{"answer":42}',
        },
      },
    ]);

    openChain();

    const body = document.querySelector<HTMLDetailsElement>(".agent-panel__tool-output");
    expect(body?.open).toBe(false);
    const code = body?.querySelector(".agent-panel__tool-result code");
    expect(code?.className).toContain("language-json");
    expect(code?.textContent).toBe('{"answer":42}');
  });

  it("counts a long output's lines in the disclosure's summary", () => {
    show([
      {
        kind: PART_TOOL_CALL,
        toolCall: { id: "t1", title: "bash: diff", status: "completed", content: "a\nb\nc" },
      },
    ]);

    openChain();
    expect(document.querySelector(".agent-panel__tool-output summary")?.textContent).toBe(
      "Output · 3 lines",
    );

    // A trailing newline ends the last line rather than starting an empty one: a command
    // whose output is four lines and a newline does not report five.
    cleanup();
    show([
      {
        kind: PART_TOOL_CALL,
        toolCall: { id: "t2", title: "bash: printf", status: "completed", content: "a\nb\nc\nd\n" },
      },
    ]);
    openChain();
    expect(document.querySelector(".agent-panel__tool-output summary")?.textContent).toBe(
      "Output · 4 lines",
    );
  });

  it("names the state of a call that failed and of one still running", () => {
    show([
      {
        kind: PART_TOOL_CALL,
        toolCall: { id: "t1", title: "edit", status: "failed", output: "no" },
      },
    ]);
    openChain();
    expect(screen.getAllByText("Failed").length).toBeGreaterThan(0);
    const failedBadge = document.querySelector(".agent-panel__tool-state");
    expect(failedBadge?.getAttribute("data-state")).toBe("failed");
    expect(failedBadge?.closest(".agent-panel__tool")).toBeTruthy();

    cleanup();
    show([{ kind: PART_TOOL_CALL, toolCall: { id: "t2", title: "read", status: "in_progress" } }]);
    openChain();
    expect(screen.getAllByText("Running").length).toBeGreaterThan(0);
    expect(document.querySelector(".agent-panel__tool-state")?.getAttribute("data-state")).toBe(
      "running",
    );
  });

  it("shows a call with no output at all as the command alone", () => {
    show([{ kind: PART_TOOL_CALL, toolCall: { id: "t1", title: "read a file" } }]);
    openChain();

    expect(screen.getByText("read a file")).toBeTruthy();
    expect(document.querySelector(".agent-panel__tool-output")).toBeNull();
    expect(document.querySelector(".agent-panel__tool-result")).toBeNull();
    expect(document.querySelector(".agent-panel__tool-envelope")).toBeNull();
  });

  it("keeps the agent's thinking collapsed behind its own word", () => {
    show([{ kind: PART_THOUGHT, text: "weighing the options" }]);
    openChain();

    expect(screen.getByText("Reasoning")).toBeTruthy();
    expect(screen.getByText("weighing the options")).toBeTruthy();
  });
});

describe("the agent's question", () => {
  /** questionCard renders one elicitation part and hands back the answer spy. */
  function questionCard(
    data: Partial<import("../lib/agent").AgentElicitation> = {},
  ): ReturnType<typeof vi.fn> {
    const onElicitation = vi.fn();
    render(
      <AgentThread
        onSend={async () => undefined}
        onCancel={() => undefined}
        thread={threadOf([
          {
            kind: PART_ELICITATION,
            elicitation: {
              requestId: "elicit-1",
              sessionId: "sess-1",
              mode: "form",
              message: "Which branch?",
              schema: {
                type: "object",
                properties: {
                  branch: { type: "string", title: "Branch" },
                  push: { type: "boolean", title: "Push it" },
                },
                required: ["branch"],
              },
              ...data,
            } as never,
          },
        ])}
        onAnswer={() => undefined}
        onElicitation={onElicitation}
      />,
    );
    return onElicitation;
  }

  it("renders one control per property and accepts with the typed values", () => {
    const onElicitation = questionCard();
    expect(screen.getByText("Which branch?")).toBeTruthy();
    expect(screen.getByText("Branch *")).toBeTruthy();

    const accept = screen.getByRole("button", { name: "Accept" }) as HTMLButtonElement;
    // A required field nobody filled cannot be accepted: the reader sees why, not a
    // silent no-op.
    expect(accept.disabled).toBe(true);

    fireEvent.change(screen.getByRole("textbox", { name: /Branch/ }), {
      target: { value: "main" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /Push it/ }));
    fireEvent.click(screen.getByRole("button", { name: "Accept" }));

    expect(onElicitation).toHaveBeenCalledWith("elicit-1", ACTION_ACCEPT, {
      branch: "main",
      push: true,
    });
  });

  it("declines without sending content", () => {
    const onElicitation = questionCard();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    expect(onElicitation).toHaveBeenCalledWith("elicit-1", ACTION_DECLINE);
  });

  it("keeps the decision and the values in place once answered", () => {
    questionCard({ decision: "accepted", content: { branch: "main" } });
    expect(screen.getByText("accepted")).toBeTruthy();
    expect(screen.getByText("main")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Accept" })).toBeNull();
  });

  it("renders a url question as a link and a cancel", () => {
    const onElicitation = questionCard({
      mode: "url",
      message: "Open the provider page",
      url: "https://example.com/auth",
      elicitationId: "e-1",
    });
    const link = screen.getByRole("link");
    expect(link.getAttribute("href")).toBe("https://example.com/auth");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onElicitation).toHaveBeenCalledWith("elicit-1", ACTION_CANCEL);
  });

  it("lets a required boolean be answered: false is an answer, not a missing field", () => {
    const onElicitation = questionCard({
      schema: {
        type: "object",
        properties: { push: { type: "boolean", title: "Push it" } },
        required: ["push"],
      },
    });

    const accept = screen.getByRole("button", { name: "Accept" }) as HTMLButtonElement;
    expect(accept.disabled).toBe(false);
    fireEvent.click(accept);
    expect(onElicitation).toHaveBeenCalledWith("elicit-1", ACTION_ACCEPT, { push: false });
  });

  it("renders a choice field as the app's own select, carrying the agent's options", () => {
    questionCard({
      schema: {
        type: "object",
        properties: {
          approccio: {
            type: "string",
            title: "Approccio",
            enum: ["riassumi", "dettaglia", "confronta"],
          },
        },
        required: ["approccio"],
      },
    });

    // Not the browser's own dropdown: the panel's control, the same one the model picker is.
    // (React Aria keeps a hidden native <select> for form semantics; what the reader sees and
    // operates is the app's button.)
    const control = screen.getByRole("button", { name: /Approccio/ });
    expect(control.className).toContain("ui-select__button");
    expect(document.querySelector("select[tabindex='-1']")).toBeTruthy();

    fireEvent.click(control);
    expect(screen.getByRole("option", { name: "dettaglia" })).toBeTruthy();
  });

  it("declines a question whose fields this panel cannot render", () => {
    questionCard({
      schema: {
        type: "object",
        properties: { files: { type: "array", title: "Files" } },
      },
    });
    expect(screen.getByText(/cannot render/)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Accept" }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    expect(screen.getByRole("button", { name: "Decline" })).toBeTruthy();
  });

  it("states a turn in flight as one line at the end, and draws no empty bubble", () => {
    render(
      <AgentThread
        onSend={async () => undefined}
        onCancel={() => undefined}
        thread={{
          sessionId: "sess-1",
          running: true,
          messages: [
            { id: "user-1", role: "user", parts: [{ kind: PART_TEXT, text: "prima riga" }] },
            // A streamed answer opens with an empty chunk; a message with nothing to show
            // is not a bubble, and the wait is a word rather than a blank surface.
            { id: "msg-1", role: "assistant", parts: [{ kind: PART_TEXT, text: "" }] },
          ],
        }}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
      />,
    );

    expect(screen.getByText("waiting for the agent…")).toBeTruthy();
    // Exactly two surfaces: the reader's message and the wait's. The empty one is absent.
    expect(document.querySelectorAll(".agent-panel__message")).toHaveLength(2);
  });
});

describe("the thread's own affordances", () => {
  /** empty renders a thread with no messages, handing back the send spy. */
  function empty() {
    const onSend = vi.fn(async () => undefined);
    render(
      <AgentThread
        thread={{ sessionId: "sess-1", messages: [] }}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
        onSend={onSend}
        onCancel={() => undefined}
      />,
    );
    return onSend;
  }

  it("states what an empty thread is and offers three starters", () => {
    empty();

    expect(document.querySelector(".agent-thread__empty")).toBeTruthy();
    expect(screen.getByText(/The agent this project declares/)).toBeTruthy();
    expect(document.querySelectorAll(".agent-thread__starter")).toHaveLength(3);
  });

  it("sends a starter through the runtime's own send path", async () => {
    const onSend = empty();

    fireEvent.click(
      screen.getByRole("button", { name: "Summarise the plan that is in progress." }),
    );

    await waitFor(() =>
      expect(onSend).toHaveBeenCalledWith("Summarise the plan that is in progress."),
    );
  });

  it("keeps the empty state out of a turn that is starting", () => {
    // A turn in flight has its own message (`WAITING_ID`), so the thread is not empty: the
    // starters must not appear over a turn the reader just started.
    render(
      <AgentThread
        thread={{ sessionId: "sess-1", running: true, messages: [] }}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
        onSend={async () => undefined}
        onCancel={() => undefined}
      />,
    );

    expect(document.querySelector(".agent-thread__empty")).toBeNull();
    expect(screen.getByText(/waiting for the agent/)).toBeTruthy();
  });

  it("offers the way back to the latest message", () => {
    show([{ kind: PART_TEXT, text: "an answer" }]);

    expect(screen.getByRole("button", { name: "Jump to the latest message" })).toBeTruthy();
  });
});

describe("what a reader can do with an answer", () => {
  it("offers copy and copy-as-markdown on an assistant message only", () => {
    render(
      <AgentThread
        thread={{
          sessionId: "sess-1",
          messages: [
            { id: "u1", role: "user", parts: [{ kind: PART_TEXT, text: "the question" }] },
            { id: "m1", role: "assistant", parts: [{ kind: PART_TEXT, text: "the answer" }] },
          ],
        }}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
        onSend={async () => undefined}
        onCancel={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: "Copy" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Copy as markdown" })).toBeTruthy();
    // One row for the assistant reply, and none under the reader's own message.
    expect(document.querySelectorAll(".agent-panel__actions")).toHaveLength(1);
  });

  it("copies the message's markdown source, not its rendered HTML", async () => {
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });

    show([{ kind: PART_TEXT, text: "a **bold** claim" }]);
    fireEvent.click(screen.getByRole("button", { name: "Copy as markdown" }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith("a **bold** claim"));
  });
});

describe("a turn's chain", () => {
  const call = (id: string, command: string): AgentPart => ({
    kind: PART_TOOL_CALL,
    toolCall: {
      id,
      title: `bash: ${command}`,
      status: "completed",
      input: { command },
      content: "ok",
    },
  });

  it("folds a run of calls into one chain that states how many", () => {
    show([call("t1", "go build ./..."), call("t2", "go test ./..."), call("t3", "go vet ./...")]);

    expect(screen.getByText("Worked through 3 steps")).toBeTruthy();
    expect(document.querySelectorAll(".agent-panel__chain")).toHaveLength(1);
    // A finished turn is closed: the commands are behind the one line, not stacked beside it.
    expect(document.querySelector(".agent-panel__tool")).toBeNull();

    openChain();
    expect(document.querySelector(".agent-panel__tool-group")).toBeTruthy();
    expect(document.querySelectorAll(".agent-panel__tool")).toHaveLength(3);
  });

  it("keeps a finished turn closed and opens it on a click", () => {
    show([call("t1", "go build ./...")]);

    const head = document.querySelector<HTMLButtonElement>(".agent-panel__chain-head");
    expect(head?.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector(".agent-panel__tool")).toBeNull();

    fireEvent.click(head as HTMLButtonElement);

    expect(head?.getAttribute("aria-expanded")).toBe("true");
    expect(document.querySelector(".agent-panel__tool")).toBeTruthy();
  });

  it("opens while the turn is in flight and closes when it ends", () => {
    const parts = [call("t1", "go build ./...")];
    const view = render(
      <AgentThread
        thread={{
          sessionId: "sess-1",
          running: true,
          messages: [{ id: "m1", role: "assistant", parts }],
        }}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
        onSend={async () => undefined}
        onCancel={() => undefined}
      />,
    );
    expect(document.querySelector(".agent-panel__tool")).toBeTruthy();

    view.rerender(
      <AgentThread
        thread={{ sessionId: "sess-1", messages: [{ id: "m1", role: "assistant", parts }] }}
        onAnswer={() => undefined}
        onElicitation={() => undefined}
        onSend={async () => undefined}
        onCancel={() => undefined}
      />,
    );

    expect(document.querySelector(".agent-panel__chain-head")?.getAttribute("aria-expanded")).toBe(
      "false",
    );
  });

  it("states the step in the singular for one call", () => {
    show([call("t1", "go build ./...")]);

    expect(screen.getByText("Worked through 1 step")).toBeTruthy();
  });
});

describe("a question with several answers", () => {
  it("renders an array of enums as a choice instead of declining it", async () => {
    const onElicitation = vi.fn();
    render(
      <AgentThread
        thread={threadOf([
          {
            kind: PART_ELICITATION,
            elicitation: {
              requestId: "elicit-1",
              sessionId: "sess-1",
              mode: "form",
              message: "Which files?",
              schema: {
                type: "object",
                properties: {
                  files: {
                    type: "array",
                    title: "Files",
                    items: { type: "string", enum: ["a.ts", "b.ts"] },
                  },
                },
                required: ["files"],
              },
            },
          },
        ])}
        onAnswer={() => undefined}
        onElicitation={onElicitation}
        onSend={async () => undefined}
        onCancel={() => undefined}
      />,
    );

    // The field is renderable, so the card explains nothing away and Accept waits for a choice.
    expect(screen.queryByText(/cannot render/)).toBeNull();
    const accept = screen.getByRole("button", { name: "Accept" }) as HTMLButtonElement;
    expect(accept.disabled).toBe(true);
    expect(document.querySelector(".agent-panel__elicitation .ui-select--multi")).toBeTruthy();
  });
});
