// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AgentPanel } from "./AgentPanel";
import { AGENT_ACTIVATE_PATH, AGENT_STATE_PATH, type AgentStateEnvelope } from "../lib/agent";

/** FakeSocket is the browser side the panel talks to, driven by the test. */
class FakeSocket {
  static instances: FakeSocket[] = [];
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSED = 3;
  readyState = FakeSocket.CONNECTING;
  sent: string[] = [];
  listeners = new Map<string, ((event: unknown) => void)[]>();

  constructor(public url: string) {
    FakeSocket.instances.push(this);
  }

  addEventListener(kind: string, listener: (event: unknown) => void) {
    this.listeners.set(kind, [...(this.listeners.get(kind) ?? []), listener]);
  }

  emit(kind: string, event: unknown) {
    for (const listener of this.listeners.get(kind) ?? []) listener(event);
  }

  send(payload: string) {
    this.sent.push(payload);
  }

  close() {
    this.readyState = FakeSocket.CLOSED;
  }

  open() {
    this.readyState = FakeSocket.OPEN;
  }
}

function state(
  thread: Partial<AgentStateEnvelope["thread"]> = {},
  sessionId = "sess-1",
): AgentStateEnvelope {
  return { type: "state", sessionId, thread: { sessionId, messages: [], ...thread } };
}

/** mockViewer answers the three endpoints as the viewer does. */
function mockViewer(options: { activated?: boolean; error?: string } = {}) {
  const fetchMock = vi.fn(async (path: string) => {
    if (path === AGENT_STATE_PATH) {
      return new Response(
        JSON.stringify({
          activated: options.activated ?? false,
          connected: false,
          error: options.error,
        }),
        { status: 200 },
      );
    }
    if (path === AGENT_ACTIVATE_PATH) {
      if (options.error)
        return new Response(JSON.stringify({ error: options.error }), { status: 502 });
      return new Response(JSON.stringify({ activated: true, connected: true, session: "sess-1" }), {
        status: 200,
      });
    }
    return new Response(JSON.stringify({}), { status: 200 });
  });
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("WebSocket", FakeSocket);
  return fetchMock;
}

function renderPanel(open: boolean, onOpenChange = vi.fn()) {
  return render(<AgentPanel open={open} onOpenChange={onOpenChange} />);
}

afterEach(() => {
  cleanup();
  FakeSocket.instances = [];
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("AgentPanel", () => {
  it("is inert while it is closed: no state read, no socket", () => {
    const fetchMock = mockViewer();
    renderPanel(false);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(FakeSocket.instances).toHaveLength(0);
    expect(screen.queryByText(/Socket path/)).toBeNull();
  });

  it("shows the activation control with the proxy recipe on the first open", async () => {
    const fetchMock = mockViewer();
    renderPanel(true);

    expect(await screen.findByText(/The bridge is off/)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledWith(
      AGENT_STATE_PATH,
      expect.objectContaining({ method: "GET" }),
    );
    expect(screen.getByText(/socat UNIX-LISTEN/)).toBeTruthy();
    // Nothing is dialled before a submit.
    expect(FakeSocket.instances).toHaveLength(0);
  });

  it("dials only when the socket is submitted, then renders what the bridge sends", async () => {
    mockViewer();
    renderPanel(true);
    await screen.findByText(/The bridge is off/);

    await userEvent.type(screen.getByLabelText("Socket path"), "/tmp/sdt-acp.sock");
    await userEvent.click(screen.getByRole("button", { name: "Connect" }));

    await waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
    const socket = FakeSocket.instances[0];
    expect(socket.url).toContain("/api/agent/ws");

    socket.open();
    socket.emit("message", {
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

    // The composer sends the command the handler accepts, and Stop sends the cancel.
    await userEvent.type(screen.getByLabelText("Message"), "and the plans?");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(socket.sent.at(-1)).toBe(JSON.stringify({ type: "prompt", text: "and the plans?" }));

    await userEvent.click(screen.getByRole("button", { name: "Stop" }));
    expect(socket.sent.at(-1)).toBe(JSON.stringify({ type: "cancel" }));
  });

  it("resolves a permission card in place with the answer the user gives", async () => {
    mockViewer();
    renderPanel(true);
    await screen.findByText(/The bridge is off/);
    await userEvent.type(screen.getByLabelText("Socket path"), "/tmp/sdt-acp.sock");
    await userEvent.click(screen.getByRole("button", { name: "Connect" }));
    await waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
    const socket = FakeSocket.instances[0];
    socket.open();

    socket.emit("message", {
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
    expect(socket.sent.at(-1)).toBe(
      JSON.stringify({ type: "permission", requestId: "perm-1", allow: true }),
    );

    // The bridge records the decision on the card, and the buttons give way to it.
    socket.emit("message", {
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

  it("shows the viewer's own sentence when the bridge refuses", async () => {
    mockViewer({ error: "no agent is listening on that socket" });
    renderPanel(true);
    expect(await screen.findByText("no agent is listening on that socket")).toBeTruthy();
  });

  it("reports a dropped socket once instead of pretending to be connected", async () => {
    mockViewer({ activated: true });
    renderPanel(true);
    await waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
    FakeSocket.instances[0].open();
    FakeSocket.instances[0].emit("close", { reason: "the agent went away" });
    expect(await screen.findByText("the agent went away")).toBeTruthy();
  });
});
