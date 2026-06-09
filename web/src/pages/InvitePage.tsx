import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useRedeemInvite } from "@/hooks/queries";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { Icon } from "@/components/ui/Icon";
import { Seal } from "@/components/ui/Seal";
import { iconBtnStyle } from "@/components/ui/Primitives";

export function InvitePage() {
  const { t, dir } = useI18n();
  const nav = useNavigate();
  const redeem = useRedeemInvite();
  const [code, setCode] = useState("");
  const [err, setErr] = useState(false);
  const [ok, setOk] = useState(false);

  const submit = async () => {
    const c = code.trim().toUpperCase();
    if (!c) return;
    try {
      await redeem.mutateAsync(c);
      setErr(false);
      setOk(true);
      setTimeout(() => nav("/kyc"), 1300);
    } catch {
      setErr(true);
      setOk(false);
    }
  };

  return (
    <ScreenShell>
      <div style={{ minHeight: "100%", display: "flex", flexDirection: "column", padding: "54px 24px 24px", background: "radial-gradient(130% 80% at 50% -10%, var(--burg-deep), var(--bg-void) 60%)" }}>
        <button onClick={() => nav("/")} aria-label={t("common_close")} style={{ ...iconBtnStyle, position: "absolute", top: 54, insetInlineStart: 16 }}>
          <Icon name="x" size={18} />
        </button>
        <div style={{ height: 70 }} />
        <div style={{ textAlign: "center" }}>
          <div style={{ color: "var(--gold)", display: "inline-flex", marginBottom: 18 }}><Icon name="crown" size={34} /></div>
          <div className="serif" style={{ fontSize: 40, letterSpacing: "0.04em", color: "var(--gold-pale)", lineHeight: 1 }}>{t("brand")}</div>
          <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginTop: 12, letterSpacing: "0.22em" }}>{t("inv_kicker")}</div>
          <p className="muted" style={{ fontSize: 12.5, lineHeight: 1.6, margin: "12px auto 0", maxWidth: 280 }}>{t("invite_elevate")}</p>
        </div>

        <div style={{ flex: 1 }} />

        <div className="fade-up" style={{ paddingBottom: 14 }}>
          <h1 className="serif" style={{ fontSize: 26, margin: "0 0 10px", color: "var(--fg)", textAlign: "center" }}>{t("inv_title")}</h1>
          <p className="muted" style={{ fontSize: 14, lineHeight: 1.6, textAlign: "center", margin: "0 0 22px" }}>{t("inv_body")}</p>

          {ok ? (
            <div className="fade-up" style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 12, padding: "10px 0 18px" }}>
              <Seal size={92} label="MEMBER" sub="DAUCTION" id="0x11" date="2026" live />
              <div style={{ color: "var(--st-good)", display: "flex", alignItems: "center", gap: 8, fontWeight: 600 }}>
                <Icon name="check" size={18} /> {t("tier_up")}
              </div>
            </div>
          ) : (
            <>
              <input
                className="field field-mono"
                value={code}
                dir="ltr"
                onChange={(e) => { setCode(e.target.value); setErr(false); }}
                onKeyDown={(e) => e.key === "Enter" && submit()}
                placeholder={t("inv_ph")}
                style={{ textAlign: "center", fontSize: 18, borderColor: err ? "var(--st-bad)" : undefined }}
              />
              {err && (
                <div style={{ color: "var(--st-bad)", fontSize: 12.5, marginTop: 8, display: "flex", gap: 6, alignItems: "center" }}>
                  <Icon name="alert" size={14} /> {t("inv_err")}
                </div>
              )}
              <button className="btn btn-gold" style={{ width: "100%", marginTop: 14 }} onClick={submit} disabled={!code.trim() || redeem.isPending}>
                {t("inv_cta")} <Icon name={dir === "rtl" ? "arrow-left" : "arrow-right"} size={18} />
              </button>
              <div className="mono" style={{ fontSize: 10.5, color: "var(--fg-faint)", textAlign: "center", marginTop: 18, letterSpacing: "0.04em" }}>{t("inv_demo")}</div>
            </>
          )}
        </div>
        <div style={{ height: 20 }} />
      </div>
    </ScreenShell>
  );
}
