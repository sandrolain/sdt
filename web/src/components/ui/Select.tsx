import {
  Button,
  ListBox,
  ListBoxItem,
  Popover,
  Select as AriaSelect,
  SelectValue,
  type SelectProps,
} from "react-aria-components";

export interface UiOption {
  id: string;
  label: string;
}

interface UiSelectProps extends Omit<
  SelectProps<UiOption>,
  "children" | "className" | "aria-label"
> {
  options: UiOption[];
  ariaLabel: string;
  className?: string;
  /** icon + value only, for toolbars */
  compact?: boolean;
}

/** Untitled-UI-style single select (React Aria) styled with Catppuccin tokens. */
export function Select({ options, ariaLabel, className, compact, ...props }: UiSelectProps) {
  return (
    <AriaSelect
      {...props}
      aria-label={ariaLabel}
      className={["ui-select", compact ? "ui-select--compact" : "", className]
        .filter(Boolean)
        .join(" ")}
    >
      <Button className="ui-select__button">
        <SelectValue className="ui-select__value" />
        <span className="ms-icon ui-select__chevron" aria-hidden="true">
          expand_more
        </span>
      </Button>
      <Popover className="ui-select__popover" offset={4}>
        <ListBox className="ui-select__list" items={options}>
          {(item) => (
            <ListBoxItem className="ui-select__item" id={item.id} textValue={item.label}>
              {item.label}
            </ListBoxItem>
          )}
        </ListBox>
      </Popover>
    </AriaSelect>
  );
}
