// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { SEMANTIC_MODE_OPTIONS, type SemanticMode } from "./graph";
import {
  semanticCapabilityActs,
  semanticCapabilityHidden,
  semanticCapabilityReason,
  semanticVerbLabel,
  type SemanticCapability,
} from "./capability";

/**
 * The matrix the docs map was reported against, written out once: for each control
 * and each mode, whether it can change the render. The test asserts the table the
 * components read against this, so a mode added later cannot leave a control
 * silently live.
 */
const MATRIX: Record<SemanticCapability, Record<SemanticMode, boolean>> = {
  layout: { edges: true, projection: false, clusters: true, combined: false },
  clusterKey: { edges: false, projection: false, clusters: false, combined: false },
  relations: { edges: true, projection: false, clusters: true, combined: true },
  edgeKinds: { edges: false, projection: false, clusters: false, combined: false },
  explore: { edges: true, projection: false, clusters: true, combined: true },
};

const CAPABILITIES = Object.keys(MATRIX) as SemanticCapability[];

describe("semantic capability map", () => {
  it("covers every mode the control offers", () => {
    for (const cap of CAPABILITIES) {
      expect(Object.keys(MATRIX[cap]).sort()).toEqual(
        SEMANTIC_MODE_OPTIONS.map((m) => m.id).sort(),
      );
    }
  });

  it("reports the matrix for every capability and mode", () => {
    for (const cap of CAPABILITIES) {
      for (const mode of SEMANTIC_MODE_OPTIONS) {
        expect([cap, mode.id, semanticCapabilityActs(cap, mode.id)]).toEqual([
          cap,
          mode.id,
          MATRIX[cap][mode.id],
        ]);
      }
    }
  });

  it("gives a reason for every control that is inert in some mode, and none for the rest", () => {
    for (const cap of CAPABILITIES) {
      for (const mode of SEMANTIC_MODE_OPTIONS) {
        const reason = semanticCapabilityReason(cap, mode.id);
        const hidden = semanticCapabilityHidden(cap, mode.id);
        if (MATRIX[cap][mode.id]) {
          expect([cap, mode.id, reason]).toEqual([cap, mode.id, null]);
          expect(hidden).toBe(false);
        } else if (hidden) {
          // Hidden: it is not there to explain itself, so its reason is recorded for
          // the guide and never rendered — which is also fine when it is empty.
          expect(typeof reason).toBe("string");
        } else {
          // Disabled: a reader must be told why, in a sentence.
          expect(typeof reason).toBe("string");
          expect((reason ?? "").length).toBeGreaterThan(20);
        }
      }
      // An empty reason is only ever allowed for a control that is not rendered:
      // a disabled control with nothing to say is the silent dead end this map fixes.
      const emptyReason = SEMANTIC_MODE_OPTIONS.some(
        (m) => semanticCapabilityReason(cap, m.id) === "" && !semanticCapabilityActs(cap, m.id),
      );
      if (emptyReason) {
        expect(
          SEMANTIC_MODE_OPTIONS.every(
            (m) => !semanticCapabilityActs(cap, m.id) && semanticCapabilityHidden(cap, m.id),
          ),
        ).toBe(true);
      }
    }
  });

  it("hides a control exactly when it can never act and never on the wiki graph", () => {
    for (const cap of CAPABILITIES) {
      const everActs = SEMANTIC_MODE_OPTIONS.some((m) => MATRIX[cap][m.id]);
      for (const mode of SEMANTIC_MODE_OPTIONS) {
        expect(semanticCapabilityHidden(cap, mode.id)).toBe(!everActs);
      }
      expect(semanticCapabilityHidden(cap, null)).toBe(false);
    }
  });

  it("leaves the wiki graph every control it has today", () => {
    for (const cap of CAPABILITIES) {
      expect(semanticCapabilityActs(cap, null)).toBe(true);
      expect(semanticCapabilityReason(cap, null)).toBeNull();
    }
  });

  it("names the docs map's one edge identity", () => {
    expect(semanticVerbLabel("semantic")).toBe("Semantic proximity");
    expect(semanticVerbLabel("depends_on")).toBe("depends_on");
  });
});
