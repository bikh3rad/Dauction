import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { LABELS, LANGS } from "@/i18n/locales";
import { useSession } from "@/hooks/useSession";
import { useWallet, useVault, useSignOut, useSetAvatar, useUpgradeMembership } from "@/hooks/queries";
import { getLevels, levelInfo, type MembershipLevel } from "@/lib/membership";
import { useSettings } from "@/mock/settings";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Sheet } from "@/components/ui/Sheet";
import { Avatar } from "@/components/ui/Avatar";
import { Icon } from "@/components/ui/Icon";
import { Chip } from "@/components/ui/Chip";
import { Money } from "@/components/ui/Money";
import { usdc, dollars } from "@/lib/format";

// One merged Account + Membership view: profile (avatar + unique username +
// roles), the paid membership levels ladder with checkout, billing/wallet,
// language and sign out.
export function ProfilePage() {
  const { t, lang, setLang } = useI18n();
  const nav = useNavigate();
  const { account, level, isGuest } = useSession();
  const { data: wallet } = useWallet();
  const { data: vault } = useVault();
  const signOut = useSignOut();
  const setAvatar = useSetAvatar();
  const upgrade = useUpgradeMembership();
  const fileRef = useRef<HTMLInputElement>(null);
  const [pay, setPay] = useState<MembershipLevel | null>(null);
  useSettings(); // re-render when an admin edits the membership levels
  const LEVELS = getLevels();

  if (isGuest) {
    return (
      <ScreenShell top={<TopBar kicker={t("nav_profile")} title={t("guest_tag")} right={<LangPill />} />}>
        <div style={{ padding: "32px 20px", textAlign: "center" }}>
          <span style={{ color: "var(--gold)" }}><Icon name="user" size={34} /></span>
          <h2 className="serif" style={{ fontSize: 22, color: "var(--gold-pale)", margin: "12px 0 6px" }}>{t("acc_guest_title")}</h2>
          <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, maxWidth: 300, margin: "0 auto 22px" }}>{t("acc_guest_body")}</p>
          <button className="btn btn-gold" style={{ width: "100%", maxWidth: 320, margin: "0 auto 10px" }} onClick={() => nav("/login")}><Icon name="arrow-right" size={17} /> {t("auth_login")}</button>
          <button className="btn btn-ghost" style={{ width: "100%", maxWidth: 320, margin: "0 auto" }} onClick={() => nav("/register")}>{t("auth_register")}</button>
        </div>
      </ScreenShell>
    );
  }

  const lv = levelInfo(level);
  const roles = account?.roles ?? [];

  const onAvatar = (file?: File) => {
    if (!file) return;
    const r = new FileReader();
    r.onload = () => setAvatar.mutate(String(r.result)); // data URL persists in the session
    r.readAsDataURL(file);
  };

  return (
    <>
      <ScreenShell top={<TopBar kicker={`@${account?.handle ?? ""}`} title={t("nav_profile")} right={<LangPill />} />}>
        <div style={{ padding: "18px 16px 28px" }}>
          {/* profile header */}
          <div style={{ display: "flex", alignItems: "center", gap: 14, marginBottom: 20 }}>
            <div style={{ position: "relative" }}>
              <Avatar account={account} size={64} />
              <button onClick={() => fileRef.current?.click()} aria-label={t("acc_change_photo")}
                style={{ position: "absolute", bottom: -2, insetInlineEnd: -2, width: 24, height: 24, borderRadius: "50%", border: "1px solid var(--gold-line)", background: "var(--bg-0)", color: "var(--gold-pale)", cursor: "pointer", display: "flex", alignItems: "center", justifyContent: "center" }}>
                <Icon name="plus" size={13} />
              </button>
              <input ref={fileRef} type="file" accept="image/*" style={{ display: "none" }} onChange={(e) => onAvatar(e.target.files?.[0])} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="serif" style={{ fontSize: 20, color: "var(--gold-pale)", overflow: "hidden", textOverflow: "ellipsis" }} dir="ltr">@{account?.handle}</div>
              <div style={{ marginTop: 5, display: "flex", gap: 6, flexWrap: "wrap" }}>
                <Chip state="active" label={`${lv?.name ?? t("member_tag")} · ${t("mem_level")} ${level}`} />
                {roles.map((r) => <Chip key={r} state="active" label={r} />)}
              </div>
            </div>
          </div>

          {/* billing + wallet */}
          <div className="kv" style={{ marginBottom: 20 }}>
            <div className="k">{t("clo_credit")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{vault ? usdc(vault.creditBalanceCents) : "—"}</div>
            <div className="k">{t("bid_wallet")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{wallet?.balanceCredits ?? "—"} {t("credits")}</div>
            <div className="k">{t("acc_identity")}</div><div className="v">{account?.kycStatus === "APPROVED" ? t("acc_verified") : account?.kycStatus}</div>
            {account?.mobileE164 && (<><div className="k">{t("auth_mobile")}</div><div className="v mono" dir="ltr">{account.mobileE164}</div></>)}
          </div>

          {/* membership levels */}
          <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em", marginBottom: 12 }}>{t("acc_membership")}</div>
          <div className="resp-cards" style={{ display: "flex", flexDirection: "column", gap: 12, marginBottom: 22 }}>
            {LEVELS.map((l) => {
              const isCurrent = level === l.level;
              const owned = level >= l.level;
              const free = l.priceCentsYear === 0;
              const top = l.level === LEVELS.length;
              return (
                <div key={l.level} style={{ position: "relative", borderRadius: "var(--r-3)", overflow: "hidden", border: "1px solid", borderColor: isCurrent ? "var(--gold)" : top ? "var(--gold-line)" : "var(--line)", background: top ? "linear-gradient(135deg,var(--burg),var(--bg-1) 70%)" : "var(--bg-1)", padding: "16px 16px", display: "flex", flexDirection: "column" }}>
                  {isCurrent && <div style={{ position: "absolute", top: 0, insetInlineEnd: 0 }}><span className="chip" data-st="live" style={{ borderRadius: "0 0 0 var(--r-2)" }}>{t("mem_current_plan")}</span></div>}
                  <span className="mono up" style={{ fontSize: 9, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("mem_level")} {l.level}</span>
                  <div className="serif" style={{ fontSize: 22, color: l.level >= 2 ? "var(--gold-pale)" : "var(--fg)" }}>{l.name}</div>
                  <div style={{ display: "flex", alignItems: "baseline", gap: 6, margin: "8px 0 4px" }}>
                    <span className="mono" style={{ fontSize: 24, color: "var(--gold-pale)" }}>{free ? t("mem_free") : dollars(l.priceCentsYear)}</span>
                    {!free && <span className="muted" style={{ fontSize: 12 }}>{t("mem_per_year")}</span>}
                  </div>
                  <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)", marginBottom: 12 }}>{t("mem_fee")} · {(l.premiumBps / 100).toFixed(0)}%</div>
                  <div style={{ display: "flex", flexDirection: "column", gap: 7, marginBottom: 14 }}>
                    {l.perkKeys.map((p) => (
                      <div key={p} style={{ display: "flex", gap: 8, alignItems: "flex-start" }}>
                        <span style={{ color: "var(--gold)", flexShrink: 0, marginTop: 1 }}><Icon name="check" size={13} /></span>
                        <span style={{ fontSize: 12.5, lineHeight: 1.4, color: "var(--fg-muted)" }}>{t(p)}</span>
                      </div>
                    ))}
                  </div>
                  {isCurrent ? (
                    <div style={{ textAlign: "center", padding: "10px", border: "1px solid var(--st-good)", borderRadius: "var(--r-1)", background: "var(--st-good-bg)", color: "var(--st-good)", fontWeight: 600, fontSize: 13 }}><Icon name="check" size={15} /> {t("mem_current_plan")}</div>
                  ) : owned ? (
                    <div className="mono up" style={{ textAlign: "center", fontSize: 10, color: "var(--fg-faint)", padding: "10px" }}>{t("mem_included")}</div>
                  ) : (
                    <button className={"btn " + (top ? "btn-gold" : "btn-ghost")} style={{ width: "100%" }} onClick={() => setPay(l)}><Icon name="crown" size={15} /> {t("mem_upgrade")} · {dollars(l.priceCentsYear)}</button>
                  )}
                </div>
              );
            })}
          </div>

          {/* quick actions */}
          <div className="resp-cards" style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <button className="btn btn-ghost" onClick={() => nav("/bidstore")}><Icon name="coins" size={16} /> {t("buy_bids")}</button>
            <button className="btn btn-ghost" onClick={() => nav("/vault")}><Icon name="package" size={16} /> {t("clo_title")}</button>
          </div>

          <div style={{ display: "flex", gap: 8, marginTop: 20 }}>
            {LANGS.map((l) => (
              <button key={l} onClick={() => setLang(l)} className="mono" style={{ flex: 1, padding: "11px", borderRadius: "var(--r-1)", border: "1px solid", borderColor: lang === l ? "var(--gold)" : "var(--line-strong)", background: lang === l ? "var(--gold)" : "transparent", color: lang === l ? "#1B1207" : "var(--fg-muted)", cursor: "pointer", fontWeight: 600 }}>{LABELS[l]}</button>
            ))}
          </div>

          <button className="btn btn-burg" style={{ width: "100%", marginTop: 18 }} onClick={async () => { await signOut.mutateAsync(); nav("/login"); }}>
            <Icon name="arrow-left" size={16} /> {t("acc_signout")}
          </button>
        </div>
      </ScreenShell>

      <PaymentSheet level={pay} onClose={() => setPay(null)} onPaid={async (l) => { await upgrade.mutateAsync(l.level); setPay(null); }} pending={upgrade.isPending} />
    </>
  );
}

function PaymentSheet({ level, onClose, onPaid, pending }: {
  level: MembershipLevel | null; onClose: () => void; onPaid: (l: MembershipLevel) => void; pending: boolean;
}) {
  const { t } = useI18n();
  const [card, setCard] = useState("");
  const [exp, setExp] = useState("");
  const [cvc, setCvc] = useState("");
  const valid = card.replace(/\s/g, "").length >= 12 && exp.length >= 4 && cvc.length >= 3;
  if (!level) return null;
  return (
    <Sheet open={!!level} onClose={onClose}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
        <span style={{ color: "var(--gold)" }}><Icon name="lock" size={20} /></span>
        <div className="serif" style={{ fontSize: 20, color: "var(--gold-pale)" }}>{t("pay_title")}</div>
      </div>
      <p className="muted" style={{ fontSize: 13, margin: "0 0 16px" }}>{level.name} · {t("mem_level")} {level.level} — <span className="mono" style={{ color: "var(--gold-pale)" }}>{dollars(level.priceCentsYear)}{t("mem_per_year")}</span></p>
      <label className="mono up" style={{ fontSize: 9.5, color: "var(--gold)" }}>{t("pay_card")}</label>
      <input className="field" dir="ltr" inputMode="numeric" value={card} onChange={(e) => setCard(e.target.value)} placeholder="4242 4242 4242 4242" style={{ width: "100%", marginTop: 6, marginBottom: 12 }} />
      <div style={{ display: "flex", gap: 10 }} dir="ltr">
        <div style={{ flex: 1 }}>
          <label className="mono up" style={{ fontSize: 9.5, color: "var(--gold)" }}>{t("pay_exp")}</label>
          <input className="field" value={exp} onChange={(e) => setExp(e.target.value)} placeholder="12 / 28" style={{ width: "100%", marginTop: 6 }} />
        </div>
        <div style={{ flex: 1 }}>
          <label className="mono up" style={{ fontSize: 9.5, color: "var(--gold)" }}>{t("pay_cvc")}</label>
          <input className="field" inputMode="numeric" value={cvc} onChange={(e) => setCvc(e.target.value.replace(/\D/g, ""))} placeholder="123" maxLength={4} style={{ width: "100%", marginTop: 6 }} />
        </div>
      </div>
      <button className="btn btn-gold" style={{ width: "100%", marginTop: 18 }} disabled={!valid || pending} onClick={() => onPaid(level)}>
        <Icon name="lock" size={16} /> {pending ? t("pay_processing") : `${t("pay_btn")} ${dollars(level.priceCentsYear)}`}
      </button>
      <p className="mono" style={{ fontSize: 10, color: "var(--fg-faint)", textAlign: "center", marginTop: 12 }}>{t("pay_secure")}</p>
    </Sheet>
  );
}
