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
      thread={threadOf(parts)}
      onAnswer={() => undefined}
      onElicitation={() => undefined}
    />,
  );
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

    // The command line is the agent's own command, highlighted — not the bold ACP title.
    const command = document.querySelector(".agent-panel__tool-command");
    expect(command?.textContent).toBe("go test ./...");
    expect(command?.classList.contains("hljs")).toBe(true);
    expect(document.querySelector(".agent-panel__tool-name")).toBeNull();
    expect(screen.getByText("Completed")).toBeTruthy();

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
    expect(screen.getByText("Failed")).toBeTruthy();
    const failedBadge = document.querySelector(".agent-panel__tool-state");
    expect(failedBadge?.getAttribute("data-state")).toBe("failed");
    expect(failedBadge?.closest(".agent-panel__tool")).toBeTruthy();

    cleanup();
    show([{ kind: PART_TOOL_CALL, toolCall: { id: "t2", title: "read", status: "in_progress" } }]);
    expect(screen.getByText("Running")).toBeTruthy();
    expect(document.querySelector(".agent-panel__tool-state")?.getAttribute("data-state")).toBe(
      "running",
    );
  });

  it("shows a call with no output at all as the command alone", () => {
    show([{ kind: PART_TOOL_CALL, toolCall: { id: "t1", title: "read a file" } }]);

    expect(screen.getByText("read a file")).toBeTruthy();
    expect(document.querySelector(".agent-panel__tool-output")).toBeNull();
    expect(document.querySelector(".agent-panel__tool-result")).toBeNull();
    expect(document.querySelector(".agent-panel__tool-envelope")).toBeNull();
  });

  it("keeps the agent's thinking collapsed behind its own word", () => {
    show([{ kind: PART_THOUGHT, text: "weighing the options" }]);

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
});
