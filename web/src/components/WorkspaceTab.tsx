import {
  DockviewDefaultTab,
  type IDockviewDefaultTabProps,
  type IDockviewPanelHeaderProps,
} from "dockview-react";
import { isTabClosable } from "../lib/workspaceTabs";

/**
 * Default dockview tab: hides the close button for the side panels and the
 * placeholder tab; document tabs keep their close action.
 */
export function WorkspaceTab(props: IDockviewPanelHeaderProps) {
  const tabProps = props as IDockviewDefaultTabProps;
  return <DockviewDefaultTab {...tabProps} hideClose={!isTabClosable(props.api.id)} />;
}
