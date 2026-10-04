/**
 * Category vocabulary for the `categories` frontmatter field (analysis work
 * types). Parallel to `kinds.ts`: a stable icon + colour per register slug
 * (`context/categories.yaml`) plus a neutral fallback, because the vocabulary is
 * advisory and an unknown slug must still render.
 */

/** Material Symbols glyph for a category slug. */
const CATEGORY_ICONS: Record<string, string> = {
  bug: "bug_report",
  issue: "report",
  "new-feature": "new_releases",
  "feature-change": "published_with_changes",
  refactor: "transform",
  improvement: "trending_up",
  research: "science",
};

/** Catppuccin token (CSS var reference) for a category slug. */
const CATEGORY_COLORS: Record<string, string> = {
  bug: "var(--ctp-red)",
  issue: "var(--ctp-peach)",
  "new-feature": "var(--ctp-green)",
  "feature-change": "var(--ctp-teal)",
  refactor: "var(--ctp-mauve)",
  improvement: "var(--ctp-sapphire)",
  research: "var(--ctp-pink)",
};

const FALLBACK_ICON = "sell";
const FALLBACK_COLOR = "var(--ctp-overlay1)";

export function categoryIcon(slug: string): string {
  return CATEGORY_ICONS[slug] ?? FALLBACK_ICON;
}

export function categoryColor(slug: string): string {
  return CATEGORY_COLORS[slug] ?? FALLBACK_COLOR;
}
