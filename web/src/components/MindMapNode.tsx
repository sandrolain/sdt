import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Icon } from "../lib/icon";
import type { MapFlowNode } from "../lib/mapModel";
import { isKnownSticker, stickerIcon } from "../lib/mapStickers";
import { sanitizeInline } from "../lib/markdown";

/**
 * One map topic: its label rendered through the shared DOMPurify allowlist, the
 * collapse toggle, its note and its stickers. A node whose whole label is a
 * single link is a button that opens the target; otherwise the node is a
 * focusable region and any link inside the label stays ordinary navigation.
 */
export function MapNodeView({ id, data, selected }: NodeProps<MapFlowNode>) {
  const { text, kind, href, hasChildren, collapsed, notes, link, stickers, onToggle, onOpen } =
    data;
  const label = (
    <span
      className="map-node__label"
      dangerouslySetInnerHTML={{ __html: sanitizeInline(data.content) }}
    />
  );
  return (
    <div
      className={`map-node map-node--${kind}${selected ? " is-selected" : ""}`}
      title={text}
      tabIndex={href ? undefined : 0}
      role={href ? undefined : "group"}
      aria-label={href ? undefined : text}
    >
      <Handle
        type="target"
        position={Position.Left}
        isConnectable={false}
        className="map-node__handle"
      />
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
      <Handle
        type="source"
        position={Position.Right}
        isConnectable={false}
        className="map-node__handle"
      />
    </div>
  );
}
