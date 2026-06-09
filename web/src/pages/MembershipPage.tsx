import { useI18n } from "@/i18n/I18nProvider";
import { useSession } from "@/hooks/useSession";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Icon } from "@/components/ui/Icon";
import type { Tier } from "@/types";

const TIERS: Array<{ key: Tier; fee: string }> = [
  { key: "GUEST", fee: "15%" },
  { key: "MEMBER", fee: "12%" },
  { key: "VIP", fee: "10%" },
];

export function MembershipPage() {
  const { t } = useI18n();
  const { tier } = useSession();

  const names: Record<Tier, string> = { GUEST: t("mem_guest"), MEMBER: t("mem_standard"), VIP: t("mem_vip") };
  const desc: Record<Tier, string> = { GUEST: t("mem_guest_a"), MEMBER: t("mem_std_a"), VIP: t("mem_vip_a") };

  return (
    <ScreenShell top={<TopBar kicker={t("mem_sub")} title={t("mem_title")} right={<LangPill />} />}>
      <div style={{ padding: "18px 16px 24px", display: "flex", flexDirection: "column", gap: 14 }}>
        {TIERS.map((tr) => {
          const on = tr.key === tier;
          const vip = tr.key === "VIP";
          return (
            <div key={tr.key} className="fade-up" style={{ position: "relative", borderRadius: "var(--r-3)", overflow: "hidden", border: "1px solid", borderColor: on ? "var(--gold)" : vip ? "var(--gold-line)" : "var(--line)", background: vip ? "linear-gradient(135deg,var(--burg),var(--bg-1) 70%)" : "var(--bg-1)", padding: "18px 18px" }}>
              {on && <div style={{ position: "absolute", top: 0, insetInlineEnd: 0 }}><span className="chip" data-st="live" style={{ borderRadius: "0 0 0 var(--r-2)" }}>{t("mem_current")}</span></div>}
              <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 12 }}>
                {vip && <span style={{ color: "var(--gold)" }}><Icon name="crown" size={22} /></span>}
                <div className="serif" style={{ fontSize: 23, color: vip ? "var(--gold-pale)" : "var(--fg)" }}>{names[tr.key]}</div>
              </div>
              <div style={{ display: "flex", gap: 20, marginBottom: 12 }}>
                <div>
                  <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{t("mem_fee")}</div>
                  <div className="mono" style={{ fontSize: 22, color: "var(--gold-pale)", marginTop: 3 }}>{tr.fee}</div>
                </div>
              </div>
              <p className="muted" style={{ fontSize: 13, lineHeight: 1.55, margin: 0 }}>{desc[tr.key]}</p>
              {!on && tr.key !== "GUEST" && (
                <button className={"btn " + (vip ? "btn-gold" : "btn-ghost")} style={{ width: "100%", marginTop: 14 }}>{t("mem_upgrade")}</button>
              )}
            </div>
          );
        })}
      </div>
    </ScreenShell>
  );
}
