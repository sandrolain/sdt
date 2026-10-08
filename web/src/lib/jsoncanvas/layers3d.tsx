/**
 * Layered 3D view for the shared JSON Canvas canvas.
 *
 * Adapted from `context/refs/react/jc-vite/src/JsonCanvas.jsx` (MIT). This is the
 * **only** module that statically imports `three`, and it is loaded lazily by the
 * view (only in 3D mode) so `three` never joins the entry chunk. Depth comes from
 * each node's `x-layer`; layer names from `doc["x-layers"]`. Read-only.
 */
import { useEffect, useMemo, useRef } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { edgeGeom } from "./geometry";
import { nodeLayer, type CanvasDocument, type CanvasNode } from "./document";
import { nodeLabelText, resolveColor, type CanvasTheme } from "./theme";
import { collapsedContainedIds } from "./collapse";

function tex(
  n: CanvasNode,
  theme: CanvasTheme,
  presets: Record<string, string>,
): THREE.CanvasTexture {
  const s = Math.min(2, 1024 / Math.max(n.width, n.height));
  const c = document.createElement("canvas");
  c.width = Math.ceil(n.width * s);
  c.height = Math.ceil(n.height * s);
  const g = c.getContext("2d");
  const col = resolveColor(n.color, presets);
  const grp = n.type === "group";
  if (g) {
    g.scale(s, s);
    g.fillStyle = grp ? theme.group : theme.node;
    g.fillRect(0, 0, n.width, n.height);
    if (col) {
      g.globalAlpha = grp ? 0.1 : 0.12;
      g.fillStyle = col;
      g.fillRect(0, 0, n.width, n.height);
      g.globalAlpha = 1;
    }
    g.strokeStyle = col || theme.border;
    g.lineWidth = grp ? 4 : 3;
    g.strokeRect(1.5, 1.5, n.width - 3, n.height - 3);
    g.fillStyle = theme.text;
    g.textBaseline = "top";
    g.font = `${grp ? 700 : 500} ${grp ? 22 : 16}px system-ui,sans-serif`;
    const txt = nodeLabelText(n);
    let y = 14;
    const lh = grp ? 28 : 22;
    for (const para of txt.split("\n")) {
      let line = "";
      for (const w of para.split(" ")) {
        const candidate = line ? `${line} ${w}` : w;
        if (g.measureText(candidate).width > n.width - 28 && line) {
          g.fillText(line, 14, y);
          y += lh;
          line = w;
        } else {
          line = candidate;
        }
      }
      g.fillText(line, 14, y);
      y += lh;
      if (y > n.height - 8) break;
    }
  }
  const t = new THREE.CanvasTexture(c);
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 4;
  return t;
}

interface Layers3DProps {
  doc: CanvasDocument;
  theme: CanvasTheme;
  presets: Record<string, string>;
  gap: number;
  hidden: number[];
  onPick?: (id: string) => void;
  collapsed?: string[];
}

/** Read-only layered 3D scene. One plane per `x-layer`. */
export function Layers3D({ doc, theme, presets, gap, hidden, onPick, collapsed }: Layers3DProps) {
  const host = useRef<HTMLDivElement>(null);
  const nodes = useMemo(() => doc.nodes ?? [], [doc]);
  const edges = useMemo(() => doc.edges ?? [], [doc]);
  const layers = useMemo(() => [...new Set(nodes.map(nodeLayer))].sort((a, b) => a - b), [nodes]);
  const collapsedIds = useMemo(
    () => collapsedContainedIds(nodes, new Set(collapsed ?? [])),
    [nodes, collapsed],
  );

  useEffect(() => {
    const el = host.current;
    if (!el || !nodes.length) return;
    const byId = Object.fromEntries(nodes.map((n) => [n.id, n]));
    const off = (n: CanvasNode) => hidden.includes(nodeLayer(n));
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setPixelRatio(Math.min(2, devicePixelRatio));
    el.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    const cam = new THREE.PerspectiveCamera(45, 1, 10, 50000);
    const x0 = Math.min(...nodes.map((n) => n.x));
    const x1 = Math.max(...nodes.map((n) => n.x + n.width));
    const y0 = Math.min(...nodes.map((n) => n.y));
    const y1 = Math.max(...nodes.map((n) => n.y + n.height));
    const W = x1 - x0;
    const H = y1 - y0;
    const S = Math.max(W, H);
    const cx = (x0 + x1) / 2;
    const cy = -(y0 + y1) / 2;
    const zs = layers.map((l) => l * gap);
    const cz = (Math.min(...zs) + Math.max(...zs)) / 2;
    cam.position.set(cx + S * 0.35, cy - S * 0.8, cz + S * 0.75);
    const ctl = new OrbitControls(cam, renderer.domElement);
    ctl.target.set(cx, cy, cz);
    ctl.enableDamping = true;
    layers
      .filter((l) => !hidden.includes(l))
      .forEach((l) => {
        const g = new THREE.PlaneGeometry(W + 240, H + 240);
        const m = new THREE.Mesh(
          g,
          new THREE.MeshBasicMaterial({
            color: theme.accent,
            transparent: true,
            opacity: 0.05,
            side: THREE.DoubleSide,
            depthWrite: false,
          }),
        );
        m.position.set(cx, cy, l * gap - 6);
        scene.add(m);
        const ln = new THREE.LineSegments(
          new THREE.EdgesGeometry(g),
          new THREE.LineBasicMaterial({ color: theme.accent, transparent: true, opacity: 0.5 }),
        );
        ln.position.copy(m.position);
        scene.add(ln);
      });
    const meshes = nodes
      .filter((n) => !off(n) && !collapsedIds.has(n.id))
      .map((n) => {
        const grp = n.type === "group";
        const m = new THREE.Mesh(
          new THREE.PlaneGeometry(n.width, n.height),
          new THREE.MeshBasicMaterial({
            map: tex(n, theme, presets),
            transparent: true,
            side: THREE.DoubleSide,
            depthWrite: !grp,
          }),
        );
        m.position.set(
          n.x + n.width / 2,
          -(n.y + n.height / 2),
          nodeLayer(n) * gap + (grp ? -3 : 0),
        );
        m.userData.id = n.id;
        scene.add(m);
        return m;
      });
    edges.forEach((e) => {
      const A = byId[e.fromNode];
      const B = byId[e.toNode];
      if (!A || !B || off(A) || off(B) || collapsedIds.has(A.id) || collapsedIds.has(B.id)) return;
      const g = edgeGeom(e, byId);
      if (!g) return;
      const zA = nodeLayer(A) * gap;
      const zB = nodeLayer(B) * gap;
      const v = (p: { x: number; y: number }, z: number) => new THREE.Vector3(p.x, -p.y, z);
      const curve = new THREE.CubicBezierCurve3(v(g.p, zA), v(g.c1, zA), v(g.c2, zB), v(g.q, zB));
      const col = new THREE.Color(resolveColor(e.color, presets) || theme.edge);
      scene.add(
        new THREE.Line(
          new THREE.BufferGeometry().setFromPoints(curve.getPoints(40)),
          new THREE.LineBasicMaterial({ color: col }),
        ),
      );
      if ((e.toEnd || "arrow") === "arrow") {
        const t = curve.getTangent(1).normalize();
        const c = new THREE.Mesh(
          new THREE.ConeGeometry(6, 18, 10),
          new THREE.MeshBasicMaterial({ color: col }),
        );
        c.position.copy(curve.getPoint(1)).addScaledVector(t, -9);
        c.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), t);
        scene.add(c);
      }
    });
    const rs = () => {
      renderer.setSize(el.clientWidth, el.clientHeight);
      cam.aspect = el.clientWidth / el.clientHeight;
      cam.updateProjectionMatrix();
    };
    const ro = new ResizeObserver(rs);
    ro.observe(el);
    rs();
    let raf = 0;
    const loop = () => {
      ctl.update();
      renderer.render(scene, cam);
      raf = requestAnimationFrame(loop);
    };
    loop();
    let d0: [number, number] | undefined;
    const rc = new THREE.Raycaster();
    const dn = (e: PointerEvent) => {
      d0 = [e.clientX, e.clientY];
    };
    const up = (e: PointerEvent) => {
      if (!d0 || Math.hypot(e.clientX - d0[0], e.clientY - d0[1]) > 4) return;
      const r = renderer.domElement.getBoundingClientRect();
      rc.setFromCamera(
        new THREE.Vector2(
          ((e.clientX - r.left) / r.width) * 2 - 1,
          -((e.clientY - r.top) / r.height) * 2 + 1,
        ),
        cam,
      );
      const h = rc.intersectObjects(
        meshes.filter((m) => byId[m.userData.id as string].type !== "group"),
      )[0];
      if (h) onPick?.(h.object.userData.id as string);
    };
    renderer.domElement.addEventListener("pointerdown", dn);
    renderer.domElement.addEventListener("pointerup", up);
    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      ctl.dispose();
      scene.traverse((o) => {
        const mesh = o as THREE.Mesh;
        mesh.geometry?.dispose();
        const mat = mesh.material as THREE.MeshBasicMaterial | undefined;
        mat?.map?.dispose();
        mat?.dispose?.();
      });
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, [doc, hidden, theme, presets, gap, nodes, layers, onPick, edges, collapsedIds]);

  return <div className="jc-3d" ref={host} />;
}

export default Layers3D;
