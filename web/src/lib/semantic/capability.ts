/**
 * Control capability on the docs semantic map: which control can change the render
 * in which mode, and why the rest cannot.
 *
 * It is the single source for every disable/hide decision on that surface
 * (analysis 20261013-063009, D1). The defect it fixes is a conditional that was
 * never written: the Cluster select and the Edge-kinds legend are dead in every
 * mode, and Projection additionally kills Layout, the Relations filter and the
 * whole Explore group — silently, until a reader wonders why nothing moved.
 *
 * The wiki graph has no entry here. It has no semantic mode, so `null` means "every
 * control acts", which is what keeps this table from touching its behaviour.
 */
import type { SemanticMode } from "./graph";

/** The docs map's controls whose usefulness depends on the mode. */
export type SemanticCapability = "layout" | "clusterKey" | "relations" | "edgeKinds" | "explore";

interface Rule {
  /** The modes in which the control changes the render. */
  acts: SemanticMode[];
  /** Shown but inert: the reason a reader is given. */
  reason: string;
  /**
   * Never usable on this surface, whatever the mode. Hidden rather than greyed, the
   * precedent `BoardControls` set for the 3D-only Layers section: a control that can
   * never act is noise, while a control that acts in some modes and not others must
   * stay visible so the sidebar does not jump as modes change.
   */
  hidden?: boolean;
}

const RULES: Record<SemanticCapability, Rule> = {
  layout: {
    acts: ["edges", "clusters"],
    reason: "Positions come from the projection in this mode",
  },
  clusterKey: {
    acts: [],
    reason: "This map groups by semantic cluster, not by this key",
    hidden: true,
  },
  relations: {
    acts: ["edges", "clusters", "combined"],
    reason: "Projection keeps no links to filter",
  },
  edgeKinds: {
    acts: [],
    reason: "",
    hidden: true,
  },
  explore: {
    acts: ["edges", "clusters", "combined"],
    reason: "Projection keeps no links to explore",
  },
};

/**
 * True when the control changes the render in `mode`. The wiki graph passes `null`
 * and gets `true` for everything.
 */
export function semanticCapabilityActs(
  type: SemanticCapability,
  mode: SemanticMode | null,
): boolean {
  if (mode === null) return true;
  return RULES[type].acts.includes(mode);
}

/**
 * True when the control is not rendered at all: it can never act on this surface,
 * so greying it would only advertise a capability the map does not have.
 */
export function semanticCapabilityHidden(
  type: SemanticCapability,
  mode: SemanticMode | null,
): boolean {
  if (mode === null) return false;
  return RULES[type].hidden === true && !semanticCapabilityActs(type, mode);
}

/**
 * The reason to show beside a control that is inert in `mode`, or `null` when the
 * control acts. A hidden control needs no reason: it is not there to explain.
 */
export function semanticCapabilityReason(
  type: SemanticCapability,
  mode: SemanticMode | null,
): string | null {
  if (semanticCapabilityActs(type, mode)) return null;
  return RULES[type].reason;
}

/**
 * The label a filter row carries. One row, `semantic`: the docs map's edges are
 * proximity between documents, never an authored relation, and saying `semantic`
 * leaves the reader to guess which of the two vocabularies they are looking at.
 */
export function semanticVerbLabel(verb: string): string {
  return verb === "semantic" ? "Semantic proximity" : verb;
}
