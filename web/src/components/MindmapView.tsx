import { useEffect, useMemo, useRef, useState } from "react";
import { Markmap } from "markmap-view";
import { annotateBoundaries, boundaryRects, parseBoundaries } from "../lib/boundaries";
import { drawBoundaryLayer } from "../lib/boundaryDraw";
import {
  DEFAULT_FUSE_OPTIONS,
  extractMapRefs,
  fuseMap,
  type FuseStats,
  type MapRef,
} from "../lib/fuse";
import { transformMindmap, type MindNode } from "../lib/mindmap";
import { fetchDoc, isCanvas } from "../lib/api";
import { isMapPath } from "../lib/documentModes";
import { Icon } from "../lib/icon";
import { loadMapIndex, loadWikiIndex, type MapIndexEntry } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";

interface MindmapViewProps {
  markdown: string;
  basePath: string;
  title: string;
}

type PureData = Parameters<Markmap["setData"]>[0];

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true
  );
}

/** markmap view with XMindMark boundary overlay and an ephemeral fused map. */
export function MindmapView({ markdown, basePath, title }: MindmapViewProps) {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const mmRef = useRef<Markmap | null>(null);
  const [wikiIndex, setWikiIndex] = useState<WikiIndex | undefined>(undefined);
  const [mapIndex, setMapIndex] = useState<Map<string, MapIndexEntry>>(new Map());
  const [mode, setMode] = useState<"current" | "fused">("current");
  const [fused, setFused] = useState<{ path: string; root: MindNode; stats: FuseStats } | null>(
    null,
  );
  const [refError, setRefError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    loadWikiIndex()
      .then((ix) => alive && setWikiIndex(ix))
      .catch(() => undefined);
    loadMapIndex()
      .then((mi) => alive && setMapIndex(mi))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const isMap = isMapPath(basePath);
  const currentRoot = useMemo(
    () => transformMindmap(markdown, { basePath, wikiIndex }),
    [markdown, basePath, wikiIndex],
  );

  const referencedIds = useMemo(
    () => extractMapRefs(currentRoot, mapIndex),
    [currentRoot, mapIndex],
  );

  useEffect(() => {
    if (mode !== "fused" || !isMap) return;
    let alive = true;
    const refs = new Map<string, MapRef>();
    const load = async () => {
      for (const id of referencedIds) {
        const entry = mapIndex.get(id);
        if (!entry) continue;
        const doc = await fetchDoc(entry.path);
        if (isCanvas(doc)) continue;
        refs.set(entry.id, {
          id: entry.id,
          title: entry.title,
          root: transformMindmap(doc.markdown, { basePath: entry.path, wikiIndex }),
        });
      }
      if (!alive) return;
      const result = fuseMap(basePath, currentRoot, refs, DEFAULT_FUSE_OPTIONS);
      setFused({ path: basePath, root: result.root, stats: result.stats });
      setRefError(null);
    };
    load().catch((err: unknown) => {
      if (alive) setRefError(err instanceof Error ? err.message : String(err));
    });
    return () => {
      alive = false;
    };
  }, [mode, isMap, referencedIds, mapIndex, currentRoot, basePath, wikiIndex]);

  const current = mode === "fused" && fused?.path === basePath && fused ? fused : null;
  const activeRoot = current ? current.root : currentRoot;
  const fusedStats = current?.stats ?? null;

  useEffect(() => {
    if (!svgRef.current) return;
    const mm = Markmap.create(svgRef.current, {
      autoFit: true,
      duration: prefersReducedMotion() ? 0 : 300,
      maxWidth: 260,
      embedGlobalCSS: true,
    });
    mmRef.current = mm;
    return () => {
      mm.destroy();
      mmRef.current = null;
    };
  }, []);

  useEffect(() => {
    const mm = mmRef.current;
    if (!mm) return;
    let alive = true;
    const parsed = parseBoundaries(markdown);
    mm.setData(activeRoot as unknown as PureData, { autoFit: mode === "current" })
      .then(() => {
        if (!alive) return;
        const rendered = (mm.state.data ?? activeRoot) as MindNode;
        const groups = annotateBoundaries(rendered, parsed);
        drawBoundaryLayer(mm, boundaryRects(groups));
        return mm.fit();
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [activeRoot, markdown, mode]);

  return (
    <div className="mindmap">
      <div className="mindmap__toolbar">
        <span className="mindmap__title">Mindmap</span>
        {isMap && referencedIds.length > 0 && (
          <div className="graph-tools__row" role="group" aria-label="Map mode">
            <button
              type="button"
              className={`graph-tools__chip${mode === "current" ? " is-active" : ""}`}
              aria-pressed={mode === "current"}
              onClick={() => setMode("current")}
            >
              <Icon name="my_location" />
              Current Map
            </button>
            <button
              type="button"
              className={`graph-tools__chip${mode === "fused" ? " is-active" : ""}`}
              aria-pressed={mode === "fused"}
              onClick={() => setMode("fused")}
            >
              <Icon name="merge" />
              Fused Map
            </button>
          </div>
        )}
        {mode === "fused" && fusedStats && (
          <span className="mindmap__stats">
            {fusedStats.imported.length} imported
            {fusedStats.deduped.length > 0 ? `, ${fusedStats.deduped.length} deduped` : ""}
            {fusedStats.cycles.length > 0 ? `, ${fusedStats.cycles.length} cycles skipped` : ""}
          </span>
        )}
      </div>
      {refError && <p className="content__empty">Fused map error: {refError}</p>}
      <svg
        ref={svgRef}
        className="mindmap__svg"
        role="img"
        aria-label={`Mindmap: ${title}`}
        preserveAspectRatio="xMidYMid meet"
      />
    </div>
  );
}
