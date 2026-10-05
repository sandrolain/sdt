/**
 * GraphNodeDetail — the always-present node detail docked panel (B3).
 *
 * Reference parity plus SDT metadata (B10): the title, the cluster chip, the
 * summary, the neighbours grouped by verb with in/out direction (click selects)
 * and "Open page", plus the node's type, status, first tags and path. Shows an
 * explicit empty state until a node is selected.
 */
import type { GraphNodeInput } from "../lib/graph/types";
import type { GraphNeighbourGroup } from "../lib/graph/neighbours";
import { Icon } from "../lib/icon";

export interface GraphNodeDetailProps {
  node: GraphNodeInput | null;
  neighbours: GraphNeighbourGroup[];
  onSelect: (id: string) => void;
  onOpen: (id: string) => void;
}

export function GraphNodeDetail({ node, neighbours, onSelect, onOpen }: GraphNodeDetailProps) {
  if (!node) {
    return (
      <div className="graph-detail">
        <p className="graph-detail__empty">Select a node</p>
      </div>
    );
  }
  const tags = Array.isArray(node.tags) ? (node.tags as string[]) : [];
  return (
    <div className="graph-detail">
      <section className="graph-detail__section">
        <h3 className="graph-detail__name">{String(node.label ?? node.id)}</h3>
        {node.group ? (
          <span className="graph-detail__chip">
            <span className="graph-detail__swatch" style={{ background: node.color }} />
            {String(node.group)}
          </span>
        ) : null}
        {node.description ? (
          <p className="graph-detail__summary">{String(node.description)}</p>
        ) : null}
        <button type="button" className="graph-tools__button" onClick={() => onOpen(node.id)}>
          <Icon name="open_in_new" />
          Open page
        </button>
      </section>

      {neighbours.length > 0 ? (
        <section className="graph-detail__section">
          <h3 className="graph-detail__title">Neighbours</h3>
          {neighbours.map((group) => (
            <div key={group.verb} className="graph-detail__group">
              <p className="graph-detail__verb">{group.verb}</p>
              <ul className="graph-tools__list" role="list">
                {group.items.map((n) => (
                  <li key={`${group.verb}-${n.direction}-${n.id}`}>
                    <button
                      type="button"
                      className="graph-detail__link"
                      onClick={() => onSelect(n.id)}
                    >
                      <span aria-hidden="true">{n.direction === "out" ? "→" : "←"}</span>
                      {n.label}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>
      ) : null}

      <section className="graph-detail__section">
        <h3 className="graph-detail__title">Metadata</h3>
        <dl className="graph-detail__meta">
          {node.type ? (
            <>
              <dt>Type</dt>
              <dd>{String(node.type)}</dd>
            </>
          ) : null}
          {node.status ? (
            <>
              <dt>Status</dt>
              <dd>{String(node.status)}</dd>
            </>
          ) : null}
          {tags.length > 0 ? (
            <>
              <dt>Tags</dt>
              <dd>{tags.slice(0, 4).join(", ")}</dd>
            </>
          ) : null}
          {node.path ? (
            <>
              <dt>Path</dt>
              <dd className="graph-detail__path">{String(node.path)}</dd>
            </>
          ) : null}
        </dl>
      </section>
    </div>
  );
}
