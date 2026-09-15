export interface TreeEntry {
  path: string;
  /** omitted for .md entries without frontmatter kind */
  kind?: string;
  title?: string;
  summary?: string;
  created?: string;
  canvas?: boolean;
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

export interface ErrorResponse {
  error: string;
}

export interface SearchResult {
  path: string;
  /** omitted for .md entries without frontmatter kind */
  kind?: string;
  title?: string;
  summary?: string;
  created?: string;
  score: number;
  snippet: string;
  /** true for `.map.md` semantic map documents */
  isMap?: boolean;
  /** canonical map id, present when isMap */
  mapId?: string;
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
}

export interface SearchQuery {
  q: string;
  /** exact frontmatter kind filter; omitted when empty */
  kind?: string;
  /** inclusive frontmatter created-date bounds (YYYY-MM-DD); omitted when empty */
  from?: string;
  to?: string;
  limit?: number;
}

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ErrorResponse | null;
    throw new Error(body?.error ?? `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

export function fetchTree(): Promise<TreeResponse> {
  return getJSON("/api/tree");
}

export function fetchDoc(path: string): Promise<DocResponse | CanvasResponse> {
  return getJSON(`/api/doc?path=${encodeURIComponent(path)}`);
}

/** Compose the /api/search URL, omitting empty q/kind/from/to params. */
export function buildSearchUrl(query: SearchQuery): string {
  const params = new URLSearchParams();
  params.set("q", query.q);
  if (query.kind) params.set("kind", query.kind);
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.limit != null) params.set("limit", String(query.limit));
  return `/api/search?${params.toString()}`;
}

export function fetchSearch(query: SearchQuery): Promise<SearchResponse> {
  return getJSON(buildSearchUrl(query));
}

export function isCanvas(res: DocResponse | CanvasResponse): res is CanvasResponse {
  return "canvas" in res;
}
