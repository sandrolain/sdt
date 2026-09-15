import type { RelEntry, RelGroup, RelResponse } from "./api";

export type Direction = "inbound" | "outbound";

export interface RelItem {
  direction: Direction;
  verb: string;
  /** neighbor wiki id */
  id: string;
  title: string;
  label?: string;
  kind: string;
  path: string;
}

export interface RelGroupView {
  verb: string;
  inbound: RelItem[];
  outbound: RelItem[];
}

function neighborId(entry: RelEntry, direction: Direction): string {
  return (direction === "outbound" ? entry.target : entry.source) ?? "";
}

function toItems(group: RelGroup | undefined, direction: Direction): RelItem[] {
  if (!group) return [];
  const out: RelItem[] = [];
  for (const [verb, entries] of Object.entries(group)) {
    for (const entry of entries) {
      out.push({
        direction,
        verb,
        id: neighborId(entry, direction),
        title: entry.title ?? neighborId(entry, direction),
        label: entry.label,
        kind: entry.kind,
        path: entry.path,
      });
    }
  }
  return out;
}

/** Flatten the rel response into ordered, verb-grouped relation rows. */
export function groupRelations(resp: RelResponse): RelGroupView[] {
  const inbound = toItems(resp.inbound, "inbound");
  const outbound = toItems(resp.outbound, "outbound");
  const verbs = [...new Set([...outbound, ...inbound].map((i) => i.verb))].sort();
  return verbs.map((verb) => ({
    verb,
    inbound: inbound.filter((i) => i.verb === verb).sort((a, b) => a.id.localeCompare(b.id)),
    outbound: outbound.filter((i) => i.verb === verb).sort((a, b) => a.id.localeCompare(b.id)),
  }));
}

/** Compact counts for the panel header. */
export function relationCounts(resp: RelResponse): { inbound: number; outbound: number } {
  const count = (group?: RelGroup) =>
    group ? Object.values(group).reduce((n, entries) => n + entries.length, 0) : 0;
  return { inbound: count(resp.inbound), outbound: count(resp.outbound) };
}
