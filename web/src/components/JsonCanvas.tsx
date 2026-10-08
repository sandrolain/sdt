/**
 * JsonCanvas — the shared, read-only, lane-agnostic infinite canvas.
 *
 * Derived from `context/refs/react/jc-vite/src/JsonCanvas.jsx` (MIT): a JSON
 * Canvas 1.0 renderer with pan/wheel-zoom/fit, an SVG edge layer, a minimap and a
 * layered 3D view. Adaptations for SDT: read-only (editing removed), token-derived
 * theme, external-link scheme guard, lazy `three`, and a fully prop-driven /
 * imperative contract so the board, the mind map and the nested-canvas drill-down
 * reuse one component instead of forking it (decision 0025).
 */
import {
  forwardRef,
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
  type ReactNode,
} from "react";
import "./JsonCanvas.css";
import { type CanvasDocument, type CanvasEdge, type CanvasNode } from "../lib/jsoncanvas/document";
import { collapsedContainedIds } from "../lib/jsoncanvas/collapse";
import { edgeKindStyle } from "../lib/jsoncanvas/edgeKind";
import { arrow, edgeGeom } from "../lib/jsoncanvas/geometry";
import { Md } from "../lib/jsoncanvas/markdown";
import { Minimap } from "../lib/jsoncanvas/minimap";
import {
  FALLBACK_THEMES,
  PRESET_COLORS,
  resolveColor,
  themeFromTokens,
} from "../lib/jsoncanvas/theme";
import { safeExternalUrl, urlHost } from "../lib/jsoncanvas/url";

const Layers3D = lazy(() => import("../lib/jsoncanvas/layers3d"));

const EMPTY: CanvasDocument = { nodes: [], edges: [] };

export interface JsonCanvasHandle {
  zoomIn(): void;
  zoomOut(): void;
  fit(): void;
  focusNode(id: string): void;
}

export interface JsonCanvasProps {
  data: CanvasDocument;
  mode?: "2d" | "3d";
  hiddenLayers?: number[];
  showMinimap?: boolean;
  /** Re-fit whenever this value changes (a layout token). */
  fitKey?: unknown;
  layerGap?: number;
  theme?: "light" | "dark" | "auto";
  presets?: Record<string, string>;
  className?: string;
  style?: CSSProperties;
  /** Custom body for a `text` node. */
  renderText?: (node: CanvasNode) => ReactNode;
  /** Custom card for a `file` node. */
  renderFile?: (node: CanvasNode) => ReactNode;
  /** Custom body for other node types (`nested-canvas`, …); undefined falls through. */
  renderNode?: (node: CanvasNode, ctx: { placeholderScale: number }) => ReactNode | undefined;
  /** Extra layer drawn inside the canvas world (behind the nodes). */
  renderOverlay?: (ctx: {
    view: { x: number; y: number; k: number };
    theme: { accent: string };
  }) => ReactNode;
  placeholderScale?: number;
  /** Accessible name of the canvas root. */
  ariaLabel?: string;
  onOpenNode?: (node: CanvasNode) => void;
  /** A 3D-scene pick (reference parity: focus the node in 2D). */
  onPick?: (node: CanvasNode) => void;
  /** Ids of collapsed `group` nodes (O6): their contained nodes are hidden. */
  collapsedGroups?: string[];
  onSelectNode?: (id: string | null) => void;
  onViewChange?: (zoom: number) => void;
}

interface ViewState {
  x: number;
  y: number;
  k: number;
}

function subscribeThemeAttr(onChange: () => void): () => void {
  if (typeof MutationObserver === "undefined" || typeof document === "undefined") return () => {};
  const obs = new MutationObserver(onChange);
  obs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  return () => obs.disconnect();
}

function readThemeAttr(): "light" | "dark" | null {
  if (typeof document === "undefined") return null;
  const attr = document.documentElement.dataset.theme;
  return attr === "light" || attr === "dark" ? attr : null;
}

function useThemeMode(theme: "light" | "dark" | "auto"): "light" | "dark" {
  const documentMode = useSyncExternalStore(subscribeThemeAttr, readThemeAttr, () => null);
  const [systemDark] = useState(
    () => typeof matchMedia !== "undefined" && matchMedia("(prefers-color-scheme: dark)").matches,
  );
  if (theme !== "auto") return theme;
  return documentMode ?? (systemDark ? "dark" : "light");
}

export const JsonCanvas = forwardRef<JsonCanvasHandle, JsonCanvasProps>(function JsonCanvas(
  {
    data,
    mode: modeProp = "2d",
    hiddenLayers = [],
    showMinimap = true,
    fitKey,
    layerGap = 260,
    theme = "auto",
    presets = PRESET_COLORS,
    className,
    style,
    renderText,
    renderFile,
    renderNode,
    renderOverlay,
    placeholderScale = 0.9,
    ariaLabel,
    onOpenNode,
    onPick,
    collapsedGroups,
    onSelectNode,
    onViewChange,
  },
  ref,
) {
  const doc = data ?? EMPTY;
  const nodes = useMemo(() => doc.nodes ?? [], [doc]);
  const edges = useMemo(() => doc.edges ?? [], [doc]);
  const byId = useMemo(() => Object.fromEntries(nodes.map((n) => [n.id, n])), [nodes]);
  const mode = useThemeMode(theme);
  const themeRef = useRef<HTMLDivElement>(null);
  const [resolvedTheme, setResolvedTheme] = useState(() => FALLBACK_THEMES[mode]);
  const [view, setView] = useState<ViewState>({ x: 0, y: 0, k: 1 });
  const [size, setSize] = useState({ w: 800, h: 600 });
  const [selected, setSelected] = useState<string[]>([]);
  const hostRef = useRef<HTMLDivElement>(null);
  const drag = useRef<{ sx: number; sy: number; vx: number; vy: number } | null>(null);
  const sizedOnce = useRef(false);

  // Re-derive concrete theme colours from the semantic tokens on mode change.
  useEffect(() => {
    setResolvedTheme(themeFromTokens(themeRef.current, mode));
  }, [mode]);

  const fit = useCallback(() => {
    const el = hostRef.current;
    if (!el || !nodes.length) return;
    const w = el.clientWidth;
    const h = el.clientHeight;
    // A panel that has not laid out yet has no size: keep the current view
    // rather than fitting to 0 (the ResizeObserver re-fits once it is sized).
    if (!w || !h) return;
    const x0 = Math.min(...nodes.map((n) => n.x));
    const x1 = Math.max(...nodes.map((n) => n.x + n.width));
    const y0 = Math.min(...nodes.map((n) => n.y - 32));
    const y1 = Math.max(...nodes.map((n) => n.y + n.height));
    const k = Math.min(1.5, w / (x1 - x0 + 160), h / (y1 - y0 + 200));
    setView({ k, x: w / 2 - (k * (x0 + x1)) / 2, y: h / 2 - (k * (y0 + y1)) / 2 - 20 });
  }, [nodes]);

  const zoomBy = useCallback((factor: number) => {
    setView((v) => ({ ...v, k: Math.min(4, Math.max(0.1, v.k * factor)) }));
  }, []);

  const zoomIn = useCallback(() => zoomBy(1.25), [zoomBy]);
  const zoomOut = useCallback(() => zoomBy(1 / 1.25), [zoomBy]);
  const focusNode = useCallback(
    (id: string) => {
      const n = byId[id];
      if (!n) return;
      setSelected([id]);
      setView({ k: 1, x: size.w / 2 - n.x - n.width / 2, y: size.h / 2 - n.y - n.height / 2 });
    },
    [byId, size],
  );

  useImperativeHandle(ref, () => ({ zoomIn, zoomOut, fit, focusNode }), [
    zoomIn,
    zoomOut,
    fit,
    focusNode,
  ]);

  // Initial fit + resize tracking.
  useEffect(() => {
    const el = hostRef.current;
    fit();
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() => {
      const w = el.clientWidth;
      const h = el.clientHeight;
      setSize({ w, h });
      // First time the panel has a real size, fit to it (the mount fit may have
      // run before layout and returned early).
      if (!sizedOnce.current && w > 0 && h > 0) {
        sizedOnce.current = true;
        fit();
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [fit]);

  // Re-fit on an external layout token change.
  useEffect(() => {
    fit();
  }, [fitKey, fit]);

  useEffect(() => {
    onViewChange?.(view.k);
  }, [view.k, onViewChange]);

  // Wheel zoom (non-passive so preventDefault works).
  useEffect(() => {
    const el = hostRef.current;
    if (!el || modeProp === "3d") return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      const cx = e.clientX - r.left;
      const cy = e.clientY - r.top;
      setView((v) => {
        const k = Math.min(
          4,
          Math.max(0.1, v.k * Math.exp(-e.deltaY * (e.ctrlKey ? 0.01 : 0.0015))),
        );
        return { k, x: cx - ((cx - v.x) * k) / v.k, y: cy - ((cy - v.y) * k) / v.k };
      });
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [modeProp]);

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (modeProp === "3d" || e.button === 2) return;
      hostRef.current?.focus({ preventScroll: true });
      const target = (e.target as HTMLElement).closest("[data-nid]");
      const id = target?.getAttribute("data-nid") ?? null;
      if (id) {
        setSelected([id]);
        onSelectNode?.(id);
        return;
      }
      setSelected([]);
      onSelectNode?.(null);
      drag.current = { sx: e.clientX, sy: e.clientY, vx: view.x, vy: view.y };
      const move = (ev: PointerEvent) => {
        const d = drag.current;
        if (!d) return;
        setView((v) => ({ ...v, x: d.vx + ev.clientX - d.sx, y: d.vy + ev.clientY - d.sy }));
      };
      const up = () => {
        drag.current = null;
        removeEventListener("pointermove", move);
        removeEventListener("pointerup", up);
      };
      addEventListener("pointermove", move);
      addEventListener("pointerup", up);
    },
    [modeProp, view.x, view.y, onSelectNode],
  );

  const openNode = useCallback(
    (n: CanvasNode) => {
      if (n.type === "link") {
        const safe = safeExternalUrl(n.url);
        if (safe) window.open(safe, "_blank", "noopener");
        return;
      }
      onOpenNode?.(n);
    },
    [onOpenNode],
  );

  const collapsed = useMemo(() => new Set(collapsedGroups ?? []), [collapsedGroups]);
  const hiddenIds = useMemo(() => collapsedContainedIds(nodes, collapsed), [nodes, collapsed]);
  const groups = useMemo(
    () =>
      nodes
        .filter((n) => n.type === "group")
        .sort((a, b) => b.width * b.height - a.width * a.height),
    [nodes],
  );
  const others = useMemo(
    () => nodes.filter((n) => n.type !== "group" && !hiddenIds.has(n.id)),
    [nodes, hiddenIds],
  );

  const renderBody = (n: CanvasNode): ReactNode => {
    const custom = renderNode?.(n, { placeholderScale });
    if (custom !== undefined) return custom;
    if (n.type === "text") return renderText ? renderText(n) : <Md text={n.text ?? ""} />;
    if (n.type === "file") {
      if (renderFile) return renderFile(n);
      const name = (n.file ?? "").split("/").pop() ?? "";
      return (
        <div className="jc-card">
          <div className="jc-ct">📄 {name}</div>
          <div className="jc-cp">
            {n.file}
            {n.subpath ?? ""}
          </div>
        </div>
      );
    }
    if (n.type === "link") {
      const safe = safeExternalUrl(n.url);
      return (
        <div className="jc-card">
          <div className="jc-ct">🔗 {urlHost(n.url)}</div>
          {safe ? (
            <a className="jc-cp" href={safe} target="_blank" rel="noopener noreferrer">
              {n.url}
            </a>
          ) : (
            <span className="jc-cp">{n.url}</span>
          )}
        </div>
      );
    }
    const label = n.title ?? n.label ?? n.id;
    return (
      <div className="jc-card">
        <div className="jc-ct">{String(label)}</div>
      </div>
    );
  };

  const renderNodeEl = (n: CanvasNode) => {
    const col = resolveColor(n.color, presets);
    const isSel = selected.includes(n.id);
    const cssVars: CSSProperties = {
      left: n.x,
      top: n.y,
      width: n.width,
      height: n.height,
      zIndex: isSel ? 2 : 1,
    };
    if (col) (cssVars as Record<string, string | number>)["--c"] = col;
    return (
      <div
        key={n.id}
        data-nid={n.id}
        role="button"
        tabIndex={0}
        aria-label={String(n.title ?? n.label ?? n.text ?? n.file ?? n.url ?? n.id)}
        aria-pressed={isSel}
        className={`jc-node jc-${n.type}${isSel ? " sel" : ""}${col ? " col" : ""}${
          n.type === "group" && collapsed.has(n.id) ? " jc-group--collapsed" : ""
        }`}
        style={cssVars}
        onDoubleClick={(e) => {
          e.stopPropagation();
          openNode(n);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            openNode(n);
          }
        }}
      >
        {n.type === "group" && <div className="jc-gl">{n.label || "Group"}</div>}
        {n.type !== "group" && <div className="jc-body">{renderBody(n)}</div>}
      </div>
    );
  };

  const cssVars: CSSProperties = {
    "--bg": resolvedTheme.bg,
    "--grid": resolvedTheme.grid,
    "--node": resolvedTheme.node,
    "--text": resolvedTheme.text,
    "--muted": resolvedTheme.muted,
    "--border": resolvedTheme.border,
    "--accent": resolvedTheme.accent,
    "--group": resolvedTheme.group,
    "--c": resolvedTheme.border,
  } as CSSProperties;

  return (
    <div
      ref={(el) => {
        hostRef.current = el;
        themeRef.current = el;
      }}
      tabIndex={0}
      role="application"
      aria-label={ariaLabel ?? "JSON Canvas (read-only)"}
      className={`jc ${className ?? ""}`}
      style={{
        ...cssVars,
        backgroundImage: "radial-gradient(circle, var(--grid) 1.2px, transparent 1.5px)",
        backgroundSize: `${24 * view.k}px ${24 * view.k}px`,
        backgroundPosition: `${view.x}px ${view.y}px`,
        ...style,
      }}
      onPointerDown={onPointerDown}
      onKeyDown={(e) => {
        if (e.key === "Escape") setSelected([]);
        if (e.key === "0") fit();
      }}
    >
      {modeProp === "3d" ? (
        <Suspense fallback={<div className="jc-3d-loading">Loading 3D…</div>}>
          <Layers3D
            doc={doc}
            theme={resolvedTheme}
            presets={presets}
            gap={layerGap}
            hidden={hiddenLayers}
            collapsed={collapsedGroups}
            onPick={(id) => {
              const n = byId[id];
              if (n) onPick?.(n);
            }}
          />
        </Suspense>
      ) : (
        <>
          <div
            className="jc-world"
            style={{ transform: `translate(${view.x}px,${view.y}px) scale(${view.k})` }}
          >
            {renderOverlay?.({ view, theme: resolvedTheme })}
            {groups.map(renderNodeEl)}
            <svg className="jc-edges" aria-hidden="true">
              {edges.map((e: CanvasEdge) => {
                const g = edgeGeom(e, byId);
                if (!g) return null;
                const col = resolveColor(e.color, presets) || resolvedTheme.edge;
                const kindStyle = edgeKindStyle(e["x-kind"]);
                const dash =
                  (typeof e["x-dash"] === "string" ? e["x-dash"] : undefined) ?? kindStyle.dash;
                return (
                  <g key={e.id} className="jc-edge">
                    <path
                      d={g.d}
                      className="line"
                      stroke={col}
                      strokeDasharray={dash}
                      strokeWidth={kindStyle.width}
                    />
                    {(e.toEnd || "arrow") === "arrow" && (
                      <polygon points={arrow(g.q, g.db)} fill={col} />
                    )}
                    {e.fromEnd === "arrow" && <polygon points={arrow(g.p, g.da)} fill={col} />}
                    {e.label && (
                      <text
                        x={g.mid.x}
                        y={g.mid.y}
                        fill={col}
                        textAnchor="middle"
                        dominantBaseline="middle"
                      >
                        {e.label}
                      </text>
                    )}
                  </g>
                );
              })}
            </svg>
            {others.map(renderNodeEl)}
          </div>
          {showMinimap && nodes.length > 0 && (
            <Minimap
              nodes={nodes}
              view={view}
              size={size}
              theme={resolvedTheme}
              presets={presets}
              onGo={(x, y) =>
                setView((v) => ({ ...v, x: size.w / 2 - x * v.k, y: size.h / 2 - y * v.k }))
              }
            />
          )}
        </>
      )}
    </div>
  );
});

export default JsonCanvas;
