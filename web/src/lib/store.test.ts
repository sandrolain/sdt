// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { createStore } from "./store";

describe("createStore", () => {
  it("seeds from a constant or a lazy initializer", () => {
    expect(createStore<number>({ initial: 1 }).get()).toBe(1);
    expect(createStore<number>({ initial: () => 2 }).get()).toBe(2);
  });

  it("emits on change and skips a no-op set", () => {
    const store = createStore<number>({ initial: 0 });
    const listener = vi.fn();
    store.subscribe(listener);
    store.set(1);
    expect(store.get()).toBe(1);
    expect(listener).toHaveBeenCalledTimes(1);
    store.set(1);
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("honours a custom equality", () => {
    const store = createStore<{ a: number; b: number }>({
      initial: { a: 0, b: 0 },
      equal: (x, y) => x.a === y.a,
    });
    const listener = vi.fn();
    store.subscribe(listener);
    store.set({ a: 0, b: 9 });
    expect(listener).not.toHaveBeenCalled();
    store.set({ a: 1, b: 0 });
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("notify emits without changing the value", () => {
    const store = createStore<number>({ initial: 0 });
    const listener = vi.fn();
    store.subscribe(listener);
    store.notify();
    expect(store.get()).toBe(0);
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("persists on set and reset via save", () => {
    const save = vi.fn();
    const store = createStore<number>({ initial: 0, save });
    store.set(3);
    expect(save).toHaveBeenLastCalledWith(3);
    store.reset();
    expect(store.get()).toBe(0);
    expect(save).toHaveBeenLastCalledWith(0);
    store.reset(5);
    expect(store.get()).toBe(5);
    expect(save).toHaveBeenLastCalledWith(5);
  });

  it("unsubscribes", () => {
    const store = createStore<number>({ initial: 0 });
    const listener = vi.fn();
    const off = store.subscribe(listener);
    off();
    store.set(1);
    expect(listener).not.toHaveBeenCalled();
  });

  it("re-renders subscribers through use(), with an optional selector", () => {
    const store = createStore<{ n: number; s: string }>({ initial: { n: 0, s: "a" } });
    const { result } = renderHook(() => [store.use((v) => v.n), store.use((v) => v.s)]);
    expect(result.current).toEqual([0, "a"]);
    act(() => store.set({ n: 1, s: "b" }));
    expect(result.current).toEqual([1, "b"]);
  });
});
