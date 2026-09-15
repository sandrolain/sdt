import type { MindNode } from "./mindmap";
import { nodeText } from "./boundaries";

export interface MapRef {
  id: string;
  title: string;
  root: MindNode;
}

export interface FuseOptions {
  maxDepth: number;
  maxNodes: number;
}

export const DEFAULT_FUSE_OPTIONS: FuseOptions = { maxDepth: 2, maxNodes: 200 };

export interface FuseStats {
  /** referenced maps actually imported */
  imported: string[];
  /** referenced maps skipped because already imported */
  deduped: string[];
  /** referenced maps skipped to break a cycle */
  cycles: string[];
  nodes: number;
  truncated: boolean;
}

export interface FuseResult {
  root: MindNode;
  stats: FuseStats;
}

const HREF_RE = /href="#\/(?:wiki|docs)\/([^"#?]+)(?:\?[^"]*)?"/g;

/** Map ids referenced by links in a single node's content. */
function refsInContent(content: string, index: Map<string, { id: string }>): string[] {
  const found: string[] = [];
  HREF_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = HREF_RE.exec(content)) !== null) {
    const key = m[1];
    const ref = index.get(key) ?? index.get(key.replace(/\.md$/, ""));
    if (ref) found.push(ref.id);
  }
  return found;
}

/** Map ids referenced by `#/wiki/<id>` / `#/docs/<path>` links in a tree. */
export function extractMapRefs(root: MindNode, index: Map<string, { id: string }>): string[] {
  const found = new Set<string>();
  const visit = (node: MindNode) => {
    for (const id of refsInContent(node.content, index)) found.add(id);
    node.children.forEach(visit);
  };
  visit(root);
  return [...found];
}

/** A boundary-labeled wrapper node for an imported map. */
function importNode(ref: MapRef): MindNode {
  return {
    content: `<strong>[${ref.title}]</strong>`,
    children: ref.root.children.map((c) => c),
  };
}

/**
 * Build an ephemeral fused map: referenced maps are imported once under a
 * labeled node, with cycle detection and depth/node budgets. Nothing is
 * written back to the corpus.
 */
export function fuseMap(
  baseId: string,
  root: MindNode,
  index: Map<string, MapRef>,
  options: FuseOptions = DEFAULT_FUSE_OPTIONS,
): FuseResult {
  const visited = new Set<string>([baseId]);
  const stats: FuseStats = { imported: [], deduped: [], cycles: [], nodes: 0, truncated: false };
  let count = 0;

  const clone = (node: MindNode, importDepth: number): MindNode => {
    count += 1;
    // children stay at the same import level; only imports deepen the budget
    const children = node.children.map((c) =>
      count < options.maxNodes ? clone(c, importDepth) : c,
    );
    if (count >= options.maxNodes) stats.truncated = true;
    if (importDepth < options.maxDepth) {
      for (const refId of refsInContent(node.content, index)) {
        if (visited.has(refId)) {
          if (stats.imported.includes(refId)) stats.deduped.push(refId);
          else stats.cycles.push(refId);
          continue;
        }
        const ref = index.get(refId);
        if (!ref || count >= options.maxNodes) {
          if (ref && count >= options.maxNodes) stats.truncated = true;
          continue;
        }
        visited.add(refId);
        stats.imported.push(refId);
        children.push(clone(importNode(ref), importDepth + 1));
      }
    }
    return { ...node, children };
  };

  const fused = clone(root, 0);
  stats.nodes = count;
  return { root: fused, stats };
}

/** Collect the top-level text of a tree, for the fused-map legend. */
export function topLevelTexts(root: MindNode): string[] {
  return root.children.map((c) => nodeText(c));
}
