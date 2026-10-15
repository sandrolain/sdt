/**
 * KindFilter — the Gallery's kind selection as a multi-select dropdown.
 *
 * A chip row could only express one kind at a time; the corpus is read by asking for
 * two or three at once ("the decisions and the analyses"). The trigger reports the
 * selection, the popover holds one checkable item per kind with the same icons the
 * chips carried, and `All` clears the set — which by `lib/kindFilter.ts` means every
 * kind, never none.
 */
import { Button, Dialog, DialogTrigger, Popover } from "react-aria-components";
import { Icon } from "../lib/icon";
import { kindColor, kindIcon, kindLabel, KIND_ORDER } from "../lib/kinds";
import { clearKinds, kindSelectionLabel, toggleKind, type KindSelection } from "../lib/kindFilter";

export function KindFilter({
  selection,
  onChange,
}: {
  selection: KindSelection;
  onChange: (next: Set<string>) => void;
}) {
  return (
    <DialogTrigger>
      <Button className="kind-filter__trigger" aria-label="Filter by kind">
        <Icon name="filter_list" />
        {kindSelectionLabel(selection)}
        <span className="ms-icon kind-filter__caret" aria-hidden="true">
          expand_more
        </span>
      </Button>
      <Popover className="kind-filter__popover" placement="bottom start">
        <Dialog className="kind-filter" aria-label="Filter by kind">
          <div className="kind-filter__head">
            <span className="kind-filter__title">Kinds</span>
            <Button className="kind-filter__all" onPress={() => onChange(clearKinds())}>
              All
            </Button>
          </div>
          <ul className="kind-filter__list" role="list">
            {KIND_ORDER.map((kind) => (
              <li key={kind}>
                <label className="kind-filter__item">
                  <input
                    type="checkbox"
                    aria-label={kindLabel(kind)}
                    checked={selection.has(kind)}
                    onChange={() => onChange(toggleKind(selection, kind))}
                  />
                  <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} />
                  <span>{kindLabel(kind)}</span>
                </label>
              </li>
            ))}
          </ul>
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
