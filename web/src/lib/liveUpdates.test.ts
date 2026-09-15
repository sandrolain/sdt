// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { applyLiveChange, connectLiveUpdates, LIVE_RELOAD_EVENT } from "./liveUpdates";

type Listener = (event: MessageEvent) => void;

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners: Record<string, Listener[]> = {};
  closed = false;
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }
  addEventListener(type: string, listener: Listener) {
    (this.listeners[type] ??= []).push(listener);
  }
  removeEventListener(type: string, listener: Listener) {
    this.listeners[type] = (this.listeners[type] ?? []).filter((l) => l !== listener);
  }
  close() {
    this.closed = true;
  }
  emit(type: string, data: unknown) {
    for (const listener of this.listeners[type] ?? []) {
      listener({ data: JSON.stringify(data) } as MessageEvent);
    }
  }
}

afterEach(() => {
  FakeEventSource.instances = [];
  vi.restoreAllMocks();
});

describe("connectLiveUpdates", () => {
  it("subscribes to /api/events and forwards parsed paths", () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const onChange = vi.fn();
    const disconnect = connectLiveUpdates(onChange);
    const source = FakeEventSource.instances[0];
    expect(source.url).toBe("/api/events");

    source.emit("change", { paths: ["context/a.md"] });
    expect(onChange).toHaveBeenCalledWith({ paths: ["context/a.md"] });

    disconnect();
    expect(source.closed).toBe(true);
    vi.unstubAllGlobals();
  });

  it("ignores malformed payloads", () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const onChange = vi.fn();
    connectLiveUpdates(onChange);
    const source = FakeEventSource.instances[0];
    for (const listener of source.listeners["change"] ?? []) {
      listener({ data: "{not json" } as MessageEvent);
    }
    expect(onChange).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });
});

describe("applyLiveChange", () => {
  it("dispatches a reload event", () => {
    const onReload = vi.fn();
    window.addEventListener(LIVE_RELOAD_EVENT, onReload);
    applyLiveChange();
    expect(onReload).toHaveBeenCalledTimes(1);
    window.removeEventListener(LIVE_RELOAD_EVENT, onReload);
  });
});
