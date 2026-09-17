import { useEffect, type RefObject } from "react";
import { clearActiveSection, setActiveSection } from "./activeSection";

/**
 * Publish the heading currently in view inside `container` for `path` while
 * `active` is true. Observing against the viewport is enough: the browser
 * already accounts for scroll-container clipping, and inactive dock panels are
 * hidden so they never fire.
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
    if (typeof IntersectionObserver === "undefined") return;
    const root = container.current;
    const headings = root
      ? Array.from(root.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6"))
      : [];
    if (headings.length === 0) return;

    const visible = new Map<Element, boolean>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) visible.set(entry.target, entry.isIntersecting);
        const first = headings.find((h) => visible.get(h));
        setActiveSection(path, first?.textContent?.trim() ?? null);
      },
      { rootMargin: "0px 0px -65% 0px", threshold: 0 },
    );
    for (const heading of headings) observer.observe(heading);
    return () => {
      observer.disconnect();
      clearActiveSection(path);
    };
  }, [container, path, active]);
}
