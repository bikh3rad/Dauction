import { useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "./Icon";

// A collapsible reference shown on auction pages: how the three auction types
// work, and every status an item passes through while it's in an auction.
const TYPES = [
  { name: "mode_dutch", rule: "dutch_rule", icon: "flame" },
  { name: "mode_vickrey", rule: "vickrey_rule", icon: "file" },
  { name: "mode_uniqbid", rule: "uniqbid_rule", icon: "hash" },
] as const;

const STATUSES = [
  { st: "proposed", name: "as_upcoming", desc: "as_upcoming_d" },
  { st: "live", name: "as_live", desc: "as_live_d" },
  { st: "warn", name: "as_closing", desc: "as_closing_d" },
  { st: "warn", name: "as_hammer", desc: "as_hammer_d" },
  { st: "good", name: "as_resolved", desc: "as_resolved_d" },
  { st: "warn", name: "as_settling", desc: "as_settling_d" },
  { st: "good", name: "as_sold", desc: "as_sold_d" },
  { st: "bad", name: "as_aborted", desc: "as_aborted_d" },
] as const;

export function AuctionStatusGuide({ defaultOpen = false }: { defaultOpen?: boolean }) {
  const { t, dir } = useI18n();
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div style={{ border: "1px solid var(--line)", borderRadius: "var(--r-2)", background: "var(--bg-1)", overflow: "hidden" }}>
      <button onClick={() => setOpen((o) => !o)} style={{ width: "100%", display: "flex", alignItems: "center", gap: 12, padding: "14px 16px", background: "none", border: "none", cursor: "pointer", color: "var(--fg)", textAlign: "start" }}>
        <span style={{ color: "var(--gold)" }}><Icon name="shield" size={18} /></span>
        <span style={{ flex: 1, fontWeight: 600, fontSize: 14 }}>{t("as_guide_title")}</span>
        <Icon name={open ? "chevron-down" : (dir === "rtl" ? "chevron-left" : "chevron-right")} size={18} style={{ color: "var(--gold)" }} />
      </button>

      {open && (
        <div className="fade-up" style={{ padding: "0 16px 16px" }}>
          {/* auction types */}
          <div className="mono up" style={{ fontSize: 9.5, color: "var(--gold)", letterSpacing: "0.12em", margin: "6px 0 10px" }}>{t("as_types_title")}</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 10, marginBottom: 18 }}>
            {TYPES.map((ty) => (
              <div key={ty.name} style={{ display: "flex", gap: 11 }}>
                <span style={{ color: "var(--gold)", flexShrink: 0, marginTop: 1 }}><Icon name={ty.icon} size={16} /></span>
                <div>
                  <div style={{ fontWeight: 600, fontSize: 13 }}>{t(ty.name)}</div>
                  <p className="muted" style={{ fontSize: 12, lineHeight: 1.55, margin: "2px 0 0" }}>{t(ty.rule)}</p>
                </div>
              </div>
            ))}
          </div>

          {/* statuses */}
          <div className="mono up" style={{ fontSize: 9.5, color: "var(--gold)", letterSpacing: "0.12em", marginBottom: 10 }}>{t("as_status_title")}</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {STATUSES.map((s) => (
              <div key={s.name} style={{ display: "flex", alignItems: "flex-start", gap: 11 }}>
                <span className="chip" data-st={s.st} style={{ flexShrink: 0, minWidth: 78, justifyContent: "center" }}>{t(s.name)}</span>
                <p className="muted" style={{ fontSize: 12, lineHeight: 1.5, margin: 0 }}>{t(s.desc)}</p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
