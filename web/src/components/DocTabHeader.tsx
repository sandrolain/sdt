import type { IDockviewPanelHeaderProps } from "dockview-react";
import { displayTitle } from "../lib/titles";
import { useCorpusKind } from "../lib/corpusIndex";
import { kindColor, kindIcon } from "../lib/kinds";
import { Icon } from "../lib/icon";

/**
 * Tab for an open document: coloured kind glyph + title + close action. The
 * close handling mirrors dockview's default tab so middle/right-click and the
 * context menu keep working through the surrounding `.dv-tab` wrapper.
 */
export function DocTabHeader(props: IDockviewPanelHeaderProps) {
  const params = props.params as { path?: string } | undefined;
  const path = String(params?.path ?? "");
  const kind = useCorpusKind(path);

  return (
    <div className="dv-default-tab dock-doc-tab" data-testid="dockview-dv-default-tab">
      <Icon
        name={kindIcon(kind)}
        className="dock-doc-tab__icon"
        style={{ color: kindColor(kind) }}
        label={`kind: ${kind}`}
      />
      <span className="dv-default-tab-content">{displayTitle({ path })}</span>
      <button
        type="button"
        className="dv-default-tab-action"
        aria-label="Close tab"
        onPointerDown={(event) => event.preventDefault()}
        onClick={(event) => {
          event.preventDefault();
          props.api.close();
        }}
      >
        <Icon name="close" />
      </button>
    </div>
  );
}
