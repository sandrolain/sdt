import type { Dispatch } from "react";
import { CLUSTER_KEYS, type ClusterKey } from "../lib/graphModel";
import { LAYOUTS, type LayoutKind } from "../lib/graphLayout";
import type { GraphToolsAction, GraphToolsState } from "../lib/graphTools";
import { Icon } from "../lib/icon";

interface GraphToolsPanelProps {
  tools: GraphToolsState;
  allVerbs: string[];
  allKinds: string[];
  selectedId: string | null;
  /** selected or hovered node; drives the Clear button's enabled state */
  focusedId?: string | null;
  selectedTitle: string | null;
  clusters: { id: string; color: string; count: number }[];
  onTools: Dispatch<GraphToolsAction>;
  onFit: () => void;
  onClear: () => void;
  onOpen: (id: string) => void;
}

export function GraphToolsPanel({
  tools,
  allVerbs,
  allKinds,
  selectedId,
  focusedId,
  selectedTitle,
  clusters,
  onTools,
  onFit,
  onClear,
  onOpen,
}: GraphToolsPanelProps) {
  return (
    <aside className="graph-tools" aria-label="Graph tools">
      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Mode</h3>
        <div className="graph-tools__row" role="group" aria-label="Graph mode">
          {(["2d", "3d"] as const).map((m) => (
            <button
              key={m}
              type="button"
              className={`graph-tools__chip${tools.mode === m ? " is-active" : ""}`}
              aria-pressed={tools.mode === m}
              onClick={() => onTools({ type: "mode", value: m })}
            >
              <Icon name={m === "2d" ? "grid_view" : "view_in_ar"} />
              {m.toUpperCase()}
            </button>
          ))}
        </div>
      </section>

      <section className="graph-tools__section">
        <label className="graph-tools__field">
          <span className="graph-tools__label">Layout</span>
          <select
            className="graph-tools__control"
            value={tools.layout}
            onChange={(e) => onTools({ type: "layout", value: e.target.value as LayoutKind })}
          >
            {LAYOUTS.map((l) => (
              <option key={l.id} value={l.id}>
                {l.label}
              </option>
            ))}
          </select>
        </label>
        <label className="graph-tools__field">
          <span className="graph-tools__label">Cluster</span>
          <select
            className="graph-tools__control"
            value={tools.clusterKey}
            onChange={(e) => onTools({ type: "clusterKey", value: e.target.value as ClusterKey })}
          >
            {CLUSTER_KEYS.map((c) => (
              <option key={c.id} value={c.id}>
                {c.label}
              </option>
            ))}
          </select>
        </label>
        <label className="graph-tools__check">
          <input
            type="checkbox"
            checked={tools.showLabels}
            onChange={(e) => onTools({ type: "labels", value: e.target.checked })}
          />
          <span>Labels</span>
        </label>
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Relations</h3>
        <ul className="graph-tools__list" role="list">
          {allVerbs.map((verb) => (
            <li key={verb}>
              <label className="graph-tools__check">
                <input
                  type="checkbox"
                  checked={!tools.hiddenVerbs.includes(verb)}
                  onChange={() => onTools({ type: "toggleVerb", value: verb })}
                />
                <span>{verb}</span>
              </label>
            </li>
          ))}
        </ul>
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Edge kind</h3>
        <ul className="graph-tools__list" role="list">
          {allKinds.map((kind) => (
            <li key={kind}>
              <label className="graph-tools__check">
                <input
                  type="checkbox"
                  checked={!tools.hiddenKinds.includes(kind)}
                  onChange={() => onTools({ type: "toggleKind", value: kind })}
                />
                <span>{kind}</span>
              </label>
            </li>
          ))}
        </ul>
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Legend</h3>
        <ul className="graph-tools__list" role="list">
          {clusters.map((c) => (
            <li key={c.id} className="graph-tools__legend">
              <span className="graph-tools__swatch" style={{ background: c.color }} />
              <span>
                {c.id} ({c.count})
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section className="graph-tools__section graph-tools__section--actions">
        <button type="button" className="graph-tools__button" onClick={onFit} title="Fit graph to view">
          <Icon name="center_focus_strong" />
          Fit
        </button>
        <button
          type="button"
          className="graph-tools__button"
          onClick={onClear}
          disabled={!focusedId}
          title="Clear selection and re-fit"
        >
          <Icon name="clear" />
          Clear
        </button>
      </section>

      {selectedId && (
        <section className="graph-tools__section graph-tools__section--selected">
          <h3 className="graph-tools__title">Selected</h3>
          <p className="graph-tools__selected-title">{selectedTitle ?? selectedId}</p>
          <button type="button" className="graph-tools__button" onClick={() => onOpen(selectedId)}>
            <Icon name="open_in_new" />
            Open page
          </button>
        </section>
      )}
    </aside>
  );
}
