/**
 * The shared semantic client: one module owns every read path to the viewer's
 * semantic surface, so the search palette and the docs map never diverge.
 */
export * from "./capability";
export * from "./graph";
export * from "./status";
