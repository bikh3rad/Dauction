import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { LABELS, LANGS } from "@/i18n/locales";
import { useSession } from "@/hooks/useSession";
import { useWallet, useVault, useSignOut } from "@/hooks/queries";
import { levelInfo, MAX_LEVEL } from "@/lib/membership";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Icon } from "@/components/ui/Icon";
import { Chip } from "@/components/ui/Chip";
import { usdc } from "@/lib/format";

export function AccountPage() {
  const { t, lang, setLang } = useI18n();
  const nav = useNavigate();
  const { account, level, isGuest } = useSession();
  const { data: wallet } = useWallet();
  const { data: vault } = useVault();
  const signOut = useSignOut();

  const lv = levelInfo(level);
  const isInspector = (account?.roles ?? []).includes("INSPECTOR");
  const isAdmin = (account?.roles ?? []).includes("ADMIN");
  const handle = account?.handle || account?.mobileE164 || (isGuest ? t("guest_tag") : t("member_tag"));

  // Signed-out: a focused gate to sign in or register.
  if (isGuest) {
    return (
      <ScreenShell top={<TopBar kicker={t("nav_account")} title={t("guest_tag")} right={<LangPill />} />}>
        <div style={{ padding: "32px 20px", textAlign: "center" }}>
          <span style={{ color: "var(--gold)" }}><Icon name="user" size={34} /></span>
          <h2 className="serif" style={{ fontSize: 22, color: "var(--gold-pale)", margin: "12px 0 6px" }}>{t("acc_guest_title")}</h2>
          <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, maxWidth: 300, margin: "0 auto 22px" }}>{t("acc_guest_body")}</p>
          <button className="btn btn-gold" style={{ width: "100%", maxWidth: 320, margin: "0 auto 10px" }} onClick={() => nav("/login")}>
            <Icon name="arrow-right" size={17} /> {t("auth_login")}
          </button>
          <button className="btn btn-ghost" style={{ width: "100%", maxWidth: 320, margin: "0 auto" }} onClick={() => nav("/register")}>
            {t("auth_register")}
          </button>
          <LangRow lang={lang} setLang={setLang} />
        </div>
      </ScreenShell>
    );
  }

  return (
    <ScreenShell top={<TopBar kicker={account?.id.slice(0, 10) ?? ""} title={t("nav_account")} right={<LangPill />} />}>
      <div style={{ padding: "18px 16px 28px" }}>
        {/* profile */}
        <div style={{ display: "flex", alignItems: "center", gap: 14, marginBottom: 18 }}>
          <div style={{ width: 56, height: 56, borderRadius: "50%", border: "1px solid var(--gold-line)", background: "var(--burg)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--gold-pale)" }}>
            <Icon name="user" size={26} />
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div className="serif" style={{ fontSize: 19, color: "var(--gold-pale)", overflow: "hidden", textOverflow: "ellipsis" }} dir="ltr">{handle}</div>
            <div style={{ marginTop: 5, display: "flex", gap: 6, flexWrap: "wrap" }}>
              <Chip state="active" label={`${lv?.name ?? t("member_tag")} · ${t("mem_level")} ${level}`} />
              {isInspector && <Chip state="active" label="INSPECTOR" />}
              {isAdmin && <Chip state="active" label="ADMIN" />}
            </div>
          </div>
        </div>

        {/* membership card */}
        <button onClick={() => nav("/membership")} style={{ width: "100%", textAlign: "start", cursor: "pointer", border: "1px solid var(--gold-line)", borderRadius: "var(--r-3)", background: "linear-gradient(120deg,var(--burg),var(--bg-1) 78%)", padding: "16px 18px", display: "flex", alignItems: "center", gap: 14, color: "var(--fg)", marginBottom: 16 }}>
          <span style={{ color: "var(--gold)" }}><Icon name="crown" size={24} /></span>
          <div style={{ flex: 1 }}>
            <div className="mono up" style={{ fontSize: 9.5, color: "var(--fg-faint)" }}>{t("acc_membership")}</div>
            <div className="serif" style={{ fontSize: 17, color: "var(--gold-pale)", marginTop: 2 }}>{lv?.name} · {t("mem_level")} {level}</div>
            <div className="muted" style={{ fontSize: 11.5, marginTop: 2 }}>{t("mem_fee")} {((lv?.premiumBps ?? 1200) / 100).toFixed(0)}%</div>
          </div>
          {level < MAX_LEVEL
            ? <span className="chip" data-st="warn">{t("acc_upgrade")}</span>
            : <span className="chip" data-st="good">{t("acc_top_tier")}</span>}
        </button>

        {/* billing + wallet */}
        <div className="kv">
          <div className="k">{t("clo_credit")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{vault ? usdc(vault.creditBalanceCents) : "—"}</div>
          <div className="k">{t("bid_wallet")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{wallet?.balanceCredits ?? "—"} {t("credits")}</div>
          <div className="k">{t("acc_identity")}</div><div className="v">{account?.kycStatus === "APPROVED" ? t("acc_verified") : account?.kycStatus}</div>
          {account?.mobileE164 && (<><div className="k">{t("auth_mobile")}</div><div className="v mono" dir="ltr">{account.mobileE164}</div></>)}
        </div>

        {/* quick actions */}
        <div className="resp-cards" style={{ display: "flex", flexDirection: "column", gap: 10, marginTop: 16 }}>
          <button className="btn btn-ghost" onClick={() => nav("/bidstore")}><Icon name="coins" size={16} /> {t("buy_bids")}</button>
          <button className="btn btn-ghost" onClick={() => nav("/vault")}><Icon name="package" size={16} /> {t("clo_title")}</button>
          {(isInspector || isAdmin) && (
            <button className="btn btn-ghost" onClick={() => nav("/admin")}><Icon name="shield" size={16} /> {t("adm_title")}</button>
          )}
        </div>

        <LangRow lang={lang} setLang={setLang} />

        <button className="btn btn-burg" style={{ width: "100%", marginTop: 18 }} onClick={async () => { await signOut.mutateAsync(); nav("/login"); }}>
          <Icon name="arrow-left" size={16} /> {t("acc_signout")}
        </button>
      </div>
    </ScreenShell>
  );
}

function LangRow({ lang, setLang }: { lang: string; setLang: (l: typeof LANGS[number]) => void }) {
  return (
    <div style={{ display: "flex", gap: 8, marginTop: 20 }}>
      {LANGS.map((l) => (
        <button key={l} onClick={() => setLang(l)} className="mono" style={{ flex: 1, padding: "11px", borderRadius: "var(--r-1)", border: "1px solid", borderColor: lang === l ? "var(--gold)" : "var(--line-strong)", background: lang === l ? "var(--gold)" : "transparent", color: lang === l ? "#1B1207" : "var(--fg-muted)", cursor: "pointer", fontWeight: 600 }}>
          {LABELS[l]}
        </button>
      ))}
    </div>
  );
}
