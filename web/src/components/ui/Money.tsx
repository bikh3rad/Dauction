import { usdc, usdc0 } from "@/lib/format";

// Renders an int64 USDC *cents* amount as a styled monospace string.
// `cents` prop name documents the unit; pass cents always.
export interface MoneyProps {
  cents: number;
  big?: boolean;
  gold?: boolean;
  withCents?: boolean;
}

export function Money({ cents, big, gold, withCents = true }: MoneyProps) {
  const s = withCents ? usdc(cents) : usdc0(cents);
  return (
    <span
      className="mono tnum"
      style={{
        fontWeight: big ? 600 : 500,
        color: gold ? "var(--gold-pale)" : "inherit",
        letterSpacing: big ? "-0.01em" : 0,
      }}
    >
      {s}
    </span>
  );
}
