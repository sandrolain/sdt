import type { ReactNode } from "react";
import { Switch as AriaSwitch, type SwitchProps } from "react-aria-components";

interface UiSwitchProps extends Omit<SwitchProps, "children" | "className"> {
  children?: ReactNode;
  className?: string;
}

/**
 * Untitled-UI-style toggle built on React Aria. Replaces raw checkboxes for
 * boolean controls, styled with the Catppuccin tokens (no Tailwind).
 */
export function Switch({ children, className, ...props }: UiSwitchProps) {
  return (
    <AriaSwitch {...props} className={["ui-switch", className].filter(Boolean).join(" ")}>
      {({ isSelected }) => (
        <>
          <span className="ui-switch__track" data-selected={isSelected || undefined}>
            <span className="ui-switch__thumb" />
          </span>
          {children != null && <span className="ui-switch__label">{children}</span>}
        </>
      )}
    </AriaSwitch>
  );
}
