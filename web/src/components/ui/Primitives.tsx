import { Fragment, useRef } from "react";
import type { CSSProperties, ReactNode } from "react";

// Small shared building blocks ported from the prototype.

export function Label({ children }: { children: ReactNode }) {
  return (
    <div className="mono up" style={{ fontSize: 10, color: "var(--fg-muted)", marginBottom: 8 }}>
      {children}
    </div>
  );
}

export function Stat({ label, value, gold }: { label: ReactNode; value: ReactNode; gold?: boolean }) {
  return (
    <div style={{ flex: 1, border: "1px solid var(--line)", borderRadius: "var(--r-2)", padding: "12px 14px", background: "var(--bg-1)" }}>
      <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)", marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 15, color: gold ? "var(--gold-pale)" : "var(--fg)" }}>{value}</div>
    </div>
  );
}

export function MiniStat({ label, value, gold }: { label: ReactNode; value: ReactNode; gold?: boolean }) {
  return (
    <div style={{ textAlign: "center" }}>
      <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)", marginBottom: 4 }}>{label}</div>
      <div className="mono" style={{ fontSize: 16, color: gold ? "var(--gold-pale)" : "var(--fg)" }}>{value}</div>
    </div>
  );
}

export function Stepper({ steps, active }: { steps: string[]; active: number }) {
  return (
    <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
      {steps.map((s, i) => (
        <Fragment key={i}>
          <div
            className="mono"
            style={{
              width: 26, height: 26, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center",
              fontSize: 12, fontWeight: 600, border: "1px solid",
              borderColor: i <= active ? "var(--gold)" : "var(--line-strong)",
              background: i <= active ? "var(--gold)" : "transparent",
              color: i <= active ? "#1B1207" : "var(--fg-faint)",
            }}
          >
            {i < active ? "✓" : s}
          </div>
          {i < steps.length - 1 && <div style={{ flex: 1, height: 1, background: i < active ? "var(--gold)" : "var(--line)" }} />}
        </Fragment>
      ))}
    </div>
  );
}

export function OtpInput({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const ref = useRef<HTMLInputElement>(null);
  return (
    <div style={{ position: "relative" }} onClick={() => ref.current?.focus()}>
      <div style={{ display: "flex", gap: 8 }}>
        {[0, 1, 2, 3].map((i) => (
          <div
            key={i}
            style={{
              flex: 1, height: 54, borderRadius: "var(--r-1)", border: "1px solid",
              borderColor: value.length === i || (value.length === 4 && i === 3) ? "var(--gold)" : "var(--line-strong)",
              background: "var(--bg-0)", display: "flex", alignItems: "center", justifyContent: "center",
              fontFamily: "var(--mono)", fontSize: 22, color: "var(--fg)",
            }}
          >
            {value[i] || ""}
          </div>
        ))}
      </div>
      <input
        ref={ref}
        value={value}
        dir="ltr"
        inputMode="numeric"
        autoFocus
        aria-label="enter code"
        onChange={(e) => onChange(e.target.value.replace(/\D/g, "").slice(0, 4))}
        style={{
          position: "absolute", inset: 0, width: "100%", height: "100%", opacity: 0, border: "none",
          background: "transparent", color: "transparent", caretColor: "transparent", cursor: "pointer",
          fontSize: 16, textAlign: "center",
        }}
      />
    </div>
  );
}

export const iconBtnStyle: CSSProperties = {
  width: 38, height: 38, borderRadius: "var(--r-1)", border: "1px solid var(--line)",
  background: "var(--bg-1)", color: "var(--fg)", display: "flex", alignItems: "center",
  justifyContent: "center", cursor: "pointer",
};
