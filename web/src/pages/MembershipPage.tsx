import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useSession } from "@/hooks/useSession";
import { useUpgradeMembership } from "@/hooks/queries";
import { LEVELS, levelOf, type MembershipLevel } from "@/lib/membership";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Sheet } from "@/components/ui/Sheet";
import { Icon } from "@/components/ui/Icon";
import { dollars } from "@/lib/format";

export function MembershipPage() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { account, isGuest } = useSession();
  const current = levelOf(account);
  const upgrade = useUpgradeMembership();
  const [pay, setPay] = useState<MembershipLevel | null>(null);

  const currentLevel = LEVELS.find((l) => l.level === current);

  return (
    <>
      <ScreenShell top={<TopBar kicker={t("mem_sub")} title={t("mem_title")} right={<LangPill />} />}>
        <div style={{ padding: "16px 16px 28px" }}>
          {/* current standing */}
          <div className="fade-up" style={{ borderRadius: "var(--r-3)", border: "1px solid var(--gold-line)", background: "linear-gradient(120deg,var(--burg),var(--bg-1) 75%)", padding: "16px 18px", marginBottom: 20, display: "flex", alignItems: "center", gap: 14 }}>
            <span style={{ color: "var(--gold)" }}><Icon name="crown" size={26} /></span>
            <div style={{ flex: 1 }}>
              <div className="mono up" style={{ fontSize: 9.5, color: "var(--fg-faint)", letterSpacing: "0.12em" }}>{t("mem_current")}</div>
              <div className="serif" style={{ fontSize: 21, color: "var(--gold-pale)", marginTop: 2 }}>
                {isGuest ? t("mem_guest") : `${currentLevel?.name ?? t("member_tag")} · ${t("mem_level")} ${current}`}
              </div>
            </div>
            {currentLevel && (
              <div style={{ textAlign: "end" }}>
                <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{t("mem_fee")}</div>
                <div className="mono" style={{ fontSize: 20, color: "var(--gold-pale)" }}>{(currentLevel.premiumBps / 100).toFixed(0)}%</div>
              </div>
            )}
          </div>

          {isGuest && (
            <button className="btn btn-gold" style={{ width: "100%", marginBottom: 18 }} onClick={() => nav("/login")}>
              <Icon name="arrow-right" size={17} /> {t("mem_signin_cta")}
            </button>
          )}

          {/* levels ladder */}
          <div className="resp-cards" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {LEVELS.map((lv) => {
              const owned = current >= lv.level && !isGuest;
              const isCurrent = current === lv.level && !isGuest;
              const free = lv.priceCentsYear === 0;
              const top = lv.level === LEVELS.length;
              return (
                <div key={lv.level} className="fade-up" style={{ position: "relative", borderRadius: "var(--r-3)", overflow: "hidden", border: "1px solid", borderColor: isCurrent ? "var(--gold)" : top ? "var(--gold-line)" : "var(--line)", background: top ? "linear-gradient(135deg,var(--burg),var(--bg-1) 70%)" : "var(--bg-1)", padding: "18px 18px", display: "flex", flexDirection: "column" }}>
                  {isCurrent && <div style={{ position: "absolute", top: 0, insetInlineEnd: 0 }}><span className="chip" data-st="live" style={{ borderRadius: "0 0 0 var(--r-2)" }}>{t("mem_current_plan")}</span></div>}

                  <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 4 }}>
                    <span className="mono up" style={{ fontSize: 9, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("mem_level")} {lv.level}</span>
                  </div>
                  <div className="serif" style={{ fontSize: 24, color: lv.level >= 2 ? "var(--gold-pale)" : "var(--fg)" }}>{lv.name}</div>

                  <div style={{ display: "flex", alignItems: "baseline", gap: 6, margin: "10px 0 4px" }}>
                    <span className="mono" style={{ fontSize: 26, color: "var(--gold-pale)" }}>{free ? t("mem_free") : dollars(lv.priceCentsYear)}</span>
                    {!free && <span className="muted" style={{ fontSize: 12 }}>{t("mem_per_year")}</span>}
                  </div>
                  <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)", marginBottom: 12 }}>{t("mem_fee")} · {(lv.premiumBps / 100).toFixed(0)}%</div>

                  <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 16, flex: 1 }}>
                    {lv.perkKeys.map((p) => (
                      <div key={p} style={{ display: "flex", gap: 9, alignItems: "flex-start" }}>
                        <span style={{ color: "var(--gold)", flexShrink: 0, marginTop: 1 }}><Icon name="check" size={14} /></span>
                        <span style={{ fontSize: 13, lineHeight: 1.4, color: "var(--fg-muted)" }}>{t(p)}</span>
                      </div>
                    ))}
                  </div>

                  {isCurrent ? (
                    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, padding: "12px", border: "1px solid var(--st-good)", borderRadius: "var(--r-1)", background: "var(--st-good-bg)", color: "var(--st-good)", fontWeight: 600, fontSize: 13.5 }}>
                      <Icon name="check" size={16} /> {t("mem_current_plan")}
                    </div>
                  ) : owned ? (
                    <div className="mono up" style={{ textAlign: "center", fontSize: 10, color: "var(--fg-faint)", padding: "12px" }}>{t("mem_included")}</div>
                  ) : free ? (
                    <button className="btn btn-ghost" style={{ width: "100%" }} onClick={() => nav("/login")} disabled={!isGuest}>
                      {isGuest ? t("mem_signin_cta") : t("mem_included")}
                    </button>
                  ) : (
                    <button className={"btn " + (top ? "btn-gold" : "btn-ghost")} style={{ width: "100%" }} disabled={isGuest} onClick={() => setPay(lv)}>
                      <Icon name="crown" size={16} /> {t("mem_upgrade")} · {dollars(lv.priceCentsYear)}
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </ScreenShell>

      <PaymentSheet
        level={pay}
        onClose={() => setPay(null)}
        onPaid={async (lv) => { await upgrade.mutateAsync(lv.level); setPay(null); }}
        pending={upgrade.isPending}
      />
    </>
  );
}

// PaymentSheet — a mock card checkout. No real processor; on "Pay" it simulates
// success and applies the membership upgrade. Production wires a PSP (Stripe…).
function PaymentSheet({ level, onClose, onPaid, pending }: {
  level: MembershipLevel | null; onClose: () => void; onPaid: (lv: MembershipLevel) => void; pending: boolean;
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
