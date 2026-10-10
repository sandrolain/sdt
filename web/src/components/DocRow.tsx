import { categoryColor, categoryIcon } from "../lib/categories";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import { kindIcon, kindLabel, type EntryFilterKind } from "../lib/kinds";

/** A status dot rendered at the row's trailing edge. */
export type DocRowDot = { tone: string; label: string };

/**
 * The shared row body rendered by both the tree rows and the right-panel
 * document links: kind glyph (or thumbnail), category icon, title + date, and
 * the status dot. One markup, so a document reads identically wherever it is
 * listed; each caller supplies its own anchor wrapper.
 */
export function DocRowBody({
  kind,
  image,
  imagePath,
  category,
  title,
  date,
  dot,
}: {
  kind?: EntryFilterKind;
  image?: string;
  imagePath?: string;
  category?: string;
  title: string;
  date?: string;
  dot?: DocRowDot | null;
}) {
  return (
    <>
      <span className="tree-entry__glyph">
        {image ? (
          <img className="tree-entry__thumb" src={imageUrl(image, imagePath ?? "")} alt="" />
        ) : (
          kind && <Icon name={kindIcon(kind)} label={kindLabel(kind)} title={kindLabel(kind)} />
        )}
      </span>
      <span className="tree-entry__text">
        <span className="tree-entry__title-row">
          <span className="tree-entry__title">{title}</span>
          <Icon
            name={categoryIcon(category ?? "")}
            className="tree-entry__category"
            style={{ color: categoryColor(category ?? "") }}
            title={category ?? "Uncategorised"}
          />
        </span>
        {date && <span className="tree-entry__date">{date}</span>}
      </span>
      {dot && (
        <span
          className={`tree-entry__dot tree-entry__dot--${dot.tone}`}
          title={dot.label}
          aria-label={dot.label}
          role="img"
        />
      )}
    </>
  );
}
