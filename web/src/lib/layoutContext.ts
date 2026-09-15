import { createContext, useContext } from "react";

/** Panel visibility API shared by the dock layout and its panels. */
export interface LayoutApi {
  hidden: string[];
  hide: (id: string) => void;
  show: (id: string) => void;
}

export const LayoutContext = createContext<LayoutApi | null>(null);

/** Access the surrounding layout's panel visibility API, if any. */
export function useLayout(): LayoutApi | null {
  return useContext(LayoutContext);
}
