import { useEffect, useState } from "react";
import { loadPreview, previewPathFromHref } from "../lib/preview";

interface HoverPreviewProps {
  /** debounce before fetching, in ms */
  delay?: number;
}

interface PreviewState {
  html: string;
  left: number;
  top: number;
}

const WIDTH = 380;

/** Hover preview popover for resolved in-document links. */
export function HoverPreview({ delay = 150 }: HoverPreviewProps) {
  const [preview, setPreview] = useState<PreviewState | null>(null);

  useEffect(() => {
    let controller: AbortController | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const hide = () => {
      if (timer !== undefined) clearTimeout(timer);
      controller?.abort();
      controller = null;
      setPreview(null);
    };

    const onOver = (event: Event) => {
      const anchor = (event.target as HTMLElement | null)?.closest?.(".doc-rendered a");
      if (!anchor) return;
      const path = previewPathFromHref(anchor.getAttribute("href") ?? "");
      if (!path) return;
      const rect = anchor.getBoundingClientRect();
      const left = Math.max(8, Math.min(rect.left, window.innerWidth - WIDTH - 8));
      const top = rect.bottom + 6;
      if (timer !== undefined) clearTimeout(timer);
      controller?.abort();
      controller = new AbortController();
      const signal = controller.signal;
      timer = setTimeout(() => {
        loadPreview(path, signal)
          .then((html) => setPreview({ html, left, top }))
          .catch(() => undefined);
      }, delay);
    };

    const onOut = (event: Event) => {
      if ((event.target as HTMLElement | null)?.closest?.(".doc-rendered a")) hide();
    };

    document.addEventListener("mouseover", onOver);
    document.addEventListener("mouseout", onOut);
    return () => {
      document.removeEventListener("mouseover", onOver);
      document.removeEventListener("mouseout", onOut);
      hide();
    };
  }, [delay]);

  if (!preview) return null;
  return (
    <div
      className="hover-preview"
      role="tooltip"
      style={{ left: preview.left, top: preview.top, width: WIDTH }}
      dangerouslySetInnerHTML={{ __html: preview.html }}
    />
  );
}
