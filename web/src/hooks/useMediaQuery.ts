import { useEffect, useState } from "react";

// Subscribe to a CSS media query and re-render when it changes.
export function useMediaQuery(query: string): boolean {
  const get = () =>
    typeof window !== "undefined" && typeof window.matchMedia === "function"
      ? window.matchMedia(query).matches
      : false;

  const [matches, setMatches] = useState(get);

  useEffect(() => {
    const mql = window.matchMedia(query);
    const onChange = () => setMatches(mql.matches);
    onChange(); // sync in case the query changed between render and effect
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}

// The single breakpoint that decides desktop-native vs mobile-native layout.
// Below this we keep the phone shell; at/above we render the wide desktop app.
export const DESKTOP_QUERY = "(min-width: 1000px)";

export function useIsDesktop(): boolean {
  return useMediaQuery(DESKTOP_QUERY);
}
