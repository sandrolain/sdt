/**
 * The docs map's inline guide: an opt-in overlay of `(?)` hotspots, one per sidebar
 * section. Hovering a hotspot shows one line; pressing it opens the extended guide
 * in a modal — the corpus page named by `GUIDE_DOC_PATH`, read through the viewer's
 * own document endpoint and rendered by the shipped markdown pipeline, so the guide
 * is versioned with the project instead of rotting in the bundle (analysis
 * 20261013-063009, D4).
 *
 * Nothing auto-opens and nothing is persisted: the toggle is the Surface section's
 * `Guide` switch, and Escape or Close returns to the map unchanged.
 */
import { useEffect, useState } from "react";
import { Button, Dialog, DialogTrigger, Modal } from "react-aria-components";
import { fetchDoc } from "../lib/api";
import { renderMarkdown } from "../lib/markdown";
import { GUIDE_DOC_PATH, guideLineFor, type GuideAnchor } from "../lib/graphGuide";
import { Icon } from "../lib/icon";
import { TooltipButton } from "./ui/Tooltip";

/** The `(?)` hotspot for one section; renders nothing when the guide is off. */
export function GuideHotspot({
  anchor,
  enabled,
  onOpen,
}: {
  anchor: GuideAnchor;
  enabled: boolean;
  onOpen: () => void;
}) {
  const line = guideLineFor(anchor);
  if (!enabled || line === null) return null;
  return (
    <TooltipButton
      label={`Guide: ${anchor}`}
      tooltip={line}
      className="graph-tools__guide"
      onPress={onOpen}
    >
      <Icon name="help" />
    </TooltipButton>
  );
}

/** The guide body: the corpus page, fetched once per open. */
function GuideBody() {
  const [markdown, setMarkdown] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    fetchDoc(GUIDE_DOC_PATH)
      .then((doc) => {
        if (alive) setMarkdown("markdown" in doc ? doc.markdown : "");
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, []);

  if (error) return <p className="guide-modal__error">Could not read the guide: {error}</p>;
  if (markdown === null) return <p className="guide-modal__loading">Loading the guide…</p>;
  return (
    <div
      className="guide-modal__body"
      // The corpus markdown renderer is the viewer's own path for a trusted page.
      dangerouslySetInnerHTML={{ __html: renderMarkdown(markdown, { basePath: GUIDE_DOC_PATH }) }}
    />
  );
}

/** The extended guide in a modal, opened from a hotspot. */
export function GuideModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <DialogTrigger isOpen={open} onOpenChange={(next) => !next && onClose()}>
      <Button className="guide-modal__opener" aria-label="Docs map guide" />
      <Modal className="guide-modal__overlay">
        <Dialog className="guide-modal" aria-label="Docs map guide">
          {({ close }) => (
            <>
              <header className="guide-modal__header">
                <h2 className="guide-modal__title">Docs map guide</h2>
                <Button className="guide-modal__close" aria-label="Close the guide" onPress={close}>
                  <Icon name="close" />
                </Button>
              </header>
              <GuideBody />
            </>
          )}
        </Dialog>
      </Modal>
    </DialogTrigger>
  );
}
