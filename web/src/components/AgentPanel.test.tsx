// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { AgentPanel } from "./AgentPanel";
import {
  DEFAULT_PANEL_WIDTH,
  MAX_PANEL_WIDTH,
  MIN_PANEL_WIDTH,
  PANEL_WIDTH_KEY,
} from "../lib/agentPanelWidth";
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
  localStorage.clear();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

// jsdom takes the pointer events but not pointer capture, which the handle asks for when a
// drag starts; a real browser always has it.
beforeAll(() => {
  Element.prototype.setPointerCapture = vi.fn();
  Element.prototype.releasePointerCapture = vi.fn();
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

  it("takes the posture from the stream: a restart is reported without a poll", async () => {
    const calls = mockViewer({ activated: true });
    render(<AgentPanel open onOpenChange={vi.fn()} />);
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const source = FakeEventSource.instances[0];
    const readsBefore = calls.filter((call) => call.path === AGENT_STATE_PATH).length;

    source.emit("message", {
      data: JSON.stringify({
        type: "status",
        activated: true,
        connected: false,
        status: "restarting",
        sessionId: "sess-1",
        error: "could not put the agent back on its conversation",
      }),
    });

    // The sentence and the phase come from the frame; nothing asked the viewer.
    expect(
      await screen.findByText(/could not put the agent back on its conversation/),
    ).toBeTruthy();
    expect(calls.filter((call) => call.path === AGENT_STATE_PATH)).toHaveLength(readsBefore);
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

  /**
   * The panel's width is the one preference it has. It is read from the store on mount and
   * written back at the end of a gesture — not per pointer move — and the stylesheet bounds
   * what is painted from the same three numbers the preference is clamped to.
   */
  describe("the panel's width", () => {
    const handle = () => screen.getByRole("separator", { name: "Resize the agent panel" });
    const panel = () => screen.getByRole("dialog", { name: "Agent chat" });
    const width = () => Number(handle().getAttribute("aria-valuenow"));

    it("opens at the remembered width, and at the default when nothing is remembered", () => {
      mockViewer();
      const first = render(<AgentPanel open onOpenChange={vi.fn()} />);
      expect(width()).toBe(DEFAULT_PANEL_WIDTH);
      expect(panel().style.getPropertyValue("--agent-panel-w")).toBe(`${DEFAULT_PANEL_WIDTH}px`);
      first.unmount();

      localStorage.setItem(PANEL_WIDTH_KEY, "720");
      render(<AgentPanel open onOpenChange={vi.fn()} />);
      expect(width()).toBe(720);
    });

    it("drags the edge to a new width and remembers where the reader left it", () => {
      mockViewer();
      render(<AgentPanel open onOpenChange={vi.fn()} />);

      fireEvent.pointerDown(handle(), { pointerId: 1, clientX: 800 });
      expect(handle().hasAttribute("data-resizing")).toBe(true);
      fireEvent.pointerMove(handle(), { pointerId: 1, clientX: 640 });
      expect(width()).toBe(760);
      // Nothing is written while the pointer is down: the reader is still choosing.
      expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBeNull();

      fireEvent.pointerUp(handle(), { pointerId: 1, clientX: 640 });
      expect(handle().hasAttribute("data-resizing")).toBe(false);
      expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBe("760");
    });

    it("steps the width from the keyboard and remembers the step", () => {
      mockViewer();
      render(<AgentPanel open onOpenChange={vi.fn()} />);

      fireEvent.keyDown(handle(), { key: "ArrowLeft" });
      expect(width()).toBe(DEFAULT_PANEL_WIDTH + 24);
      fireEvent.keyDown(handle(), { key: "ArrowLeft", shiftKey: true });
      expect(width()).toBe(DEFAULT_PANEL_WIDTH + 32);
      expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBe(String(DEFAULT_PANEL_WIDTH + 32));

      fireEvent.keyDown(handle(), { key: "ArrowRight" });
      expect(width()).toBe(DEFAULT_PANEL_WIDTH + 8);

      fireEvent.keyDown(handle(), { key: "Home" });
      expect(width()).toBe(DEFAULT_PANEL_WIDTH);
    });

    it("bounds the width at both ends, however far the pointer travels", () => {
      mockViewer();
      render(<AgentPanel open onOpenChange={vi.fn()} />);

      fireEvent.pointerDown(handle(), { pointerId: 1, clientX: 800 });
      fireEvent.pointerMove(handle(), { pointerId: 1, clientX: 5000 });
      expect(width()).toBe(MIN_PANEL_WIDTH);

      fireEvent.pointerMove(handle(), { pointerId: 1, clientX: -5000 });
      expect(width()).toBe(MAX_PANEL_WIDTH);

      fireEvent.pointerUp(handle(), { pointerId: 1, clientX: -5000 });
      expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBe(String(MAX_PANEL_WIDTH));
    });

    it("leaves an unrelated key to the page", () => {
      mockViewer();
      render(<AgentPanel open onOpenChange={vi.fn()} />);

      fireEvent.keyDown(handle(), { key: "a" });
      expect(width()).toBe(DEFAULT_PANEL_WIDTH);
      expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBeNull();
    });
  });

  /**
   * Dismissing the panel is closing a surface, not ending anything: the agent, the session
   * and a turn in flight are the bridge's, and the panel only stops being looked at.
   */
  describe("dismissal", () => {
    const backdrop = () => document.querySelector(".agent-panel__overlay") as HTMLElement;

    it("closes on a press on the backdrop, on Escape and on its own button", async () => {
      mockViewer();
      const onOpenChange = vi.fn();
      render(<AgentPanel open onOpenChange={onOpenChange} />);
      await screen.findByRole("button", { name: "Start the agent" });

      fireEvent.pointerDown(backdrop(), { pointerId: 1, clientX: 5, clientY: 5 });
      fireEvent.pointerUp(backdrop(), { pointerId: 1, clientX: 5, clientY: 5 });
      fireEvent.click(backdrop(), { detail: 1 });
      await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));

      onOpenChange.mockClear();
      await userEvent.keyboard("{Escape}");
      await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));

      onOpenChange.mockClear();
      await userEvent.click(screen.getByRole("button", { name: "Close the agent panel" }));
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });

    it("closes with the app's icon control, named for assistive tech", async () => {
      mockViewer();
      render(<AgentPanel open onOpenChange={vi.fn()} />);
      const close = await screen.findByRole("button", { name: "Close the agent panel" });

      // The glyph every other surface closes with, decorative because the button is named.
      const icon = close.querySelector(".ms-icon");
      expect(icon?.textContent).toBe("close");
      expect(icon?.getAttribute("aria-hidden")).toBe("true");
      expect(close.classList.contains("agent-panel__icon-button")).toBe(true);
    });

    it("leaves the agent, the session and the turn alone when it closes", async () => {
      const calls = mockViewer();
      const onOpenChange = vi.fn();
      render(<AgentPanel open onOpenChange={onOpenChange} />);
      await userEvent.click(await screen.findByRole("button", { name: "Start the agent" }));
      await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
      const source = FakeEventSource.instances[0];
      source.emit("message", {
        data: JSON.stringify(
          state({
            messages: [
              { id: "msg-1", role: "assistant", parts: [{ kind: "text", text: "still working" }] },
            ],
          }),
        ),
      });
      expect(await screen.findByText("still working")).toBeTruthy();

      const posted = commands(calls).length;
      fireEvent.pointerDown(backdrop(), { pointerId: 1, clientX: 5, clientY: 5 });
      fireEvent.pointerUp(backdrop(), { pointerId: 1, clientX: 5, clientY: 5 });
      fireEvent.click(backdrop(), { detail: 1 });
      await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));

      // The stream is still the bridge's, nothing was posted, and the thread is where the
      // reader left it: closing the surface stopped nothing.
      expect(source.closed).toBe(false);
      expect(commands(calls)).toHaveLength(posted);
      expect(screen.getByText("still working")).toBeTruthy();
    });
  });

  /**
   * The controls declare what they are: the accent belongs to the answer the panel wants, a
   * secondary control is readable but not competing, and Stop is quiet. The class is the
   * contract — a test pins it so a later restyle cannot silently flatten the hierarchy.
   */
  describe("control hierarchy", () => {
    it("gives Send the accent and Stop the quiet treatment beside it", async () => {
      mockViewer();
      await open();

      const send = screen.getByRole("button", { name: "Send" });
      const stop = screen.getByRole("button", { name: "Stop" });
      expect(send.className).toContain("agent-panel__action");
      expect(send.className).not.toContain("agent-panel__action--");
      expect(stop.className).toContain("agent-panel__action--quiet");
    });

    it("keeps the first start control primary and the retry secondary", async () => {
      mockViewer();
      render(<AgentPanel open onOpenChange={vi.fn()} />);
      const start = await screen.findByRole("button", { name: "Start the agent" });
      expect(start.className).not.toContain("agent-panel__action--");

      cleanup();
      mockViewer({ error: "the agent exited" });
      render(<AgentPanel open onOpenChange={vi.fn()} />);
      const retry = await screen.findByRole("button", {
        name: /Start the agent again|Read the bridge state again/,
      });
      expect(retry.className).toContain("agent-panel__action--secondary");
    });
  });
});
