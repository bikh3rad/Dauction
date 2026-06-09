import { useEffect, useRef } from "react";

// setInterval that survives re-renders and respects an `on` toggle.
export function useInterval(cb: () => void, ms: number, on = true) {
  const ref = useRef(cb);
  ref.current = cb;
  useEffect(() => {
    if (!on) return;
    const id = setInterval(() => ref.current(), ms);
    return () => clearInterval(id);
  }, [ms, on]);
}
