import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { loadPreview, previewPathFromHref, type PreviewMeta } from "../lib/preview";
import { formatFieldDate } from "../lib/frontmatter";

interface HoverPreviewProps {
  /** debounce before fetching, in ms */
  delay?: number;
  /** how long the card stays after the pointer leaves the link, in ms */
  leaveDelay?: number;
}

interface PreviewState {
  meta: PreviewMeta;
  left: number;
  top: number;
}

const WIDTH = 340;
const MARGIN = 8;

/** Hover preview card with the target document's metadata. */
export function HoverPreview({ delay = 150, leaveDelay = 160 }: HoverPreviewProps) {
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const cardRef = useRef<HTMLDivElement | null>(null);
  const hideTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const cancelHide = useCallback(() => {
    if (hideTimer.current) clearTimeout(hideTimer.current);
    hideTimer.current = undefined;
  }, []);

  const scheduleHide = useCallback(() => {
    cancelHide();
    hideTimer.current = setTimeout(() => setPreview(null), leaveDelay);
  }, [cancelHide, leaveDelay]);

  useEffect(() => {
    let controller: AbortController | null = null;
    let hoverTimer: ReturnType<typeof setTimeout> | undefined;

    const onOver = (event: Event) => {
      const anchor = (event.target as HTMLElement | null)?.closest?.(".doc-rendered a");
      if (!anchor) return;
      const path = previewPathFromHref(anchor.getAttribute("href") ?? "");
      if (!path) return;
      cancelHide();
      const rect = anchor.getBoundingClientRect();
      const left = Math.max(MARGIN, Math.min(rect.left, window.innerWidth - WIDTH - MARGIN));
      const top = rect.bottom + 6;
      if (hoverTimer !== undefined) clearTimeout(hoverTimer);
      controller?.abort();
      controller = new AbortController();
      const signal = controller.signal;
      hoverTimer = setTimeout(() => {
        loadPreview(path, signal)
          .then((meta) => setPreview({ meta, left, top }))
          .catch(() => undefined);
      }, delay);
    };

    const onOut = (event: Event) => {
      const anchor = (event.target as HTMLElement | null)?.closest?.(".doc-rendered a");
      if (!anchor) return;
      if (hoverTimer !== undefined) clearTimeout(hoverTimer);
      controller?.abort();
      scheduleHide();
    };

    document.addEventListener("mouseover", onOver);
    document.addEventListener("mouseout", onOut);
    return () => {
      document.removeEventListener("mouseover", onOver);
      document.removeEventListener("mouseout", onOut);
      if (hoverTimer !== undefined) clearTimeout(hoverTimer);
      controller?.abort();
      cancelHide();
    };
  }, [delay, cancelHide, scheduleHide]);

  // clamp vertically once the card height is known (flip above the link if needed)
  useLayoutEffect(() => {
    const card = cardRef.current;
    if (!card || !preview) return;
    const height = card.offsetHeight;
    const maxTop = window.innerHeight - height - MARGIN;
    if (preview.top > maxTop) {
      const flipped = Math.max(MARGIN, preview.top - height - 18);
      if (flipped !== preview.top) setPreview({ ...preview, top: flipped });
    }
  }, [preview]);

  if (!preview) return null;
  const { meta } = preview;
  return createPortal(
    <div
      ref={cardRef}
      className="hover-preview"
      role="tooltip"
      style={{ left: preview.left, top: preview.top, width: WIDTH }}
      onMouseEnter={cancelHide}
      onMouseLeave={scheduleHide}
    >
      <div className="hover-preview__title">{meta.title}</div>
      {meta.summary && <p className="hover-preview__summary">{meta.summary}</p>}
      <dl className="hover-preview__dates">
        {meta.created && (
          <div>
            <dt>Created</dt>
            <dd>{formatFieldDate(meta.created)}</dd>
          </div>
        )}
        {meta.modified && (
          <div>
            <dt>Modified</dt>
            <dd>{formatFieldDate(meta.modified)}</dd>
          </div>
        )}
      </dl>
      <div className="hover-preview__path">{meta.path}</div>
    </div>,
    document.body,
  );
}
