import { useEffect, useState } from "react";
import { NavLink } from "react-router-dom";
import { fetchWikiRel, type RelResponse } from "../lib/api";
import { groupRelations, relationCounts } from "../lib/relations";
import { Icon } from "../lib/icon";
import { SkeletonLines } from "./Skeleton";
import { displayTitle } from "../lib/titles";

interface RelatedPanelProps {
  /** wiki page id */
  id: string;
}

/** Inbound/outbound relations grouped by verb, with direction arrows. */
export function RelatedPanel({ id }: RelatedPanelProps) {
  const [resp, setResp] = useState<RelResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    fetchWikiRel(id)
      .then((r) => {
        if (alive) setResp(r);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [id]);

  const counts = resp ? relationCounts(resp) : null;

  return (
    <div className="related" aria-label="Related pages">
      {counts && (
        <span className="related-counts">
          {counts.outbound} out · {counts.inbound} in
        </span>
      )}
      {error ? (
        <p className="content__empty">relations error: {error}</p>
      ) : !resp ? (
        <SkeletonLines count={3} label="Loading relations" />
      ) : counts && counts.inbound + counts.outbound === 0 ? (
        <p className="content__empty">No relations.</p>
      ) : (
        <div className="related-groups">
          {groupRelations(resp).map((group) => (
            <section key={group.verb} className="related-group">
              <h3 className="related-group__verb">{group.verb}</h3>
              <ul className="related-list" role="list">
                {group.outbound.map((item) => (
                  <li key={`out-${item.id}`}>
                    <NavLink className="related-item" to={`/wiki/${item.id}`}>
                      <Icon name="arrow_outward" label="outbound" className="related-item__arrow" />
                      <span className="related-item__title">
                        {displayTitle({ title: item.title, path: item.id })}
                      </span>
                      <span className="related-item__kind">{item.kind}</span>
                    </NavLink>
                  </li>
                ))}
                {group.inbound.map((item) => (
                  <li key={`in-${item.id}`}>
                    <NavLink className="related-item" to={`/wiki/${item.id}`}>
                      <Icon name="arrow_back" label="inbound" className="related-item__arrow" />
                      <span className="related-item__title">
                        {displayTitle({ title: item.title, path: item.id })}
                      </span>
                      <span className="related-item__kind">{item.kind}</span>
                    </NavLink>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
