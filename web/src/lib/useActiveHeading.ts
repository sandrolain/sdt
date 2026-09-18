import { useEffect, type RefObject } from "react";
import { clearActiveSection, setActiveSection } from "./activeSection";

/** Fraction of the scroll viewport below which a heading becomes the active one. */
const THRESHOLD = 0.10;

/**
 * Publish the section currently in view for `path` while `active` is true.
 * Uses scroll position (not intersection): the active section is the last
 * heading whose top has passed a threshold near the top of the scroll area, so
 * a section is always selected — including when scrolled mid-section.
 */
export function useActiveHeading(
  container: RefObject<HTMLElement | null>,
  path: string,
  active: boolean,
): void {
  useEffect(() => {
    if (!active) {
      clearActiveSection(path);
      return;
    }
    const root = container.current;
    if (!root || !root.isConnected) return;
    const headings = Array.from(root.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6"));
    if (headings.length === 0) return;

    const compute = () => {
      const limit = root.getBoundingClientRect().top + root.clientHeight * THRESHOLD;
      let current = headings[0];
      let dist = Infinity;
      for (const heading of headings) {
        const actDist = Math.abs(heading.getBoundingClientRect().top - limit);
        if (actDist < dist) {
          current = heading;
          dist = actDist;
        }
      }
      setActiveSection(path, (current.dataset.heading ?? current.textContent ?? "").trim() || null);
    };

    compute();
    // capture-phase catches scrolling from the container or any ancestor
    window.addEventListener("scroll", compute, { capture: true, passive: true });
    window.addEventListener("resize", compute);
    return () => {
      window.removeEventListener("scroll", compute, { capture: true });
      window.removeEventListener("resize", compute);
      clearActiveSection(path);
    };
  }, [container, path, active]);
}
