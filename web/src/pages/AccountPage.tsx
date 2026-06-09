import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { LABELS, LANGS } from "@/i18n/locales";
import { useSession } from "@/hooks/useSession";
import { useWallet } from "@/hooks/queries";
import { useVault } from "@/hooks/queries";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Icon } from "@/components/ui/Icon";
import { Chip } from "@/components/ui/Chip";
import { usdc } from "@/lib/format";

export function AccountPage() {
  const { t, lang, setLang } = useI18n();
  const nav = useNavigate();
  const { account, tier, isGuest } = useSession();
  const { data: wallet } = useWallet();
  const { data: vault } = useVault();

  const tierTag = tier === "VIP" ? t("mem_vip") : tier === "MEMBER" ? t("member_tag") : t("guest_tag");

  return (
    <ScreenShell top={<TopBar kicker={account?.id.slice(0, 10) ?? "@guest"} title={t("nav_account")} right={<LangPill />} />}>
      <div style={{ padding: "18px 16px 24px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 14, marginBottom: 20 }}>
          <div style={{ width: 56, height: 56, borderRadius: "50%", border: "1px solid var(--gold-line)", background: "var(--burg)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--gold-pale)" }}>
            <Icon name="user" size={26} />
          </div>
          <div>
            <div className="serif" style={{ fontSize: 19, color: "var(--gold-pale)" }}>{account?.id.slice(0, 12) ?? "Guest"}</div>
            <div style={{ marginTop: 4 }}><Chip state={isGuest ? "neut" : "active"} label={tierTag.toUpperCase()} /></div>
          </div>
        </div>

        <div className="kv">
          <div className="k">{t("clo_credit")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{vault ? usdc(vault.creditBalanceCents) : "—"}</div>
          <div className="k">{t("bid_wallet")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{wallet?.balanceCredits ?? "—"} {t("credits")}</div>
          <div className="k">KYC</div><div className="v">{account?.kycStatus ?? "—"}</div>
          <div className="k">{t("mem_current")}</div><div className="v">{tier}</div>
        </div>

        <button className="btn btn-ghost" style={{ width: "100%", marginTop: 16 }} onClick={() => nav("/bidstore")}><Icon name="coins" size={16} /> {t("buy_bids")}</button>

        {isGuest && (
          <button className="btn btn-burg" style={{ width: "100%", marginTop: 12 }} onClick={() => nav("/invite")}>
            <Icon name="crown" size={16} /> {t("enter_invite")}
          </button>
        )}

        <div style={{ display: "flex", gap: 8, marginTop: 20 }}>
          {LANGS.map((l) => (
            <button key={l} onClick={() => setLang(l)} className="mono" style={{ flex: 1, padding: "11px", borderRadius: "var(--r-1)", border: "1px solid", borderColor: lang === l ? "var(--gold)" : "var(--line-strong)", background: lang === l ? "var(--gold)" : "transparent", color: lang === l ? "#1B1207" : "var(--fg-muted)", cursor: "pointer", fontWeight: 600 }}>
              {LABELS[l]}
            </button>
          ))}
        </div>
      </div>
    </ScreenShell>
  );
}
