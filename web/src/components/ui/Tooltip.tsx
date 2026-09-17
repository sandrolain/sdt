import type { ReactNode } from "react";
import { Button, Tooltip, TooltipTrigger, type ButtonProps } from "react-aria-components";

interface TooltipButtonProps extends Omit<ButtonProps, "children" | "aria-label"> {
  /** accessible name (used for tests and screen readers) */
  label: string;
  /** tooltip text shown on hover/focus */
  tooltip: string;
  children: ReactNode;
}

/**
 * Icon button with a react-aria tooltip: an accessible name plus a hover/focus
 * tooltip that is not a plain `title` (and therefore keyboard reachable).
 */
export function TooltipButton({ label, tooltip, children, ...rest }: TooltipButtonProps) {
  return (
    <TooltipTrigger delay={300}>
      <Button {...rest} aria-label={label} className={rest.className}>
        {children}
      </Button>
      <Tooltip className="ui-tooltip">{tooltip}</Tooltip>
    </TooltipTrigger>
  );
}
