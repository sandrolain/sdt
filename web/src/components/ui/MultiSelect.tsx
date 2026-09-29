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

/** A one-click shortcut that sets the whole selection to a known id set. */
export interface UiOptionPreset {
  id: string;
  label: string;
  ids: string[];
}

interface UiMultiSelectProps {
  /** flat options; ignored when `sections` is given */
  options?: UiOption[];
  /** grouped options, rendered as list sections in the given order */
  sections?: UiOptionSection[];
  /** quick selections rendered above the list, each replacing the selection */
  presets?: UiOptionPreset[];
  selected: string[];
  onChange: (ids: string[]) => void;
  ariaLabel: string;
  placeholder?: string;
  /** summary shown when nothing is selected; defaults to the placeholder */
  emptyLabel?: string;
  /** summary shown when everything is selected; defaults to the counted one */
  allLabel?: string;
}

/** Untitled-UI-style multi select (React Aria) with a summary trigger value,
 *  optionally rendering its options in labelled groups, per-group bulk actions
 *  and one-click presets. */
export function MultiSelect({
  options,
  sections,
  presets,
  selected,
  onChange,
  ariaLabel,
  placeholder = "Any",
  emptyLabel,
  allLabel,
}: UiMultiSelectProps) {
  const all = sections ? sections.flatMap((section) => section.options) : (options ?? []);
  const summary =
    selected.length === 0
      ? (emptyLabel ?? placeholder)
      : selected.length === 1
        ? (all.find((o) => o.id === selected[0])?.label ?? "1 selected")
        : selected.length === all.length && allLabel
          ? allLabel
          : `${selected.length} selected`;

  /** Toggle a whole section: clear it when every option is already selected. */
  const toggleSection = (section: UiOptionSection) => {
    const ids = new Set(section.options.map((o) => o.id));
    const everySelected = section.options.every((o) => selected.includes(o.id));
    onChange(
      everySelected ? selected.filter((id) => !ids.has(id)) : [...new Set([...selected, ...ids])],
    );
  };

  /** Exact-set comparison for the preset pressed state. */
  const sameSet = (a: string[], b: string[]) =>
    a.length === b.length && a.every((id) => b.includes(id));

  /** Selected items carry a check glyph, straight from the list item state. */
  const itemBody = (item: UiOption) => (
    <>
      <span className="ui-select__item-label">{item.label}</span>
      {selected.includes(item.id) && (
        <span className="ms-icon ui-select__item-check" aria-hidden="true">
          check
        </span>
      )}
    </>
  );

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
        {presets && presets.length > 0 && (
          <div className="ui-select__presets" role="group" aria-label={`${ariaLabel} presets`}>
            {presets.map((preset) => (
              <button
                key={preset.id}
                type="button"
                className="ui-select__preset"
                data-active={sameSet(selected, preset.ids) || undefined}
                onClick={() => onChange(preset.ids)}
              >
                {preset.label}
              </button>
            ))}
          </div>
        )}
        {sections ? (
          <ListBox className="ui-select__list" selectionMode="multiple">
            {sections.map((section) => {
              const everySelected = section.options.every((o) => selected.includes(o.id));
              return (
                <ListBoxSection key={section.id} id={section.id} aria-label={section.label}>
                  <Header className="ui-select__section-header">
                    <span>{section.label}</span>
                    <button
                      type="button"
                      className="ui-select__section-action"
                      aria-label={`${everySelected ? "Clear" : "Select all"} ${section.label}`}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => toggleSection(section)}
                    >
                      {everySelected ? "Clear" : "All"}
                    </button>
                  </Header>
                  {section.options.map((item) => (
                    <ListBoxItem
                      key={item.id}
                      className="ui-select__item"
                      id={item.id}
                      textValue={item.label}
                    >
                      {itemBody(item)}
                    </ListBoxItem>
                  ))}
                </ListBoxSection>
              );
            })}
          </ListBox>
        ) : (
          <ListBox className="ui-select__list" items={all} selectionMode="multiple">
            {(item) => (
              <ListBoxItem className="ui-select__item" id={item.id} textValue={item.label}>
                {itemBody(item)}
              </ListBoxItem>
            )}
          </ListBox>
        )}
      </Popover>
    </AriaSelect>
  );
}
