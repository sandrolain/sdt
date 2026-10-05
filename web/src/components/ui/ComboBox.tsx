import {
  Button,
  ComboBox as AriaComboBox,
  Group,
  Input,
  ListBox,
  ListBoxItem,
  Popover,
  type ComboBoxProps,
} from "react-aria-components";

export interface UiComboOption {
  id: string;
  label: string;
}

interface UiComboBoxProps extends Omit<
  ComboBoxProps<UiComboOption>,
  "children" | "className" | "aria-label"
> {
  options: UiComboOption[];
  ariaLabel: string;
  /** visible label above the control; the aria label stays the accessible name */
  label?: string;
  placeholder?: string;
  className?: string;
}

/**
 * Searchable single select (React Aria): a text input filters the list with the
 * library's language-sensitive "contains" filter, keyboard navigation and the
 * a11y contract come for free (analysis D5/B9). Styled with Catppuccin tokens.
 */
export function ComboBox({
  options,
  ariaLabel,
  label,
  placeholder,
  className,
  ...props
}: UiComboBoxProps) {
  const control = (
    <AriaComboBox
      {...props}
      aria-label={ariaLabel}
      className={["ui-combo", className].filter(Boolean).join(" ")}
    >
      <Group className="ui-combo__group">
        <Input className="ui-combo__input" placeholder={placeholder} />
        <Button className="ui-combo__button">
          <span className="ms-icon ui-combo__chevron" aria-hidden="true">
            expand_more
          </span>
        </Button>
      </Group>
      <Popover className="ui-combo__popover" offset={4}>
        <ListBox className="ui-combo__list" items={options}>
          {(item) => (
            <ListBoxItem className="ui-combo__item" id={item.id} textValue={item.label}>
              {item.label}
            </ListBoxItem>
          )}
        </ListBox>
      </Popover>
    </AriaComboBox>
  );
  if (!label) return control;
  return (
    <div className="ui-field">
      <span className="ui-field__label">{label}</span>
      {control}
    </div>
  );
}
