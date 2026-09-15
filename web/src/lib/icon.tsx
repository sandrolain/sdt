import type { CSSProperties } from "react";

interface IconProps {
  name: string;
  /** Label for the icon; omit for a purely decorative glyph */
  label?: string;
  weight?: number;
  fill?: boolean;
  className?: string;
  style?: CSSProperties;
}

/** Material Symbols glyph with a fixed variation axis policy (see `--ms-*` CSS). */
export function Icon({ name, label, weight = 600, fill = true, className, style }: IconProps) {
  const role = label
    ? { role: "img", "aria-label": label }
    : { role: "presentation", "aria-hidden": "true" as const };
  return (
    <span
      {...role}
      className={`ms-icon${className ? ` ${className}` : ""}`}
      style={{
        fontVariationSettings: `"FILL" ${fill ? 1 : 0}, "GRAD" 0, "opsz" 24, "wght" ${weight}`,
        ...style,
      }}
    >
      {name}
    </span>
  );
}
