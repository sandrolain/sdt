import { useEffect, useState } from "react";
import { LIVE_RELOAD_EVENT } from "./liveUpdates";

/** Counter that increments on each live corpus change, to retrigger fetches. */
export function useReloadToken(): number {
  const [token, setToken] = useState(0);
  useEffect(() => {
    const onReload = () => setToken((value) => value + 1);
    window.addEventListener(LIVE_RELOAD_EVENT, onReload);
    return () => window.removeEventListener(LIVE_RELOAD_EVENT, onReload);
  }, []);
  return token;
}
