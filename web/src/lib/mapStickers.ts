/**
 * The closed sticker vocabulary of the map dialect (plan D5) and its Material
 * Symbols ligature. `[!name]` is SDT's own syntax — XMindMark has no sticker —
 * so the vocabulary lives here with the renderer that draws it; an unknown name
 * still renders, as a text chip, so a typo is visible instead of silent.
 */

export const STICKER_VOCABULARY = [
  "star",
  "flag",
  "idea",
  "done",
  "risk",
  "question",
  "goal",
  "time",
  "data",
  "priority",
] as const;

export type StickerName = (typeof STICKER_VOCABULARY)[number];

const STICKER_ICONS: Record<StickerName, string> = {
  star: "star",
  flag: "flag",
  idea: "lightbulb",
  done: "check_circle",
  risk: "warning",
  question: "help",
  goal: "emoji_events",
  time: "schedule",
  data: "database",
  priority: "priority_high",
};

/** The icon for a marker name; an unknown name has no icon and falls back. */
export function stickerIcon(name: string): string {
  return STICKER_ICONS[name as StickerName] ?? "bookmark";
}

/** Whether a name is part of the documented vocabulary. */
export function isKnownSticker(name: string): boolean {
  return name in STICKER_ICONS;
}
