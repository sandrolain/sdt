import { Navigate, useParams } from "react-router-dom";

/**
 * Legacy `#/wiki/<id>` route: the wiki keeps only graph/board, so a wiki page
 * opens as a documents tab instead.
 */
export function WikiPageDetail() {
  const id = useParams()["*"] ?? "";
  return <Navigate to={`/docs/context/wiki/${id}.md`} replace />;
}
