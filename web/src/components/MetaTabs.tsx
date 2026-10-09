import { Icon } from "../lib/icon";
import { setMetaTab, useMetaTab, type MetaTab } from "../lib/metaTab";

const TABS: { id: MetaTab; icon: string; label: string }[] = [
  { id: "info", icon: "info", label: "Info" },
  { id: "sections", icon: "toc", label: "Sections" },
  { id: "links", icon: "link", label: "Links" },
  { id: "related", icon: "hub", label: "Related" },
];

/**
 * Tab selector for the document metadata panel. It lives in the dockview group
 * header (not the panel body); the selected tab drives `DocMetaPanel` through
 * the shared `metaTab` store, so the two stay in sync across the dockview tree.
 */
export function MetaTabs() {
  const tab = useMetaTab();
  return (
    <div
      className="meta-tabs__list meta-tabs__list--header"
      role="tablist"
      aria-label="Document panel"
    >
      {TABS.map((t) => (
        <button
          key={t.id}
          type="button"
          role="tab"
          aria-selected={tab === t.id}
          className={`meta-tabs__tab${tab === t.id ? " is-selected" : ""}`}
          onClick={() => setMetaTab(t.id)}
        >
          <Icon name={t.icon} />
          <span>{t.label}</span>
        </button>
      ))}
    </div>
  );
}
