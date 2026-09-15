import { createContext, useContext } from "react";
import type { OpenDocsAction, OpenDocsState } from "../lib/openDocs";

/** Open-documents stack API, provided by `OpenDocsProvider`. */
export interface OpenDocsApi {
  state: OpenDocsState;
  dispatch: (action: OpenDocsAction) => void;
}

export const OpenDocsContext = createContext<OpenDocsApi | null>(null);

export function useOpenDocs(): OpenDocsApi {
  const api = useContext(OpenDocsContext);
  if (!api) throw new Error("useOpenDocs must be used inside OpenDocsProvider");
  return api;
}
