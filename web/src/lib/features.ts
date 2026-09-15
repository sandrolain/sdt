/**
 * Frontend feature flags. KaTeX and Mermaid stay off until a bundle/security
 * review clears them (analysis 20260911-204414, "Markdown safety" row).
 */
export const FEATURES = {
  katex: false,
  mermaid: false,
} as const;

export type FeatureName = keyof typeof FEATURES;
