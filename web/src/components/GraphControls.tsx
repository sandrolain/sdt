/**
 * GraphControls — the right-docked sidebar content for the graph surface.
 *
 * Replaces `GraphToolsPanel` (analysis B2/D2): mode, layout, cluster, labels,
 * centrality (Hub), neighborhood (Neighbors), verb/kind filters, the path
 * finder, export and the legend/detail. It only dispatches into the view — the
 * canvas itself carries no chrome.
 */
import type { Dispatch } from "react";
import { CLUSTER_KEYS, type ClusterKey } from "../lib/graphModel";
import { LAYOUT_OPTIONS } from "../lib/graph/layout";
import type { GraphLayout } from "../lib/graph/types";
import type { GraphToolsAction, GraphToolsState } from "../lib/graphTools";
import { Icon } from "../lib/icon";
import { Select } from "./ui/Select";
import { MultiSelect } from "./ui/MultiSelect";
import { Switch } from "./ui/Switch";

export interface GraphControlsProps {
  tools: GraphToolsState;
  allVerbs: string[];
  allKinds: string[];
  clusters: { id: string; color: string; count: number }[];
  selectedId: string | null;
  selectedTitle: string | null;
  nodeOptions: { id: string; label: string }[];
  pathFrom: string;
  pathTo: string;
  pathActive: boolean;
  onTools: Dispatch<GraphToolsAction>;
  onPathFrom: (id: string) => void;
  onPathTo: (id: string) => void;
  onFindPath: () => void;
  onClearPath: () => void;
  onFit: () => void;
  onClear: () => void;
  onOpen: (id: string) => void;
  onExportSVG: () => void;
  onExportPNG: () => void;
}

export function GraphControls({
  tools,
  allVerbs,
  allKinds,
  clusters,
  selectedId,
  selectedTitle,
  nodeOptions,
  pathFrom,
  pathTo,
  pathActive,
  onTools,
  onPathFrom,
  onPathTo,
  onFindPath,
  onClearPath,
  onFit,
  onClear,
  onOpen,
  onExportSVG,
  onExportPNG,
}: GraphControlsProps) {
  return (
    <div className="graph-controls">
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
          <Select
            ariaLabel="Layout"
            options={LAYOUT_OPTIONS.map((l) => ({ id: l.id, label: l.label }))}
            selectedKey={tools.layout}
            onSelectionChange={(key) =>
              onTools({ type: "layout", value: String(key) as GraphLayout })
            }
          />
        </label>
        <label className="graph-tools__field">
          <span className="graph-tools__label">Cluster</span>
          <Select
            ariaLabel="Cluster"
            options={CLUSTER_KEYS.map((c) => ({ id: c.id, label: c.label }))}
            selectedKey={tools.clusterKey}
            onSelectionChange={(key) =>
              onTools({ type: "clusterKey", value: String(key) as ClusterKey })
            }
          />
        </label>
        <Switch
          isSelected={tools.showLabels}
          onChange={(isSelected) => onTools({ type: "labels", value: isSelected })}
        >
          Labels
        </Switch>
        <Switch
          isSelected={tools.centrality}
          onChange={(isSelected) => onTools({ type: "centrality", value: isSelected })}
        >
          Hub
        </Switch>
        <Switch
          isSelected={tools.neighborsOnly}
          isDisabled={!selectedId}
          onChange={(isSelected) => onTools({ type: "neighbors", value: isSelected })}
        >
          Neighbors
        </Switch>
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Relations</h3>
        <MultiSelect
          ariaLabel="Visible relations"
          options={allVerbs.map((verb) => ({ id: verb, label: verb }))}
          selected={allVerbs.filter((v) => !tools.hiddenVerbs.includes(v))}
          onChange={(ids) =>
            onTools({ type: "setHiddenVerbs", value: allVerbs.filter((v) => !ids.includes(v)) })
          }
        />
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Edge kind</h3>
        <MultiSelect
          ariaLabel="Visible edge kinds"
          options={allKinds.map((kind) => ({ id: kind, label: kind }))}
          selected={allKinds.filter((k) => !tools.hiddenKinds.includes(k))}
          onChange={(ids) =>
            onTools({ type: "setHiddenKinds", value: allKinds.filter((k) => !ids.includes(k)) })
          }
        />
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Path</h3>
        <label className="graph-tools__field">
          <span className="graph-tools__label">From</span>
          <Select
            ariaLabel="Path from"
            options={nodeOptions}
            selectedKey={pathFrom || null}
            onSelectionChange={(key) => onPathFrom(String(key))}
          />
        </label>
        <label className="graph-tools__field">
          <span className="graph-tools__label">To</span>
          <Select
            ariaLabel="Path to"
            options={nodeOptions}
            selectedKey={pathTo || null}
            onSelectionChange={(key) => onPathTo(String(key))}
          />
        </label>
        <div className="graph-tools__row">
          <button
            type="button"
            className="graph-tools__button"
            onClick={onFindPath}
            disabled={!pathFrom || !pathTo}
            title="Highlight the shortest path"
          >
            <Icon name="route" />
            Find
          </button>
          <button
            type="button"
            className="graph-tools__button"
            onClick={onClearPath}
            disabled={!pathActive}
            title="Clear the highlighted path"
          >
            <Icon name="clear" />
            Clear path
          </button>
        </div>
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
        <button
          type="button"
          className="graph-tools__button"
          onClick={onFit}
          title="Fit graph to view"
        >
          <Icon name="center_focus_strong" />
          Fit
        </button>
        <button
          type="button"
          className="graph-tools__button"
          onClick={onClear}
          disabled={!selectedId}
          title="Clear selection and re-fit"
        >
          <Icon name="clear" />
          Clear
        </button>
        <button
          type="button"
          className="graph-tools__button"
          onClick={onExportSVG}
          title="Export as SVG"
        >
          <Icon name="download" />
          SVG
        </button>
        <button
          type="button"
          className="graph-tools__button"
          onClick={onExportPNG}
          title="Export as PNG"
        >
          <Icon name="download" />
          PNG
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
    </div>
  );
}
