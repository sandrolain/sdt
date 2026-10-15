// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  AGENT_ACTIVATE_PATH,
  AGENT_DEACTIVATE_PATH,
  AGENT_STATE_PATH,
  AGENT_WS_PATH,
  agentSocketURL,
  cancelCommand,
  connectAgent,
  deactivateAgent,
  DECISION_ALLOWED,
  ENVELOPE_CANCEL,
  ENVELOPE_PERMISSION,
  ENVELOPE_PROMPT,
  ENVELOPE_STATE,
  fetchAgentState,
  activateAgent,
  permissionCommand,
  promptCommand,
  type AgentStateEnvelope,
} from "./agent";

/** FakeSocket is the browser side the module talks to, driven by the test. */
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
    this.emit("close", { reason: "closed by the panel" });
  }

  open() {
    this.readyState = FakeSocket.OPEN;
  }
}

function state(thread = {}): AgentStateEnvelope {
  return {
    type: ENVELOPE_STATE,
    sessionId: "sess-1",
    thread: { sessionId: "sess-1", messages: [], ...thread },
  };
}

afterEach(() => {
  FakeSocket.instances = [];
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the agent wire", () => {
  it("builds the three commands with the envelope names the bridge defines", () => {
    expect(promptCommand("hello")).toEqual({ type: ENVELOPE_PROMPT, text: "hello" });
    expect(cancelCommand()).toEqual({ type: ENVELOPE_CANCEL });
    expect(permissionCommand("perm-1", true)).toEqual({
      type: ENVELOPE_PERMISSION,
      requestId: "perm-1",
      allow: true,
    });
    expect(DECISION_ALLOWED).toBe("allowed");
  });

  it("derives the socket URL from the page's own origin", () => {
    expect(agentSocketURL({ protocol: "http:", host: "127.0.0.1:8791" })).toBe(
      `ws://127.0.0.1:8791${AGENT_WS_PATH}`,
    );
    expect(agentSocketURL({ protocol: "https:", host: "example.test" })).toBe(
      `wss://example.test${AGENT_WS_PATH}`,
    );
  });

  it("reads the state with a GET and activates with the socket the user typed", async () => {
    const fetchMock = vi.fn(async (path: string, init?: RequestInit) => {
      return new Response(
        JSON.stringify({ activated: init?.method === "POST", connected: false }),
        {
          status: 200,
        },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    await fetchAgentState();
    expect(fetchMock).toHaveBeenCalledWith(AGENT_STATE_PATH, {
      method: "GET",
      headers: undefined,
      body: undefined,
    });

    await activateAgent("/tmp/sdt-acp.sock");
    expect(fetchMock).toHaveBeenLastCalledWith(AGENT_ACTIVATE_PATH, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ socket: "/tmp/sdt-acp.sock" }),
    });

    await deactivateAgent();
    expect(fetchMock).toHaveBeenLastCalledWith(AGENT_DEACTIVATE_PATH, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
  });

  it("reports the viewer's own sentence when a call is refused", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ error: "no agent is listening on that socket" }), {
            status: 502,
          }),
      ),
    );
    await expect(activateAgent("/tmp/absent.sock")).rejects.toThrow(
      "no agent is listening on that socket",
    );
  });
});

describe("the agent connection", () => {
  it("hands every whole-thread state to the caller and drops what the wire does not define", () => {
    vi.stubGlobal("WebSocket", FakeSocket);
    const states: AgentStateEnvelope[] = [];
    const connection = connectAgent({
      onState: (envelope) => states.push(envelope),
      url: "ws://test",
    });
    const socket = FakeSocket.instances[0];
    expect(socket.url).toBe("ws://test");

    socket.emit("message", { data: JSON.stringify(state()) });
    socket.emit("message", { data: "not a state at all" });
    socket.emit("message", { data: JSON.stringify({ type: "something-else" }) });

    expect(states).toHaveLength(1);
    expect(states[0].sessionId).toBe("sess-1");
    connection.close();
  });

  it("sends only on an open socket", () => {
    vi.stubGlobal("WebSocket", FakeSocket);
    const connection = connectAgent({ onState: () => undefined, url: "ws://test" });
    const socket = FakeSocket.instances[0];

    connection.send(promptCommand("too early"));
    expect(socket.sent).toEqual([]);

    socket.open();
    connection.send(promptCommand("hello"));
    expect(socket.sent).toEqual([JSON.stringify({ type: ENVELOPE_PROMPT, text: "hello" })]);
  });

  it("reports a closed socket once", () => {
    vi.stubGlobal("WebSocket", FakeSocket);
    const reasons: string[] = [];
    const connection = connectAgent({
      onState: () => undefined,
      onClose: (reason) => reasons.push(reason),
      url: "ws://test",
    });
    FakeSocket.instances[0].emit("close", { reason: "the agent went away" });
    expect(reasons).toEqual(["the agent went away"]);
    connection.close();
  });
});
