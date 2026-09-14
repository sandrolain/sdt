export interface TreeEntry {
  path: string;
  /** omitted for .md entries without frontmatter kind */
  kind?: string;
  title?: string;
  summary?: string;
  created?: string;
  canvas?: boolean;
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

export function isCanvas(res: DocResponse | CanvasResponse): res is CanvasResponse {
  return "canvas" in res;
}
