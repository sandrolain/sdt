/**
 * A recording stand-in for a canvas 2D context, for the jsdom tests.
 *
 * jsdom has no 2D context, and the variants cannot be asserted on pixels here; what a test
 * can prove is that a state paints, that a theme repaints, and that the drawing surface is
 * asked for what the base promises. This module is imported only by tests, so it never
 * reaches a bundle.
 */
export interface Fake2d {
  canvas: HTMLCanvasElement;
  calls: string[];
  count(op: string): number;
}

export function fake2dCanvas(): Fake2d {
  const calls: string[] = [];
  const gradient = { addColorStop: () => {} };
  const target: Record<string, unknown> = {};
  const ctx = new Proxy(target, {
    get(_t, prop: string) {
      if (prop === "createRadialGradient" || prop === "createLinearGradient") {
        return () => {
          calls.push(prop);
          return gradient;
        };
      }
      return (...args: unknown[]) => {
        calls.push(args.length ? `${prop}(${args.length})` : prop);
      };
    },
    set(t, prop: string, value: unknown) {
      t[prop] = value;
      return true;
    },
  });
  const canvas = document.createElement("canvas");
  canvas.getContext = (() => ctx) as unknown as HTMLCanvasElement["getContext"];
  return {
    canvas,
    calls,
    count: (op) => calls.filter((c) => c === op || c.startsWith(`${op}(`)).length,
  };
}
