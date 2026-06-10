import type { CSSProperties, ReactNode } from "react";

// Shared House-Ops primitives — ported from the HTML prototype's admin.jsx so
// the React console matches the visual spec (burgundy+gold tables, stat tiles).

export function SecHead({ kicker, title, action }: { kicker?: string; title: string; action?: ReactNode }) {
  return (
    <div style={{ marginBottom: 22, display: "flex", alignItems: "flex-end", gap: 16 }}>
      <div style={{ flex: 1 }}>
        {kicker && <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginBottom: 6 }}>{kicker}</div>}
        <h1 className="serif" style={{ fontSize: 30, margin: 0, color: "var(--gold-pale)" }}>{title}</h1>
      </div>
      {action}
    </div>
  );
}

export function Tile({ label, value, sub, accent }: { label: string; value: ReactNode; sub?: string; accent?: boolean }) {
  return (
    <div style={{ flex: 1, minWidth: 170, padding: "18px 20px", border: "1px solid var(--line)", borderRadius: "var(--r-2)", background: accent ? "linear-gradient(135deg,var(--burg-deep),var(--bg-1))" : "var(--bg-1)" }}>
      <div className="mono up" style={{ fontSize: 9.5, color: "var(--fg-faint)", marginBottom: 10 }}>{label}</div>
      <div className="serif" style={{ fontSize: 30, color: "var(--gold-pale)", lineHeight: 1 }}>{value}</div>
      {sub && <div className="mono" style={{ fontSize: 11, color: "var(--fg-muted)", marginTop: 8 }}>{sub}</div>}
    </div>
  );
}

export interface Col { label: string; end?: boolean }
export function DTable({ cols, children }: { cols: Col[]; children: ReactNode }) {
  return (
    <div style={{ border: "1px solid var(--line)", borderRadius: "var(--r-2)", overflow: "hidden", background: "var(--bg-1)" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
        <thead><tr style={{ background: "var(--bg-0)" }}>
          {cols.map((c, i) => (
            <th key={i} style={{ textAlign: c.end ? "end" : "start", padding: "12px 16px", fontFamily: "var(--mono)", fontSize: 9.5, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--fg-faint)", fontWeight: 600, borderBottom: "1px solid var(--line)" }}>{c.label}</th>
          ))}
        </tr></thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

export const tdS: CSSProperties = { padding: "13px 16px", borderBottom: "1px solid var(--line)", verticalAlign: "middle" };
export const tdEnd: CSSProperties = { ...tdS, textAlign: "end" };
export const tdMuted: CSSProperties = { ...tdS, color: "var(--fg-muted)" };

type GBtnKind = "ghost" | "gold" | "bad";
export function GBtn({ children, onClick, kind = "ghost", small, disabled }: { children: ReactNode; onClick?: () => void; kind?: GBtnKind; small?: boolean; disabled?: boolean }) {
  const bg = kind === "gold" ? "var(--gold)" : "transparent";
  const col = kind === "gold" ? "#1B1207" : kind === "bad" ? "var(--st-bad)" : "var(--fg)";
  const bd = kind === "gold" ? "var(--gold-bright)" : kind === "bad" ? "var(--st-bad)" : "var(--line-strong)";
  return (
    <button onClick={onClick} disabled={disabled} className="mono" style={{ fontSize: 11, fontWeight: 600, padding: small ? "6px 11px" : "8px 14px", borderRadius: "var(--r-1)", border: "1px solid " + bd, background: bg, color: col, cursor: disabled ? "default" : "pointer", opacity: disabled ? 0.4 : 1, letterSpacing: "0.04em" }}>{children}</button>
  );
}

// A row of contextual actions, right-aligned.
export function Actions({ children }: { children: ReactNode }) {
  return <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>{children}</div>;
}
export const Dash = () => <span className="mono" style={{ fontSize: 11, color: "var(--fg-faint)" }}>—</span>;
