import { Dialog, DialogTrigger, Popover, ToggleButton } from "react-aria-components";
import {
  FONT_SCALES,
  MEASURE_CH,
  setFontScale,
  setMeasureOn,
  useReadingPrefs,
  type FontScale,
} from "../lib/readingPrefs";
import { Icon } from "../lib/icon";
import { Switch } from "./ui/Switch";

const STEP_LABELS: Record<number, string> = {
  [FONT_SCALES[0]]: "Small",
  [FONT_SCALES[1]]: "Default",
  [FONT_SCALES[2]]: "Large",
  [FONT_SCALES[3]]: "Largest",
};

/**
 * Global reading settings (adoption decision Q2): text size and the line-length
 * measure. One preference for every document, applied as CSS custom properties
 * on the root — no per-tab control, no reload.
 */
export function ReadingSettings() {
  const { fontScale, measureOn } = useReadingPrefs();
  const step = FONT_SCALES.indexOf(fontScale as FontScale);
  return (
    <DialogTrigger>
      <ToggleButton className="top-bar__icon-button" aria-label="Reading settings">
        <Icon name="format_size" />
      </ToggleButton>
      <Popover className="reading-settings" offset={6}>
        <Dialog className="reading-settings__dialog" aria-label="Reading settings">
          <div className="reading-settings__row">
            <span className="reading-settings__label" id="reading-size-label">
              Text size
            </span>
            <div
              className="reading-settings__sizes"
              role="group"
              aria-labelledby="reading-size-label"
            >
              {FONT_SCALES.map((scale, index) => (
                <ToggleButton
                  key={scale}
                  className="reading-settings__size"
                  aria-label={`Text size: ${STEP_LABELS[scale]}`}
                  aria-pressed={index === step}
                  isSelected={index === step}
                  onChange={() => setFontScale(scale)}
                >
                  <span aria-hidden="true" style={{ fontSize: `${0.75 + index * 0.15}rem` }}>
                    A
                  </span>
                </ToggleButton>
              ))}
            </div>
          </div>
          <div className="reading-settings__row">
            <Switch
              className="reading-settings__measure"
              isSelected={measureOn}
              onChange={setMeasureOn}
            >
              Limit line length ({MEASURE_CH}ch)
            </Switch>
          </div>
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
