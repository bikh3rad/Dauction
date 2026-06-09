// Formatting helpers. Money is int64 USDC *cents* end-to-end; we only convert to
// a display string at the very edge. Bid credits are whole units ($1 each).

/** USDC cents → "$1,234.00 USDC" */
export function usdc(cents: number): string {
  const n = cents / 100;
  return (
    "$" +
    n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) +
    " USDC"
  );
}

/** USDC cents → "$1,234 USDC" (whole-dollar display, rounded) */
export function usdc0(cents: number): string {
  return "$" + Math.round(cents / 100).toLocaleString("en-US") + " USDC";
}

/** USDC cents → "$1,234" (bare, no suffix) */
export function dollars(cents: number): string {
  return "$" + Math.round(cents / 100).toLocaleString("en-US");
}

/** whole credits → "120" */
export function credits(n: number): string {
  return n.toLocaleString("en-US");
}

/** seconds → "1:02:03" or "02:03" */
export function fmtClock(s: number): string {
  s = Math.max(0, Math.floor(s));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = s % 60;
  const p = (x: number) => String(x).padStart(2, "0");
  return h > 0 ? `${h}:${p(m)}:${p(ss)}` : `${p(m)}:${p(ss)}`;
}

/** seconds until → {d,h,m,s,left,done} */
export function breakdown(secondsLeft: number) {
  const left = Math.max(0, Math.floor(secondsLeft));
  return {
    d: Math.floor(left / 86400),
    h: Math.floor((left % 86400) / 3600),
    m: Math.floor((left % 3600) / 60),
    s: left % 60,
    left,
    done: left <= 0,
  };
}

/** ISO timestamp → seconds remaining (clamped at 0). */
export function secondsUntil(iso?: string): number {
  if (!iso) return 0;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return 0;
  return Math.max(0, Math.floor((t - Date.now()) / 1000));
}

/** A short reference like "DXB·26·W23·007" derived deterministically from an id. */
export function shortRef(isoWeek: string, seq: number): string {
  const week = isoWeek?.replace("-", "·") ?? "26·W00";
  return `DXB·${week}·${String(seq).padStart(3, "0")}`;
}
