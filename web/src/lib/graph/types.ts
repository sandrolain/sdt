/**
 * Shared input types for the ported graph engine.
 *
 * The engine is deliberately generic: it accepts any object with a stable `id`
 * plus optional presentation fields, exactly like the reference component. SDT's
 * typed graph is mapped onto this shape by `adapter.ts`.
 */

export interface GraphNodeInput {
  id: string;
  label?: string;
  group?: string;
  size?: number;
  color?: string;
  description?: string;
  [key: string]: unknown;
}

export interface GraphLinkInput {
  source: string;
  target: string;
  type?: string;
  label?: string;
  weight?: number;
  color?: string;
  description?: string;
  [key: string]: unknown;
}

export type GraphMode = "2d" | "3d";

/** Engine layouts: the reference's three plus SDT's two. `projection` is a
 *  static layout driven by the semantic map's 2D PCA coordinates (carried on the
 *  node inputs as `x`/`y`); it is not offered in the Layout selector. */
export type GraphLayout = "force" | "groups" | "radial" | "hierarchy" | "circular" | "projection";

/** Label visibility policy (mirrors the reference's `labels` prop). */
export type LabelMode = "auto" | "always" | "hover" | "none";
