import { createContext, useContext } from "react";
import type { OpenDocsState } from "./openDocs";

/** App-level open-documents API shared by the documents workspace and the wiki views. */
export interface OpenDocsApi {
  state: OpenDocsState;
  /** append (or activate) a document and navigate to it */
  open: (path: string) => void;
  /** focus an already-open document and navigate to it */
  activate: (path: string) => void;
  /** close a document tab, moving focus to a neighbour */
  close: (path: string) => void;
}

export const OpenDocsContext = createContext<OpenDocsApi | null>(null);

export function useOpenDocs(): OpenDocsApi {
  const api = useContext(OpenDocsContext);
  if (!api) throw new Error("useOpenDocs must be used inside OpenDocsProvider");
  return api;
}

/** Optional variant: components that only use the API when a provider is present. */
export function useOpenDocsOptional(): OpenDocsApi | null {
  return useContext(OpenDocsContext);
}
