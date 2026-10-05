/**
 * External-link scheme guard for the shared JSON Canvas view.
 *
 * A `link` node renders its `url` into an `<a href>`; only `http`/`https` are
 * allowed so a crafted `.canvas` cannot execute a `javascript:`/`data:` URL.
 */

/** The URL when it is a safe external link, else null. */
export function safeExternalUrl(url: string | undefined): string | null {
  if (!url) return null;
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.href : null;
  } catch {
    return null;
  }
}

/** The hostname of a URL for a card label, or the raw value when unparsable. */
export function urlHost(url: string | undefined): string {
  if (!url) return "";
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}
