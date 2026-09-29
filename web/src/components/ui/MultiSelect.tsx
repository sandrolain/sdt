import {
  Button,
  Header,
  ListBox,
  ListBoxItem,
  ListBoxSection,
  Popover,
  Select as AriaSelect,
  type Key,
} from "react-aria-components";
import type { UiOption } from "./Select";

export interface UiOptionSection {
  id: string;
  label: string;
  options: UiOption[];
}

interface UiMultiSelectProps {
  /** flat options; ignored when `sections` is given */
  options?: UiOption[];
  /** grouped options, rendered as list sections in the given order */
  sections?: UiOptionSection[];
  selected: string[];
  onChange: (ids: string[]) => void;
  ariaLabel: string;
  placeholder?: string;
  /** summary shown when nothing is selected; defaults to the placeholder */
  emptyLabel?: string;
}

/** Untitled-UI-style multi select (React Aria) with a summary trigger value,
 *  optionally rendering its options in labelled groups. */
export function MultiSelect({
  options,
  sections,
  selected,
  onChange,
  ariaLabel,
  placeholder = "Any",
  emptyLabel,
}: UiMultiSelectProps) {
  const all = sections ? sections.flatMap((section) => section.options) : (options ?? []);
  const summary =
    selected.length === 0
      ? (emptyLabel ?? placeholder)
      : selected.length === 1
        ? (all.find((o) => o.id === selected[0])?.label ?? "1 selected")
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
        {sections ? (
          <ListBox className="ui-select__list" selectionMode="multiple">
            {sections.map((section) => (
              <ListBoxSection key={section.id} id={section.id} aria-label={section.label}>
                <Header className="ui-select__section-header">{section.label}</Header>
                {section.options.map((item) => (
                  <ListBoxItem
                    key={item.id}
                    className="ui-select__item"
                    id={item.id}
                    textValue={item.label}
                  >
                    {item.label}
                  </ListBoxItem>
                ))}
              </ListBoxSection>
            ))}
          </ListBox>
        ) : (
          <ListBox className="ui-select__list" items={all} selectionMode="multiple">
            {(item) => (
              <ListBoxItem className="ui-select__item" id={item.id} textValue={item.label}>
                {item.label}
              </ListBoxItem>
            )}
          </ListBox>
        )}
      </Popover>
    </AriaSelect>
  );
}
