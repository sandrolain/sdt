import { Icon } from "../lib/icon";
import type { MapNodeData } from "../lib/mapModel";
import { isKnownSticker, stickerIcon } from "../lib/mapStickers";
import { sanitizeInline } from "../lib/markdown";

/**
 * One map topic body, mounted by the shared `JsonCanvas` view through its
 * `renderText` seam. The label renders through the shared DOMPurify allowlist;
 * the collapse toggle, the note and the stickers sit beside it. A node whose
 * whole label is a single link is a button that opens the target; otherwise any
 * link inside the label stays ordinary navigation. The box (border, radius,
 * padding) belongs to the view's `.jc-node`, so this is the content only.
 */
export function MapNodeBody({ id, data }: { id: string; data: MapNodeData }) {
  const {
    text,
    kind,
    href,
    hasChildren,
    collapsed,
    notes,
    link,
    stickers,
    onToggle,
    onOpen,
    depth,
  } = data;
  const label = (
    <span
      className="map-node__label"
      dangerouslySetInnerHTML={{ __html: sanitizeInline(data.content) }}
    />
  );
  return (
    <div className={`map-node map-node--${kind}${depth === 0 ? " map-node--root" : ""}`}>
      {href && onOpen ? (
        <button
          type="button"
          className="map-node__button"
          aria-label={`Open ${text}`}
          onClick={() => onOpen(href)}
        >
          {label}
        </button>
      ) : (
        <div className="map-node__text">{label}</div>
      )}
      {link && !href && (
        <a
          className="map-node__link"
          href={link}
          target="_blank"
          rel="noreferrer"
          aria-label={`External link: ${text}`}
        >
          <Icon name="open_in_new" />
        </a>
      )}
      {stickers.length > 0 && (
        <span className="map-node__stickers">
          {stickers.map((marker) => (
            <span
              key={marker}
              className={`map-node__sticker${isKnownSticker(marker) ? "" : " is-unknown"}`}
              aria-label={`Marker ${marker}`}
              data-sticker={marker}
            >
              <Icon name={stickerIcon(marker)} />
            </span>
          ))}
        </span>
      )}
      {notes.length > 0 && (
        <span
          className="map-node__note"
          aria-label={`Note: ${notes.join("; ")}`}
          title={notes.join("\n")}
        >
          <Icon name="sticky_note_2" />
        </span>
      )}
      {hasChildren && onToggle && (
        <button
          type="button"
          className="map-node__toggle"
          aria-expanded={!collapsed}
          aria-label={collapsed ? `Expand ${text}` : `Collapse ${text}`}
          onClick={() => onToggle(id)}
        >
          <Icon name={collapsed ? "add" : "remove"} />
        </button>
      )}
    </div>
  );
}
