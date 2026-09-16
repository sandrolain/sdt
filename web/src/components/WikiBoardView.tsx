import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { fetchTree, fetchWikiBoard, type TreeEntry } from "../lib/api";
import { normalizeBoard, type BoardModel, type BoardNode } from "../lib/canvas";
import { displayTitle } from "../lib/titles";
import { BoardView } from "./BoardView";
import { SkeletonLines } from "./Skeleton";
import { useReloadToken } from "../lib/useReloadToken";
import { useOpenDocs } from "../lib/openDocsContext";

/** #/wiki/board — read-only board from the graph default or a .canvas file. */
export function WikiBoardView() {
  const [params, setParams] = useSearchParams();
  const { open: openDoc } = useOpenDocs();
  const file = params.get("file") ?? "";
  const [model, setModel] = useState<BoardModel | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [canvases, setCanvases] = useState<TreeEntry[]>([]);
  const reloadToken = useReloadToken();

  useEffect(() => {
    let alive = true;
    fetchTree()
      .then((res) => {
        if (alive) setCanvases(res.entries.filter((e) => e.canvas));
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  useEffect(() => {
    let alive = true;
    fetchWikiBoard(file || undefined)
      .then((res) => {
        if (alive) {
          setModel(normalizeBoard(res));
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (alive) {
          setModel(null);
          setError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      alive = false;
    };
  }, [file, reloadToken]);

  const current = useMemo(() => params.get("file") ?? "", [params]);

  const open = (node: BoardNode) => {
    if (node.file) {
      const path = node.file.startsWith("context/") ? node.file : `context/${node.file}`;
      openDoc(path);
      return;
    }
    if (node.type === "text" && node.id) openDoc(`context/wiki/${node.id}.md`);
  };

  return (
    <div className="board-page">
      <div className="board-page__toolbar">
        <label className="graph-tools__field">
          <span className="graph-tools__label">Source</span>
          <select
            className="graph-tools__control"
            value={current}
            onChange={(e) => {
              const next = new URLSearchParams(params);
              if (e.target.value) next.set("file", e.target.value);
              else next.delete("file");
              setParams(next, { replace: true });
            }}
            aria-label="Board source"
          >
            <option value="">Wiki graph (default)</option>
            {canvases.map((c) => (
              <option key={c.path} value={c.path}>
                {displayTitle({ title: c.title, path: c.path })}
              </option>
            ))}
          </select>
        </label>
        <span className="board-page__hint">Read-only · pan with drag · zoom with the wheel</span>
      </div>
      {error ? (
        <p className="content__empty">Board error: {error}</p>
      ) : !model ? (
        <SkeletonLines count={5} label="Loading board" />
      ) : (
        <BoardView key={file} model={model} onOpen={open} />
      )}
    </div>
  );
}
