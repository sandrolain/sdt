import { fetchJSON } from "./fetchJson";

export interface TreeEntry {
  path: string;
  /** omitted for .md entries without frontmatter kind */
  kind?: string;
  title?: string;
  summary?: string;
  /** frontmatter `objective` grouping key (kebab-case slug) */
  objective?: string;
  /** frontmatter `status` (plan/task execution state) */
  status?: string;
  /** frontmatter `sources` + `links` references, corpus-relative */
  sources?: string[];
  created?: string;
  /** frontmatter `updated`, else the file mtime (RFC3339) */
  modified?: string;
  /** frontmatter `image` (corpus- or doc-relative), served via /api/file */
  image?: string;
  canvas?: boolean;
  /** true for `.mmd` standalone mermaid documents */
  mermaid?: boolean;
  /** true for `.map.md` semantic map documents */
  isMap?: boolean;
  /** canonical map id, present when isMap */
  mapId?: string;
}

export interface TreeResponse {
  entries: TreeEntry[];
}

export interface DocResponse {
  path: string;
  frontmatter: string;
  markdown: string;
}

export interface CanvasResponse {
  path: string;
  canvas: unknown;
}

/** Raw mermaid source payload natively returned by /api/doc for `.mmd` files. */
export interface MermaidResponse {
  path: string;
  source: string;
}

export interface ErrorResponse {
  error: string;
}

export interface WikiGraphNode {
  id: string;
  title: string;
  type?: string;
  status?: string;
  tags?: string[];
  summary?: string;
  path: string;
}

export interface WikiGraphEdge {
  source: string;
  target: string;
  verb: string;
  label?: string;
  kind: string;
}

export interface WikiGraphResponse {
  nodes: WikiGraphNode[];
  edges: WikiGraphEdge[];
}

export interface GraphEdge {
  source: string;
  target: string;
  verb: string;
  label?: string;
  kind: string;
}

export interface RelEntry {
  source?: string;
  target?: string;
  label?: string;
  kind: string;
  title?: string;
  type?: string;
  status?: string;
  summary?: string;
  path: string;
}

export type RelGroup = Record<string, RelEntry[]>;

export interface RelResponse {
  id: string;
  title: string;
  inbound?: RelGroup;
  outbound?: RelGroup;
}

/** Canvas payload natively returned by /api/doc for `.canvas` files. */
export interface CanvasFile {
  nodes?: unknown[];
  edges?: unknown[];
}

export interface SearchResult {
  path: string;
  /** omitted for .md entries without frontmatter kind */
  kind?: string;
  title?: string;
  summary?: string;
  /** frontmatter `objective` grouping key (kebab-case slug) */
  objective?: string;
  created?: string;
  /** frontmatter `updated` date, else file mtime (fallback) */
  modified?: string;
  score: number;
  snippet: string;
  /** true for `.map.md` semantic map documents */
  isMap?: boolean;
  /** canonical map id, present when isMap */
  mapId?: string;
  /** true for `.mmd` mermaid documents */
  isMermaid?: boolean;
  /** canonical mermaid id, present when isMermaid */
  mermaidId?: string;
  /** true for `.canvas` board documents */
  isCanvas?: boolean;
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
}

export interface SearchQuery {
  q: string;
  /** exact frontmatter kind filter; omitted when empty */
  kind?: string;
  /** exact frontmatter objective filter; omitted when empty */
  objective?: string;
  /** inclusive frontmatter created-date bounds (YYYY-MM-DD); omitted when empty */
  from?: string;
  to?: string;
  limit?: number;
}

async function getJSON<T>(url: string): Promise<T> {
  return fetchJSON<T>(url);
}

export function fetchTree(): Promise<TreeResponse> {
  return getJSON("/api/tree");
}

export function fetchDoc(path: string): Promise<DocResponse | CanvasResponse | MermaidResponse> {
  return getJSON(`/api/doc?path=${encodeURIComponent(path)}`);
}

/** Compose the /api/search URL, omitting empty q/kind/objective/from/to params. */
export function buildSearchUrl(query: SearchQuery): string {
  const params = new URLSearchParams();
  params.set("q", query.q);
  if (query.kind) params.set("kind", query.kind);
  if (query.objective) params.set("objective", query.objective);
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.limit != null) params.set("limit", String(query.limit));
  return `/api/search?${params.toString()}`;
}

export function fetchSearch(query: SearchQuery): Promise<SearchResponse> {
  return getJSON(buildSearchUrl(query));
}

export function fetchWikiRel(id: string): Promise<RelResponse> {
  return getJSON(`/api/wiki/rel?id=${encodeURIComponent(id)}`);
}

/** Fetch the API-generated board, or a specific `.canvas` file when given. */
export function fetchWikiBoard(file?: string): Promise<unknown> {
  const suffix = file ? `?file=${encodeURIComponent(file)}` : "";
  return getJSON<unknown>(`/api/wiki/board${suffix}`);
}

export function isCanvas(
  res: DocResponse | CanvasResponse | MermaidResponse,
): res is CanvasResponse {
  return "canvas" in res;
}

export function isMermaid(
  res: DocResponse | CanvasResponse | MermaidResponse,
): res is MermaidResponse {
  return "source" in res;
}
