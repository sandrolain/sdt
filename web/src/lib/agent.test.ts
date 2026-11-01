// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  AGENT_ACTIVATE_PATH,
  AGENT_COMMAND_PATH,
  AGENT_DEACTIVATE_PATH,
  AGENT_STATE_PATH,
  AGENT_STREAM_PATH,
  activateAgent,
  cancelCommand,
  deactivateAgent,
  ENVELOPE_CANCEL,
  ENVELOPE_PERMISSION,
  ENVELOPE_PROMPT,
  ENVELOPE_STATE,
  fetchAgentState,
  permissionCommand,
  promptCommand,
  offeredModels,
  type AgentConfigOption,
  sendAgentCommand,
  streamAgent,
  type AgentStateEnvelope,
} from "./agent";

/** FakeEventSource is the browser side the module talks to, driven by the test. */
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

function state(thread = {}): AgentStateEnvelope {
  return {
    type: ENVELOPE_STATE,
    sessionId: "sess-1",
    thread: { sessionId: "sess-1", messages: [], ...thread },
  };
}

afterEach(() => {
  FakeEventSource.instances = [];
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the agent wire", () => {
  it("builds the three commands, naming a session only when there is one", () => {
    expect(promptCommand("hello")).toEqual({ type: ENVELOPE_PROMPT, text: "hello" });
    expect(promptCommand("hello", "sess-2")).toEqual({
      type: ENVELOPE_PROMPT,
      text: "hello",
      sessionId: "sess-2",
    });
    expect(cancelCommand()).toEqual({ type: ENVELOPE_CANCEL });
    expect(cancelCommand("sess-2")).toEqual({ type: ENVELOPE_CANCEL, sessionId: "sess-2" });
    expect(permissionCommand("perm-1", "allow_once")).toEqual({
      type: ENVELOPE_PERMISSION,
      requestId: "perm-1",
      optionId: "allow_once",
    });
  });

  it("reads the state with a GET and starts the declared agent with an empty POST", async () => {
    const fetchMock = vi.fn(async (_path: string, init?: RequestInit) => {
      return new Response(
        JSON.stringify({ activated: init?.method === "POST", connected: false }),
        { status: 200 },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    await fetchAgentState();
    expect(fetchMock).toHaveBeenCalledWith(AGENT_STATE_PATH, {
      method: "GET",
      headers: undefined,
      body: undefined,
    });

    // The request carries no body at all: the command is the project's, and a body that
    // named one would be refused by the viewer rather than ignored.
    await activateAgent();
    expect(fetchMock).toHaveBeenLastCalledWith(AGENT_ACTIVATE_PATH, {
      method: "POST",
      headers: undefined,
      body: undefined,
    });

    await deactivateAgent();
    expect(fetchMock).toHaveBeenLastCalledWith(AGENT_DEACTIVATE_PATH, {
      method: "POST",
      headers: undefined,
      body: undefined,
    });
  });

  it("posts a command and reports the viewer's own sentence when it is refused", async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(JSON.stringify({ error: "the agent bridge is not activated" }), {
          status: 403,
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(sendAgentCommand(cancelCommand("sess-1"))).rejects.toThrow(
      "the agent bridge is not activated",
    );
    expect(fetchMock).toHaveBeenCalledWith(AGENT_COMMAND_PATH, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type: ENVELOPE_CANCEL, sessionId: "sess-1" }),
    });

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("{}", { status: 200 })),
    );
    await expect(sendAgentCommand(promptCommand("hi"))).resolves.toBeUndefined();
  });
});

describe("the agent stream", () => {
  it("hands every whole-thread state to the caller and drops what the wire does not define", () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const states: AgentStateEnvelope[] = [];
    const stream = streamAgent({
      onState: (envelope) => states.push(envelope),
      url: AGENT_STREAM_PATH,
    });
    const source = FakeEventSource.instances[0];
    expect(source.url).toBe(AGENT_STREAM_PATH);

    source.emit("message", { data: JSON.stringify(state()) });
    source.emit("message", { data: "not a state at all" });
    source.emit("message", { data: JSON.stringify({ type: "something-else" }) });

    expect(states).toHaveLength(1);
    expect(states[0].sessionId).toBe("sess-1");
    stream.close();
    expect(source.closed).toBe(true);
  });

  it("closes a failed stream instead of letting EventSource reconnect, and reports once", () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const reasons: string[] = [];
    streamAgent({ onState: () => undefined, onClose: (reason) => reasons.push(reason) });
    const source = FakeEventSource.instances[0];

    source.emit("error", {});

    expect(source.closed).toBe(true);
    expect(reasons).toEqual(["the agent stream ended"]);
  });
});

describe("offeredModels", () => {
  const option: AgentConfigOption = {
    id: "model",
    category: "model",
    type: "select",
    currentValue: "agent/one",
    options: [
      { value: "agent/one", name: "One" },
      { value: "agent/two", name: "Two" },
      { value: "agent/three", name: "Three" },
    ],
  };

  it("keeps the agent's whole list when the project declares nothing", () => {
    const offered = offeredModels(option, undefined, "agent/one");
    expect(offered.declared).toBe(false);
    expect(offered.matched).toBe(true);
    expect(offered.rows.map((row) => row.value)).toEqual(["agent/one", "agent/two", "agent/three"]);
  });

  it("follows the declaration's order, not the agent's", () => {
    const offered = offeredModels(option, ["agent/three", "agent/one"], "agent/one");
    expect(offered.declared).toBe(true);
    expect(offered.matched).toBe(true);
    expect(offered.rows.map((row) => row.value)).toEqual(["agent/three", "agent/one"]);
    // A row carries the agent's own name: the declaration names an id, never a label.
    expect(offered.rows[0].name).toBe("Three");
  });

  it("leaves out a declared model the agent does not offer, and collapses a repeat", () => {
    const offered = offeredModels(option, ["gone/away", "agent/two", "agent/two"], "agent/two");
    expect(offered.rows.map((row) => row.value)).toEqual(["agent/two"]);
    expect(offered.matched).toBe(true);
  });

  it("offers nothing when no declared model is one the agent offers", () => {
    const offered = offeredModels(option, ["gone/away"], "agent/one");
    expect(offered.declared).toBe(true);
    expect(offered.matched).toBe(false);
    expect(offered.rows).toEqual([]);
  });

  it("keeps the model in use visible when the declaration leaves it out", () => {
    const offered = offeredModels(option, ["agent/two"], "agent/one");
    // First, so the control shows what is running rather than an empty value.
    expect(offered.rows.map((row) => row.value)).toEqual(["agent/one", "agent/two"]);
    expect(offered.rows[0].name).toBe("One");
  });

  it("does not add the model in use when the declaration matched nothing", () => {
    // There is no picker in that case: the panel states the model in its note instead.
    expect(offeredModels(option, ["gone/away"], "agent/one").rows).toEqual([]);
  });
});
