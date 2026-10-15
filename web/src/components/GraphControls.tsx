/**
 * GraphControls — the left-docked tools/filters sidebar content for the graph
 * surface.
 *
 * Four collapsible groups (analysis B7/D8): Surface, Explore, Filters (the legend)
 * and Actions. It only dispatches into the view — the canvas itself carries no
 * chrome. The legend is the only filter surface (B4): cluster groups, relations
 * and edge kinds, all driving the engine's dimming filters.
 *
 * On the docs map a control whose input cannot change the current mode is disabled
 * with its reason, or absent when it can never act at all: the decisions come from
 * the capability map in `lib/semantic/capability.ts`, never from an ad-hoc
 * conditional here (analysis 20261013-063009, D1).
 */
import { useState, type Dispatch, type ReactNode } from "react";
import { CLUSTER_KEYS, type ClusterKey } from "../lib/graphModel";
import { LAYOUT_OPTIONS } from "../lib/graph/layout";
import type { GraphLayout } from "../lib/graph/types";
import {
  SEMANTIC_MODE_OPTIONS,
  semanticCapabilityHidden,
  semanticCapabilityReason,
  semanticVerbLabel,
  type SemanticMode,
} from "../lib/semantic";
import type { GraphToolsAction, GraphToolsState } from "../lib/graphTools";
import { Icon } from "../lib/icon";
import { GuideHotspot, GuideModal } from "./GraphGuide";
import { Select } from "./ui/Select";
import { ComboBox } from "./ui/ComboBox";
import { Switch } from "./ui/Switch";

export interface GraphControlsProps {
  tools: GraphToolsState;
  allVerbs: string[];
  allKinds: string[];
  clusters: { id: string; color: string; count: number }[];
  selectedId: string | null;
  nodeOptions: { id: string; label: string }[];
  pathFrom: string;
  pathTo: string;
  pathActive: boolean;
  /** The group whose legend row is hovered, if any: the other rows dim and, through
   * the workspace, the map dims the clusters that are not it. */
  hoverGroup?: string | null;
  onHoverGroup?: (id: string | null) => void;
  /** Present only on the docs semantic map: the mode control's state/handler. */
  semantic?: { mode: SemanticMode; onChange: (mode: SemanticMode) => void };
  onTools: Dispatch<GraphToolsAction>;
  onPathFrom: (id: string) => void;
  onPathTo: (id: string) => void;
  onFindPath: () => void;
  onClearPath: () => void;
  onFit: () => void;
  onClear: () => void;
  onExportSVG: () => void;
  onExportPNG: () => void;
}

/** One collapsible tools group; its open state is local view furniture (D8). */
function ToolSection({
  title,
  action,
  guide,
  children,
}: {
  title: string;
  action?: ReactNode;
  /** The section's `(?)` hotspot, shown while the inline guide is on. */
  guide?: ReactNode;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(true);
  return (
    <section className="graph-tools__section">
      <div className="graph-tools__header">
        <button
          type="button"
          className={`graph-tools__section-toggle${open ? "" : " is-collapsed"}`}
          aria-expanded={open}
          aria-label={title}
          onClick={() => setOpen((o) => !o)}
        >
          <span className="ms-icon graph-tools__caret" aria-hidden="true">
            expand_more
          </span>
          {title}
        </button>
        {guide}
        {action}
      </div>
      {open ? <div className="graph-tools__body">{children}</div> : null}
    </section>
  );
}

export function GraphControls({
  tools,
  allVerbs,
  allKinds,
  clusters,
  selectedId,
  nodeOptions,
  pathFrom,
  pathTo,
  pathActive,
  hoverGroup = null,
  onHoverGroup,
  semantic,
  onTools,
  onPathFrom,
  onPathTo,
  onFindPath,
  onClearPath,
  onFit,
  onClear,
  onExportSVG,
  onExportPNG,
}: GraphControlsProps) {
  // `null` is the wiki graph: it has no semantic mode, every control acts, and the
  // capability map answers `true` for everything (see lib/semantic/capability.ts).
  const mode: SemanticMode | null = semantic?.mode ?? null;
  // The inline guide is opt-in view furniture: off by default, never persisted, and
  // offered only where there is a guide to read (analysis 20261013-063009, O6).
  const [guide, setGuide] = useState(false);
  const [guideOpen, setGuideOpen] = useState(false);
  const openGuide = () => setGuideOpen(true);
  const layoutReason = semanticCapabilityReason("layout", mode);
  const exploreReason = semanticCapabilityReason("explore", mode);
  const relationsReason = semanticCapabilityReason("relations", mode);
  return (
    <div className="graph-controls">
      <ToolSection
        title="Surface"
        guide={<GuideHotspot anchor="surface" enabled={guide} onOpen={openGuide} />}
      >
        {semantic && (
          <div
            className="graph-tools__row graph-tools__row--wrap"
            role="group"
            aria-label="Semantic mode"
          >
            {SEMANTIC_MODE_OPTIONS.map((m) => (
              <button
                key={m.id}
                type="button"
                className={`graph-tools__chip${semantic.mode === m.id ? " is-active" : ""}`}
                aria-pressed={semantic.mode === m.id}
                onClick={() => semantic.onChange(m.id)}
              >
                {m.label}
              </button>
            ))}
          </div>
        )}
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
        <label className="graph-tools__field" title={layoutReason ?? undefined}>
          <span className="graph-tools__label">Layout</span>
          <Select
            ariaLabel="Layout"
            isDisabled={layoutReason !== null}
            options={LAYOUT_OPTIONS.map((l) => ({ id: l.id, label: l.label }))}
            selectedKey={tools.layout}
            onSelectionChange={(key) =>
              onTools({ type: "layout", value: String(key) as GraphLayout })
            }
          />
          {layoutReason && <span className="graph-tools__note">{layoutReason}</span>}
        </label>
        {!semanticCapabilityHidden("clusterKey", mode) && (
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
        )}
        <Switch
          aria-label="Labels"
          isSelected={tools.showLabels}
          onChange={(isSelected) => onTools({ type: "labels", value: isSelected })}
        >
          Labels
        </Switch>
        {semantic && (
          <Switch
            aria-label="Guide"
            isSelected={guide}
            onChange={(isSelected) => {
              setGuide(isSelected);
              if (!isSelected) setGuideOpen(false);
            }}
          >
            Guide
          </Switch>
        )}
      </ToolSection>

      <ToolSection
        title="Explore"
        guide={<GuideHotspot anchor="explore" enabled={guide} onOpen={openGuide} />}
      >
        {exploreReason && <p className="graph-tools__note">{exploreReason}</p>}
        <Switch
          aria-label="Hub"
          isSelected={tools.centrality}
          isDisabled={exploreReason !== null}
          onChange={(isSelected) => onTools({ type: "centrality", value: isSelected })}
        >
          Hub
        </Switch>
        <Switch
          aria-label="Neighbors"
          isSelected={tools.neighborsOnly}
          isDisabled={!selectedId || exploreReason !== null}
          onChange={(isSelected) => onTools({ type: "neighbors", value: isSelected })}
        >
          Neighbors
        </Switch>
        <label className="graph-tools__field">
          <span className="graph-tools__label">From</span>
          <ComboBox
            ariaLabel="Path from"
            isDisabled={exploreReason !== null}
            options={nodeOptions}
            selectedKey={pathFrom || null}
            onSelectionChange={(key) => onPathFrom(String(key))}
          />
        </label>
        <label className="graph-tools__field">
          <span className="graph-tools__label">To</span>
          <ComboBox
            ariaLabel="Path to"
            isDisabled={exploreReason !== null}
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
            disabled={!pathFrom || !pathTo || exploreReason !== null}
            title={exploreReason ?? "Highlight the shortest path"}
          >
            <Icon name="route" />
            Find
          </button>
          <button
            type="button"
            className="graph-tools__button"
            onClick={onClearPath}
            disabled={!pathActive || exploreReason !== null}
            title={exploreReason ?? "Clear the highlighted path"}
          >
            <Icon name="clear" />
            Clear path
          </button>
        </div>
      </ToolSection>

      <ToolSection
        title="Filters"
        guide={<GuideHotspot anchor="filters" enabled={guide} onOpen={openGuide} />}
        action={
          <button
            type="button"
            className="graph-tools__reset"
            onClick={() => onTools({ type: "clearFilters" })}
          >
            Reset
          </button>
        }
      >
        {semantic && !semanticCapabilityHidden("relations", mode) && (
          <>
            <p className="graph-tools__subtitle">Semantic proximity</p>
            <ul className="graph-tools__list" role="list">
              {allVerbs.map((verb) => (
                <li key={verb}>
                  <label className="graph-tools__legend" title={relationsReason ?? undefined}>
                    <input
                      type="checkbox"
                      aria-label={semanticVerbLabel(verb)}
                      disabled={relationsReason !== null}
                      checked={!tools.hiddenRelations.includes(verb)}
                      onChange={() => onTools({ type: "toggleRelation", value: verb })}
                    />
                    <span>{semanticVerbLabel(verb)}</span>
                  </label>
                </li>
              ))}
            </ul>
          </>
        )}
        <p className="graph-tools__subtitle">Groups</p>
        <ul className="graph-tools__list" role="list">
          {clusters.map((c) => (
            <li key={c.id}>
              <label
                className={`graph-tools__legend${
                  hoverGroup && hoverGroup !== c.id ? " is-dimmed" : ""
                }`}
                onPointerEnter={() => onHoverGroup?.(c.id)}
                onPointerLeave={() => onHoverGroup?.(null)}
              >
                <input
                  type="checkbox"
                  aria-label={`${c.id} (${c.count})`}
                  checked={!tools.hiddenGroups.includes(c.id)}
                  onChange={() => onTools({ type: "toggleGroup", value: c.id })}
                />
                <span className="graph-tools__swatch" style={{ background: c.color }} />
                <span>
                  {c.id} ({c.count})
                </span>
              </label>
            </li>
          ))}
        </ul>
        {!semanticCapabilityHidden("edgeKinds", mode) && (
          <>
            <p className="graph-tools__subtitle">Relations</p>
            <ul className="graph-tools__list" role="list">
              {allVerbs.map((verb) => (
                <li key={verb}>
                  <label className="graph-tools__legend">
                    <input
                      type="checkbox"
                      aria-label={verb}
                      checked={!tools.hiddenRelations.includes(verb)}
                      onChange={() => onTools({ type: "toggleRelation", value: verb })}
                    />
                    <span>{verb}</span>
                  </label>
                </li>
              ))}
            </ul>
            <p className="graph-tools__subtitle">Edge kinds</p>
            <ul className="graph-tools__list" role="list">
              {allKinds.map((kind) => (
                <li key={kind}>
                  <label className="graph-tools__legend">
                    <input
                      type="checkbox"
                      aria-label={kind}
                      checked={!tools.hiddenKinds.includes(kind)}
                      onChange={() => onTools({ type: "toggleKind", value: kind })}
                    />
                    <span>{kind}</span>
                  </label>
                </li>
              ))}
            </ul>
          </>
        )}
      </ToolSection>

      <ToolSection
        title="Actions"
        guide={<GuideHotspot anchor="actions" enabled={guide} onOpen={openGuide} />}
      >
        <div className="graph-tools__actions">
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
        </div>
      </ToolSection>
      <GuideModal open={guideOpen} onClose={() => setGuideOpen(false)} />
    </div>
  );
}
