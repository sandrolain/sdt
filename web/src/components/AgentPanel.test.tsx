// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AgentPanel } from "./AgentPanel";
import {
  AGENT_ACTIVATE_PATH,
  AGENT_COMMAND_PATH,
  AGENT_STATE_PATH,
  AGENT_STREAM_PATH,
  type AgentStateEnvelope,
} from "../lib/agent";

/** FakeEventSource is the browser side the panel talks to, driven by the test. */
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners = new Map<string, ((event: unknown) => void)[]>();
  closed = false;

  url: string;

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(kind: string, listener: (event: unknown) => void) {
    this.listeners.set(kind, [...(this.listeners.get(kind) ?? []), listener]);
  }

  emit(kind: string, event: unknown) {
    for (const listener of this.listeners.get(kind) ?? []) listener(event);
  }

  close() {
    this.closed = true;
  }
}

function state(
  thread: Partial<AgentStateEnvelope["thread"]> = {},
  sessionId = "sess-1",
): AgentStateEnvelope {
  return { type: "state", sessionId, thread: { sessionId, messages: [], ...thread } };
}

/**
 * fixtureThread is the thread the Go side writes, read from the same file the wire test reads:
 * the panel is exercised against the shape the bridge actually sends, model option included,
 * rather than against one this test invented.
 */
function fixtureThread(): AgentStateEnvelope {
  const path = join(
    process.cwd(),
    "..",
    "internal",
    "acpbridge",
    "testdata",
    "wire",
    "session.json",
  );
  return JSON.parse(readFileSync(path, "utf8")) as AgentStateEnvelope;
}

/** modelConfigOption is the agent's own option, with the model now in use. */
function modelConfigOption(current: string) {
  return {
    id: "model",
    name: "Model",
    category: "model",
    currentValue: current,
    type: "select",
    options: [
      { value: "openrouter/meta/muse-spark-open", name: "openrouter/meta/muse-spark-open" },
      { value: "opencode/deepseek-v4.1-flash-free", name: "opencode/deepseek-v4.1-flash-free" },
      { value: "opencode/big-pickle", name: "opencode/big-pickle" },
    ],
  };
}

/** mockViewer answers the viewer's endpoints as the real one does. */
function mockViewer(
  options: {
    activated?: boolean;
    error?: string;
    command?: string[];
    commandError?: string;
  } = {},
) {
  const calls: { path: string; init?: RequestInit }[] = [];
  const command = options.command ?? ["opencode", "acp"];
  const fetchMock = vi.fn(async (path: string, init?: RequestInit) => {
    calls.push({ path, init });
    if (path === AGENT_STATE_PATH) {
      return new Response(
        JSON.stringify({
          activated: options.activated ?? false,
          connected: options.activated ?? false,
          status: options.activated ? "active" : "stopped",
          command,
          error: options.error,
        }),
        { status: 200 },
      );
    }
    if (path === AGENT_ACTIVATE_PATH) {
      if (options.error)
        return new Response(JSON.stringify({ error: options.error }), { status: 502 });
      return new Response(
        JSON.stringify({
          activated: true,
          connected: true,
          status: "active",
          session: "sess-1",
          command,
        }),
        { status: 200 },
      );
    }
    if (path === AGENT_COMMAND_PATH) {
      if (options.commandError)
        return new Response(JSON.stringify({ error: options.commandError }), { status: 502 });
      return new Response(JSON.stringify({ ok: true }), { status: 200 });
    }
    return new Response(JSON.stringify({}), { status: 200 });
  });
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("EventSource", FakeEventSource);
  return calls;
}

function commands(calls: { path: string; init?: RequestInit }[]): unknown[] {
  return calls
    .filter((call) => call.path === AGENT_COMMAND_PATH)
    .map((call) => JSON.parse(String(call.init?.body)));
}

/**
 * open opens the panel and asks for a start, which is the only thing that makes the stream
 * open: the agent is the one the project declared, and the reader asks for it rather than
 * naming it.
 */
async function open() {
  render(<AgentPanel open onOpenChange={vi.fn()} />);
  await userEvent.click(await screen.findByRole("button", { name: "Start the agent" }));
  await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
  return FakeEventSource.instances[0];
}

afterEach(() => {
  cleanup();
  FakeEventSource.instances = [];
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("AgentPanel", () => {
  it("is inert while it is closed: no state read, no stream", () => {
    const calls = mockViewer();
    render(<AgentPanel open={false} onOpenChange={vi.fn()} />);
    expect(calls).toHaveLength(0);
    expect(FakeEventSource.instances).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "Start the agent" })).toBeNull();
  });

  it("shows the command the project declared and starts nothing until it is asked", async () => {
    const calls = mockViewer();
    render(<AgentPanel open onOpenChange={vi.fn()} />);

    expect(await screen.findByRole("button", { name: "Start the agent" })).toBeTruthy();
    expect(calls[0].path).toBe(AGENT_STATE_PATH);
    expect(calls[0].init?.method).toBe("GET");
    // What will run is on screen, and the reader never types it.
    expect(screen.getByText("opencode acp")).toBeTruthy();
    expect(FakeEventSource.instances).toHaveLength(0);
    expect(calls.filter((call) => call.path === AGENT_ACTIVATE_PATH)).toHaveLength(0);
  });

  it("names the .sdt.yaml key when the project declares no agent, and offers no start", async () => {
    mockViewer({ command: [] });
    render(<AgentPanel open onOpenChange={vi.fn()} />);

    expect(await screen.findByText(/declares no agent/)).toBeTruthy();
    expect(screen.getByText("agent.command")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Start the agent" })).toBeNull();
  });

  it("shows a refused turn as the agent's own sentence, on the message that was sent", async () => {
    mockViewer({ activated: true });
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const source = FakeEventSource.instances[0];
    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [
            {
              id: "user-1",
              role: "user",
              parts: [
                { kind: "text", text: "how big is the corpus?" },
                { kind: "error", text: "This request requires more credits, or fewer max_tokens" },
              ],
            },
          ],
        }),
      ),
    });

    // Not a silence the reader has to interpret: the agent's words, exactly as it said them.
    expect(await screen.findByText("how big is the corpus?")).toBeTruthy();
    expect(
      screen.getByText("This request requires more credits, or fewer max_tokens"),
    ).toBeTruthy();
  });

  it("shows a message with no answer yet as waiting, and the outcome takes the wait away", async () => {
    mockViewer({ activated: true });
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const source = FakeEventSource.instances[0];
    const message = {
      id: "user-1",
      role: "user",
      parts: [{ kind: "text", text: "how big is the corpus?" }],
    };
    source.emit("message", {
      data: JSON.stringify(state({ messages: [{ ...message, provisional: true }] })),
    });
    expect(await screen.findByText("waiting for the agent…")).toBeTruthy();

    // The turn stops before it answers: the wait goes, the reason stays.
    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [
            { ...message, parts: [...message.parts, { kind: "stop", text: "max_tokens" }] },
          ],
        }),
      ),
    });
    await waitFor(() => expect(screen.queryByText("waiting for the agent…")).toBeNull());
    expect(screen.getByText("the agent stopped: max_tokens")).toBeTruthy();
  });

  it("shows a restart in flight with the thread still on screen", async () => {
    const calls = mockViewer({ activated: true });
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const source = FakeEventSource.instances[0];
    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [
            { id: "msg-1", role: "assistant", parts: [{ kind: "text", text: "still here" }] },
          ],
        }),
      ),
    });
    expect(await screen.findByText("still here")).toBeTruthy();
    void calls;

    // The bridge marks a restart; the reader sees the interruption, not an empty panel.
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              activated: true,
              connected: true,
              status: "restarting",
              command: ["opencode", "acp"],
            }),
            { status: 200 },
          ),
      ),
    );
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    expect(await screen.findByText(/starting it again/)).toBeTruthy();
    expect(screen.getByText("still here")).toBeTruthy();
  });

  it("offers a start again when a start fails, and shows the viewer's own sentence", async () => {
    mockViewer({ error: "the declared agent could not be restarted" });
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    expect(await screen.findByText("the declared agent could not be restarted")).toBeTruthy();
  });

  it("opens the stream only after a start, then renders what it sends and posts the commands", async () => {
    const calls = mockViewer();
    const source = await open();
    expect(source.url).toBe(AGENT_STREAM_PATH);

    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [
            {
              id: "msg-1",
              role: "user",
              parts: [{ kind: "text", text: "how big is the corpus?" }],
            },
            { id: "msg-2", role: "assistant", parts: [{ kind: "text", text: "224 analyses." }] },
          ],
        }),
      ),
    });
    expect(await screen.findByText("224 analyses.")).toBeTruthy();
    expect(screen.getByText("how big is the corpus?")).toBeTruthy();

    await userEvent.type(screen.getByLabelText("Message"), "and the plans?");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(commands(calls).at(-1)).toEqual({
      type: "prompt",
      text: "and the plans?",
      sessionId: "sess-1",
    });

    await userEvent.click(screen.getByRole("button", { name: "Stop" }));
    expect(commands(calls).at(-1)).toEqual({ type: "cancel", sessionId: "sess-1" });
  });

  it("resolves a permission card in place with the answer the user gives", async () => {
    const calls = mockViewer();
    const source = await open();

    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [
            {
              id: "msg-1",
              role: "assistant",
              parts: [
                {
                  kind: "permission",
                  permission: {
                    requestId: "perm-1",
                    sessionId: "sess-1",
                    title: "Write context/notes/x.md",
                    toolKind: "edit",
                  },
                },
              ],
            },
          ],
        }),
      ),
    });
    expect(await screen.findByText(/Write context\/notes\/x.md/)).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Allow" }));
    expect(commands(calls).at(-1)).toEqual({
      type: "permission",
      requestId: "perm-1",
      allow: true,
      sessionId: "sess-1",
    });

    // The bridge records the decision on the card, and the buttons give way to it.
    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [
            {
              id: "msg-1",
              role: "assistant",
              parts: [
                {
                  kind: "permission",
                  permission: {
                    requestId: "perm-1",
                    sessionId: "sess-1",
                    title: "Write context/notes/x.md",
                    toolKind: "edit",
                    decision: "allowed",
                  },
                },
              ],
            },
          ],
        }),
      ),
    });
    expect(await screen.findByText("allowed")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Allow" })).toBeNull();
  });

  it("shows the model the session is on and posts the reader's choice from the agent's list", async () => {
    const calls = mockViewer();
    const source = await open();
    source.emit("message", { data: JSON.stringify(fixtureThread()) });

    // The control is built from the wire fixture's own option and shows the value the agent
    // says is in use — the panel has no copy of the truth to disagree with.
    const control = (await screen.findByRole("combobox", {
      name: "Model",
    })) as HTMLInputElement;
    expect(control.value).toBe("openrouter/meta/muse-spark-open");

    await userEvent.click(control);
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.click(await screen.findByRole("option", { name: "opencode/big-pickle" }));
    expect(commands(calls).at(-1)).toEqual({
      type: "config",
      configId: "model",
      value: "opencode/big-pickle",
      sessionId: "sess-1",
    });

    // Nothing moved: the choice is a request, and the agent's own answer is what changes the
    // control. A local guess would show a model the session is not on.
    expect(control.value).toBe("openrouter/meta/muse-spark-open");

    source.emit("message", {
      data: JSON.stringify(state({ config: [modelConfigOption("opencode/big-pickle")] })),
    });
    await waitFor(() => expect(control.value).toBe("opencode/big-pickle"));
  });

  it("shows the agent's refusal where the control is, and leaves the session on its model", async () => {
    const calls = mockViewer({ commandError: "the provider has no such model" });
    const source = await open();
    source.emit("message", { data: JSON.stringify(fixtureThread()) });
    const control = (await screen.findByRole("combobox", {
      name: "Model",
    })) as HTMLInputElement;

    await userEvent.click(control);
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.click(
      await screen.findByRole("option", { name: "opencode/deepseek-v4.1-flash-free" }),
    );

    // The sentence the viewer sent back, above the control that caused it, and the model the
    // agent says is in use is untouched.
    expect(await screen.findByText("the provider has no such model")).toBeTruthy();
    expect(control.value).toBe("openrouter/meta/muse-spark-open");

    // A refused choice is not the end of the conversation: the next turn is posted as usual.
    await userEvent.type(screen.getByLabelText("Message"), "still there?");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(commands(calls).at(-1)).toEqual({
      type: "prompt",
      text: "still there?",
      sessionId: "sess-1",
    });
  });

  it("has no model control when the agent offers no model option", async () => {
    mockViewer();
    const source = await open();
    source.emit("message", {
      data: JSON.stringify(
        state({
          messages: [{ id: "msg-1", role: "assistant", parts: [{ kind: "text", text: "ready" }] }],
        }),
      ),
    });

    // Exactly the panel that was there before: no box to try, and the rest is unchanged.
    expect(await screen.findByText("ready")).toBeTruthy();
    expect(screen.queryByRole("combobox", { name: "Model" })).toBeNull();
    expect(screen.getByLabelText("Message")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Send" })).toBeTruthy();
  });

  it("re-reads the bridge when a stream ends instead of pretending to be connected", async () => {
    const calls = mockViewer({ activated: true });
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const reads = calls.filter((call) => call.path === AGENT_STATE_PATH).length;

    FakeEventSource.instances[0].emit("error", {});
    // The bridge is the one that knows why the stream ended, so the panel asks it rather than
    // inventing a reason, and it does not open a second stream on its own.
    await waitFor(() =>
      expect(calls.filter((call) => call.path === AGENT_STATE_PATH).length).toBeGreaterThan(reads),
    );
    expect(FakeEventSource.instances).toHaveLength(1);
  });
});
