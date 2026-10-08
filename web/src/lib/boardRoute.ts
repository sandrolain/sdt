/**
 * Board route grammar (O2, `/e/`-marked): the board source and its nested
 * drill path live in `/wiki/board/*`.
 *
 *   /wiki/board                                  → default wiki graph
 *   /wiki/board/<natural canvas path>            → a selected `.canvas`
 *   /wiki/board/<path>/e/<nodeId>/e/<nodeId>…    → each entered nested level
 *
 * The canvas path keeps its natural slashes; each drill node id is one
 * `encodeURIComponent`-encoded segment after an `e` marker.
 */
import type { BoardModel, BoardNode } from "./canvas";

/** One entered nested-canvas level (in-place drill-down, no DOM reparenting). */
export interface BoardLevel {
  nodeId: string;
  label: string;
  model: BoardModel;
}

export function boardPath(file: string, ids: string[]): string {
  let path = "/wiki/board";
  if (file) path += "/" + file;
  for (const id of ids) path += "/e/" + encodeURIComponent(id);
  return path;
}

export function parseBoardSplat(splat: string | undefined): { file: string; ids: string[] } {
  const s = (splat ?? "").replace(/^\/+/, "").replace(/\/+$/, "");
  if (!s) return { file: "", ids: [] };
  const segs = s.split("/");
  const eIndex = segs.indexOf("e");
  if (eIndex < 0) return { file: s, ids: [] };
  const file = segs.slice(0, eIndex).join("/");
  const ids: string[] = [];
  for (let i = eIndex; i < segs.length; i++) {
    if (segs[i] === "e" && i + 1 < segs.length) {
      const raw = segs[i + 1];
      try {
        ids.push(decodeURIComponent(raw));
      } catch {
        ids.push(raw);
      }
      i++;
    }
  }
  return { file, ids };
}

/** Walk the drill ids from the root model; stop at the deepest resolvable one. */
export function buildLevels(root: BoardModel, ids: string[]): BoardLevel[] {
  const levels: BoardLevel[] = [];
  let doc: BoardModel = root;
  for (const id of ids) {
    const node: BoardNode | undefined = doc.nodes.find(
      (n) => n.id === id && n.type === "nested-canvas" && n.canvas,
    );
    if (!node || !node.canvas) break;
    levels.push({ nodeId: id, label: String(node.title ?? "Nested canvas"), model: node.canvas });
    doc = node.canvas;
  }
  return levels;
}
