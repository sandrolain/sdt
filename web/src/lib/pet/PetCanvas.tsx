/**
 * The React face of a pet: a host element, a canvas created inside it, and the controller
 * lifecycle around them.
 *
 * Vendored from `context/refs/react/pet-react/PetCanvas.tsx` (this project's own
 * material), with comments translated and three changes the panel needs:
 *
 * - `onUnavailable` is called when the browser cannot give the canvas a context, instead of
 *   throwing out of React. The pet is the gloss on the panel, never a reason for the
 *   conversation to break.
 * - the pointer listener takes a target element, so a caller can scope the gaze to its own
 *   surface rather than to the whole window.
 * - `family` picks the factory and both factories are resolved through a dynamic import, so
 *   a 2D choice never pulls the `three` chunk and a WebGL choice never pulls the 2D one.
 */
import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import type { CSSProperties } from "react";
import type {
  Pet2DVariant,
  PetController,
  PetOptions,
  PetState,
  PetTheme,
  PetVariant,
  PetWebGLVariant,
} from "../pet/types";

/** The handle exposed through the ref: always safe to call, even before the pet exists. */
export interface PetHandle {
  readonly state: PetState;
  setState(state: PetState): void;
  pulse(amount?: number): void;
  lookAt(x: number, y: number): void;
}

/** Which factory paints the variant: `webgl` for the three, `2d` for the three new ones. */
export type PetFamily = "webgl" | "2d";

export interface PetCanvasProps {
  variant?: PetVariant;
  family?: PetFamily;
  /** Declarative state. Use the ref and `setState` instead if the caller owns a clock. */
  state?: PetState;
  /** The panel's theme, for a controller that derives its ink per frame (`setTheme`). */
  theme?: PetTheme;
  options?: PetOptions;
  /** The gaze follows the pointer over this element (default: the dialog it sits in, else
   *  the host). A panel-scoped surface, never the whole window. */
  pointerTarget?: HTMLElement | null;
  /** Still frames: one settled frame per state change, for reduced motion or a resting pet. */
  still?: boolean;
  /** Paused: the loop stops without settling, e.g. while the document is hidden. */
  paused?: boolean;
  /** Whether the gaze follows the pointer at all (default true). */
  followPointer?: boolean;
  /** Called when the canvas cannot get a context: the caller hides the pet. */
  onUnavailable?: () => void;
  onStateChange?: (state: PetState) => void;
  className?: string;
  style?: CSSProperties;
  /** Alternative text for assistive technology. */
  label?: string;
}

export const PetCanvas = forwardRef<PetHandle, PetCanvasProps>(function PetCanvas(
  {
    variant = "sciame",
    family = "webgl",
    state,
    theme,
    options,
    pointerTarget,
    still = false,
    paused = false,
    followPointer = true,
    onUnavailable,
    onStateChange,
    className,
    style,
    label = "Virtual assistant",
  },
  ref,
) {
  const box = useRef<HTMLDivElement>(null);
  const pet = useRef<PetController | null>(null);
  const stateRef = useRef(state);
  stateRef.current = state;
  const themeRef = useRef(theme);
  themeRef.current = theme;
  const cbRef = useRef(onStateChange);
  cbRef.current = onStateChange;
  const unavailableRef = useRef(onUnavailable);
  unavailableRef.current = onUnavailable;
  const stillRef = useRef(still);
  stillRef.current = still;
  const pausedRef = useRef(paused);
  pausedRef.current = paused;

  useImperativeHandle(
    ref,
    () => ({
      get state() {
        return pet.current?.state ?? "idle";
      },
      setState: (s) => pet.current?.setState(s),
      pulse: (a) => pet.current?.pulse(a),
      lookAt: (x, y) => pet.current?.lookAt(x, y),
    }),
    [],
  );

  useEffect(() => {
    const host = box.current;
    if (!host) return;

    // The canvas is created here, so every mount (and every variant change) gets a fresh
    // context.
    const canvas = document.createElement("canvas");
    canvas.style.cssText = "position:absolute;inset:0;width:100%;height:100%;display:block";
    host.appendChild(canvas);

    let disposed = false;
    let teardown: (() => void) | undefined;

    const start = async () => {
      let p: PetController;
      try {
        if (family === "2d") {
          const { createPet2D } = await import("../pet2d/createPet");
          p = createPet2D(variant as Pet2DVariant, canvas, options, themeRef.current ?? "dark");
        } else {
          const { createPet } = await import("./createPet");
          p = createPet(variant as PetWebGLVariant, canvas, options);
        }
      } catch {
        // No context: a background tab, a browser without the API, a machine out of context
        // slots. The caller decides what to paint; this module does not throw through React.
        if (!disposed) {
          canvas.remove();
          pet.current = null;
          unavailableRef.current?.();
        }
        return;
      }
      if (disposed) {
        p.dispose();
        canvas.remove();
        return;
      }

      pet.current = p;
      if (stateRef.current) p.setState(stateRef.current);
      p.setStill(stillRef.current);
      p.setPaused(pausedRef.current);
      p.setTheme?.(themeRef.current ?? "dark");
      const off = p.on("stateChange", (s) => cbRef.current?.(s));

      const ro = new ResizeObserver(() => p.resize(host.clientWidth, host.clientHeight));
      ro.observe(host);
      p.resize(host.clientWidth, host.clientHeight);

      const io = new IntersectionObserver(([e]) => p.setPaused(!e.isIntersecting));
      io.observe(host);

      // The gaze is the panel's, not the page's: a cursor over the document behind the panel
      // is not attention on the pet. The dialog is resolved once, at creation.
      const surface = pointerTarget ?? host.closest('[role="dialog"]') ?? host;
      const onMove = (e: PointerEvent) => {
        const r = host.getBoundingClientRect();
        if (r.width < 1 || r.height < 1) return;
        p.lookAt(
          ((e.clientX - r.left) / r.width) * 2 - 1,
          -(((e.clientY - r.top) / r.height) * 2 - 1),
        );
      };
      if (followPointer) surface.addEventListener("pointermove", onMove);

      teardown = () => {
        surface.removeEventListener("pointermove", onMove);
        io.disconnect();
        ro.disconnect();
        off();
        p.dispose();
        pet.current = null;
        canvas.remove();
      };
    };
    void start();

    return () => {
      disposed = true;
      teardown?.();
      canvas.remove();
    };
    // Options are read only at creation: their primitive properties are the dependencies.
  }, [
    variant,
    family,
    followPointer,
    options?.density,
    options?.pixelRatioCap,
    options?.antialias,
    pointerTarget,
  ]);

  useEffect(() => {
    if (state) pet.current?.setState(state);
  }, [state]);

  useEffect(() => {
    if (theme) pet.current?.setTheme?.(theme);
  }, [theme]);

  useEffect(() => {
    pet.current?.setStill(still);
  }, [still]);

  useEffect(() => {
    pet.current?.setPaused(paused);
  }, [paused]);

  return (
    <div
      ref={box}
      className={className}
      role="img"
      aria-label={label}
      style={{ position: "relative", width: "100%", height: "100%", ...style }}
    />
  );
});
