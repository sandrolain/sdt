/**
 * Semantic availability probe: `/api/semantic/status`.
 *
 * The two capabilities are independent. `enabled` is the server's semantic
 * **search** branch (on and operational); the search palette hides its Semantic
 * toggle when it is false. `snapshot` is a persisted vector snapshot, which is
 * what the docs semantic map needs; the map is its own consumer and works while
 * `enabled` is false.
 */
import { fetchJSON } from "../fetchJson";

export interface SemanticStatus {
  /** The search branch is on and operational; hide the toggle when false. */
  enabled: boolean;
  /** A vector snapshot exists on disk (the docs map's data). */
  snapshot: boolean;
}

export function fetchSemanticStatus(): Promise<SemanticStatus> {
  return fetchJSON<SemanticStatus>("/api/semantic/status");
}
