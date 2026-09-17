/**
 * Frontend feature flags for the heavy markdown renderers. KaTeX is enabled by
 * round 9 (lazy chunk, no sanitiser widening); Mermaid follows in the same
 * round. See analysis 20260917-150443 and the "Markdown safety" row of
 * analysis 20260911-204414.
 */
export const FEATURES = {
  katex: true,
  mermaid: true,
} as const;

export type FeatureName = keyof typeof FEATURES;
