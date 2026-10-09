import { useSyncExternalStore } from "react";

/**
 * The reactive store primitive the app's module-level stores share (W2 of the
 * viewer-refactoring wave). One value, one listener set and a
 * `useSyncExternalStore` hook — replacing the copy-pasted boilerplate each store
 * carried by hand. Persistence stays with the caller (via `save`), so a store
 * keeps its own storage shape and reset semantics.
 */
export interface Store<T> {
  /** Current value, without subscribing. */
  get(): T;
  /** Replace the value; emits only when it actually changes. */
  set(next: T): void;
  /** Emit without changing the value (identity-preserving updates). */
  notify(): void;
  /** Reset to the seed (or `seed`) and emit; persists via `save`. */
  reset(seed?: T): void;
  subscribe(listener: () => void): () => void;
  /** Subscribe as a React hook; an optional selector narrows the value. */
  use<R = T>(selector?: (value: T) => R): R;
}

export interface StoreOptions<T> {
  /** Initial value, or a lazy initializer run once (storage seeding). */
  initial: T | (() => T);
  /** Persist on every change (best effort; storage errors are the caller's). */
  save?: (value: T) => void;
  /** Value equality used to skip a no-op `set` (default `Object.is`). */
  equal?: (a: T, b: T) => boolean;
}

export function createStore<T>(options: StoreOptions<T>): Store<T> {
  const seed = (): T =>
    typeof options.initial === "function" ? (options.initial as () => T)() : options.initial;
  const equal = options.equal ?? Object.is;
  let current = seed();
  const listeners = new Set<() => void>();

  const emit = (): void => {
    for (const listener of listeners) listener();
  };

  const get = (): T => current;

  const set = (next: T): void => {
    if (equal(current, next)) return;
    current = next;
    options.save?.(current);
    emit();
  };

  const reset = (value?: T): void => {
    current = value === undefined ? seed() : value;
    options.save?.(current);
    emit();
  };

  const subscribe = (listener: () => void): (() => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  };

  const use = <R = T>(selector?: (value: T) => R): R => {
    const getSnapshot = (): R => (selector ? selector(current) : (current as unknown as R));
    return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  };

  return { get, set, notify: emit, reset, subscribe, use };
}
