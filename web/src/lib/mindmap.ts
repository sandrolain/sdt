import { Transformer } from "markmap-lib/no-plugins";
import { stripFrontmatter } from "./outline";
import { rewriteDocLinks, rewriteWikiLinks, type WikiIndex } from "./wikiLinks";

/** Structural node shape shared by markmap's IPureNode/INode and our trees. */
export interface MindNode {
  content: string;
  children: MindNode[];
  payload?: unknown;
  state?: { rect?: { x: number; y: number; width: number; height: number } };
}

/** markmap-lib without built-in plugins (no KaTeX/prism), matching our flags. */
export const transformer = new Transformer([]);

export interface MindmapOptions {
  basePath?: string;
  wikiIndex?: WikiIndex;
}

/** Prepare page markdown for markmap: strip frontmatter, rewrite links. */
export function markmapMarkdown(md: string, opts: MindmapOptions = {}): string {
  const body = stripFrontmatter(md);
  return rewriteDocLinks(rewriteWikiLinks(body, opts.wikiIndex), opts.basePath);
}

/** Transform page markdown into a markmap tree. */
export function transformMindmap(md: string, opts: MindmapOptions = {}): MindNode {
  return transformer.transform(markmapMarkdown(md, opts)).root as unknown as MindNode;
}
