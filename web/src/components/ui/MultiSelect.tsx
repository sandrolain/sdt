import {
  Button,
  ListBox,
  ListBoxItem,
  Popover,
  Select as AriaSelect,
  type Key,
} from "react-aria-components";
import type { UiOption } from "./Select";

interface UiMultiSelectProps {
  options: UiOption[];
  selected: string[];
  onChange: (ids: string[]) => void;
  ariaLabel: string;
  placeholder?: string;
}

/** Untitled-UI-style multi select (React Aria) with a summary trigger value. */
export function MultiSelect({
  options,
  selected,
  onChange,
  ariaLabel,
  placeholder = "Any",
}: UiMultiSelectProps) {
  const summary =
    selected.length === 0
      ? placeholder
      : selected.length === 1
        ? (options.find((o) => o.id === selected[0])?.label ?? "1 selected")
        : `${selected.length} selected`;

  return (
    <AriaSelect
      aria-label={ariaLabel}
      selectionMode="multiple"
      value={selected}
      onChange={(keys: Key[]) => onChange(keys.map(String))}
      className="ui-select ui-select--multi"
    >
      <Button className="ui-select__button">
        <span className="ui-select__value">{summary}</span>
        <span className="ms-icon ui-select__chevron" aria-hidden="true">
          expand_more
        </span>
      </Button>
      <Popover className="ui-select__popover" offset={4}>
        <ListBox className="ui-select__list" items={options} selectionMode="multiple">
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
