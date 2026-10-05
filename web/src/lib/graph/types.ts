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

/** Engine layouts: the reference's three plus SDT's two. */
export type GraphLayout = "force" | "groups" | "radial" | "hierarchy" | "circular";

/** Label visibility policy (mirrors the reference's `labels` prop). */
export type LabelMode = "auto" | "always" | "hover" | "none";
