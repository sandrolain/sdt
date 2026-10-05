/**
 * BoardControls — the right-docked sidebar content for the JSON Canvas board.
 *
 * Every board control lives here, never over the canvas (analysis Behaviour
 * decision 3): the source selector, zoom out / fit / zoom in, the 2D/3D toggle,
 * the per-layer visibility switches and the minimap toggle. It only dispatches
 * into the view through props.
 */
import { Icon } from "../lib/icon";
import { Select } from "./ui/Select";
import { Switch } from "./ui/Switch";

export interface BoardLayer {
  id: number;
  name: string;
}

/** One step of the nested-canvas breadcrumb. */
export interface BoardCrumb {
  id: string;
  label: string;
}

export interface BoardControlsProps {
  sources: { path: string; label: string }[];
  source: string;
  onSource: (path: string) => void;
  crumbs: BoardCrumb[];
  onCrumb: (id: string | null) => void;
  zoom: number;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
  mode: "2d" | "3d";
  onMode: (mode: "2d" | "3d") => void;
  layers: BoardLayer[];
  hiddenLayers: number[];
  onToggleLayer: (id: number) => void;
  showMinimap: boolean;
  onShowMinimap: (value: boolean) => void;
}

export function BoardControls({
  sources,
  source,
  onSource,
  crumbs,
  onCrumb,
  zoom,
  onZoomIn,
  onZoomOut,
  onFit,
  mode,
  onMode,
  layers,
  hiddenLayers,
  onToggleLayer,
  showMinimap,
  onShowMinimap,
}: BoardControlsProps) {
  return (
    <div className="graph-controls board-controls">
      <section className="graph-tools__section">
        <label className="graph-tools__field">
          <span className="graph-tools__label">Source</span>
          <Select
            ariaLabel="Board source"
            options={[
              { id: "", label: "Wiki graph (default)" },
              ...sources.map((s) => ({ id: s.path, label: s.label })),
            ]}
            selectedKey={source}
            onSelectionChange={(key) => onSource(String(key))}
          />
        </label>
        <p className="graph-tools__hint">Read-only · pan by drag, zoom with the wheel</p>
      </section>

      {crumbs.length > 1 && (
        <section className="graph-tools__section">
          <h3 className="graph-tools__title">Path</h3>
          <nav className="board-breadcrumb" aria-label="Board path">
            {crumbs.map((c, i) => (
              <span key={c.id || "root"} className="board-breadcrumb__item">
                {i > 0 && (
                  <span className="board-breadcrumb__sep" aria-hidden="true">
                    /
                  </span>
                )}
                <button
                  type="button"
                  className="board-breadcrumb__btn"
                  onClick={() => onCrumb(c.id)}
                  disabled={i === crumbs.length - 1}
                >
                  {c.label}
                </button>
              </span>
            ))}
          </nav>
        </section>
      )}

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">View</h3>
        <div className="graph-tools__row">
          <button
            type="button"
            className="graph-tools__button"
            onClick={onZoomOut}
            title="Zoom out"
          >
            <Icon name="remove" />
            Out
          </button>
          <span className="graph-tools__readout" aria-live="polite">
            {Math.round(zoom * 100)}%
          </span>
          <button type="button" className="graph-tools__button" onClick={onZoomIn} title="Zoom in">
            <Icon name="add" />
            In
          </button>
        </div>
        <button type="button" className="graph-tools__button" onClick={onFit} title="Fit to view">
          <Icon name="center_focus_strong" />
          Fit
        </button>
        <Switch isSelected={showMinimap} onChange={onShowMinimap}>
          Minimap
        </Switch>
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Mode</h3>
        <div className="graph-tools__row" role="group" aria-label="Board mode">
          {(["2d", "3d"] as const).map((m) => (
            <button
              key={m}
              type="button"
              className={`graph-tools__chip${mode === m ? " is-active" : ""}`}
              aria-pressed={mode === m}
              onClick={() => onMode(m)}
            >
              <Icon name={m === "2d" ? "grid_view" : "view_in_ar"} />
              {m.toUpperCase()}
            </button>
          ))}
        </div>
      </section>

      <section className="graph-tools__section">
        <h3 className="graph-tools__title">Layers</h3>
        {layers.map((l) => (
          <Switch
            key={l.id}
            isSelected={!hiddenLayers.includes(l.id)}
            onChange={() => onToggleLayer(l.id)}
          >
            {l.name}
          </Switch>
        ))}
      </section>
    </div>
  );
}
