import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useCheckOtp, useDemoLogin, useOAuthLogin, useRequestOtp, useVerifyOtp } from "@/hooks/queries";
import { continueAsGuest } from "@/auth/session";
import { COUNTRIES, DEFAULT_COUNTRY } from "@/auth/countries";
import { Icon } from "@/components/ui/Icon";
import type { OAuthProvider } from "@/types";

type Mode = "login" | "register";
type Step = "mobile" | "otp" | "connect";

// Onboarding. Login: mobile-OTP or one-tap social. Register is a 2-step wizard —
// (1) verify the mobile number, then (2) connect Google/Facebook (which supplies
// the profile image). Either path ends verified as Member · Level 1.
export function AuthPage({ mode }: { mode: Mode }) {
  const { t } = useI18n();
  const nav = useNavigate();
  const requestOtp = useRequestOtp();
  const checkOtp = useCheckOtp();
  const verifyOtp = useVerifyOtp();
  const oauth = useOAuthLogin();
  const demo = useDemoLogin();
  const register = mode === "register";

  const [step, setStep] = useState<Step>("mobile");
  const [name, setName] = useState("");
  const [dial, setDial] = useState(DEFAULT_COUNTRY.dial);
  const [local, setLocal] = useState("");
  const [code, setCode] = useState("");
  const [devCode, setDevCode] = useState<string | undefined>();
  const [err, setErr] = useState("");

  const e164 = `${dial}${local.replace(/[^\d]/g, "")}`;
  const mobileValid = local.replace(/[^\d]/g, "").length >= 6;

  const send = async () => {
    setErr("");
    try {
      const r = await requestOtp.mutateAsync(e164);
      setDevCode(r.devCode);
      if (r.devCode) setCode(r.devCode);
      setStep("otp");
    } catch { setErr(t("auth_err_send")); }
  };

  const onOtp = async () => {
    setErr("");
    try {
      if (register) {
        await checkOtp.mutateAsync({ mobile: e164, code }); // verify only
        setStep("connect");                                  // -> step 2
      } else {
        await verifyOtp.mutateAsync({ mobile: e164, code }); // login: sign in
        nav("/");
      }
    } catch { setErr(t("auth_err_code")); }
  };

  const social = async (provider: OAuthProvider) => {
    setErr("");
    try {
      // In the register wizard, attach the already-verified mobile + name.
      await oauth.mutateAsync(register ? { provider, mobile: e164, name } : { provider });
      nav("/");
    } catch { setErr(t("auth_err_oauth")); }
  };

  const demoLogin = async (p: string) => { await demo.mutateAsync(p); nav("/"); };
  const busy = requestOtp.isPending || checkOtp.isPending || verifyOtp.isPending || oauth.isPending;

  return (
    <div style={{ minHeight: "100%", display: "flex", flexDirection: "column", justifyContent: "center", padding: "32px 22px", background: "var(--bg-0)" }}>
      <div className="fade-up" style={{ width: "100%", maxWidth: 400, margin: "0 auto" }}>
        <div style={{ textAlign: "center", marginBottom: 22 }}>
          <span style={{ color: "var(--gold)", display: "inline-flex" }}><Icon name="crown" size={30} /></span>
          <h1 className="serif" style={{ fontSize: 26, color: "var(--gold-pale)", margin: "10px 0 4px" }}>
            {register ? t("auth_register_title") : t("auth_login_title")}
          </h1>
          <p className="muted" style={{ fontSize: 13, margin: 0 }}>{register ? t("auth_register_sub") : t("auth_login_sub")}</p>
        </div>

        {/* register wizard progress */}
        {register && (
          <div style={{ display: "flex", gap: 8, marginBottom: 20 }}>
            {[t("auth_step_mobile"), t("auth_step_connect")].map((label, i) => {
              const active = (i === 0 && step !== "connect") || (i === 1 && step === "connect");
              const done = i === 0 && step === "connect";
              return (
                <div key={i} style={{ flex: 1 }}>
                  <div style={{ height: 3, borderRadius: 2, background: active || done ? "var(--gold)" : "var(--line-strong)" }} />
                  <div className="mono up" style={{ fontSize: 8.5, marginTop: 5, color: active ? "var(--gold-pale)" : "var(--fg-faint)", letterSpacing: "0.08em" }}>{i + 1} · {label}</div>
                </div>
              );
            })}
          </div>
        )}

        {step === "mobile" && (
          <>
            {register && (<>
              <label className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("auth_name")}</label>
              <input className="field" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("auth_name_ph")} style={{ width: "100%", marginTop: 8, marginBottom: 14 }} />
            </>)}

            <label className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("auth_mobile")}</label>
            <div style={{ display: "flex", gap: 8, marginTop: 8 }} dir="ltr">
              <select className="field" value={dial} onChange={(e) => setDial(e.target.value)} style={{ flex: "0 0 128px" }} aria-label="country code">
                {COUNTRIES.map((c) => (<option key={c.iso} value={c.dial}>{c.flag} {c.dial}</option>))}
              </select>
              <input className="field" dir="ltr" inputMode="tel" value={local} onChange={(e) => setLocal(e.target.value)} placeholder="50 123 4567" style={{ flex: 1 }}
                onKeyDown={(e) => { if (e.key === "Enter" && mobileValid) send(); }} />
            </div>
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 14 }} onClick={send} disabled={!mobileValid || busy}>
              <Icon name="arrow-right" size={17} /> {t("auth_send_code")}
            </button>

            {/* login also offers one-tap social + demo accounts */}
            {!register && (<>
              <Divider label={t("auth_or")} />
              <button className="btn btn-ghost" style={{ width: "100%", marginBottom: 10 }} onClick={() => social("GOOGLE")} disabled={busy}><Icon name="user" size={17} /> {t("auth_google")}</button>
              <button className="btn btn-ghost" style={{ width: "100%" }} onClick={() => social("FACEBOOK")} disabled={busy}><Icon name="users" size={17} /> {t("auth_facebook")}</button>
              <DemoPanel onPick={demoLogin} busy={demo.isPending} />
            </>)}
          </>
        )}

        {step === "otp" && (
          <>
            <label className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("auth_enter_code")}</label>
            <p className="muted" style={{ fontSize: 12, margin: "6px 0 10px" }} dir="ltr">{e164}</p>
            <input className="field" dir="ltr" inputMode="numeric" value={code} maxLength={6}
              onChange={(e) => setCode(e.target.value.replace(/[^\d]/g, ""))} placeholder="000000"
              style={{ width: "100%", textAlign: "center", letterSpacing: "0.4em", fontSize: 20 }}
              onKeyDown={(e) => { if (e.key === "Enter" && code.length >= 4) onOtp(); }} />
            {devCode && <p className="mono" style={{ fontSize: 11, color: "var(--fg-faint)", marginTop: 8, textAlign: "center" }}>demo code · {devCode}</p>}
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 14 }} onClick={onOtp} disabled={code.length < 4 || busy}>
              <Icon name={register ? "arrow-right" : "check"} size={17} /> {register ? t("common_continue") : t("auth_verify")}
            </button>
            <button className="btn btn-ghost" style={{ width: "100%", marginTop: 10 }} onClick={() => setStep("mobile")}>{t("auth_change_number")}</button>
          </>
        )}

        {step === "connect" && (
          <>
            <div style={{ textAlign: "center", marginBottom: 14 }}>
              <span className="chip" data-st="good"><Icon name="check" size={12} /> {t("auth_mobile_verified")}</span>
            </div>
            <p className="muted" style={{ fontSize: 13, textAlign: "center", margin: "0 0 16px" }}>{t("auth_connect_body")}</p>
            <button className="btn btn-gold" style={{ width: "100%", marginBottom: 10 }} onClick={() => social("GOOGLE")} disabled={busy}><Icon name="user" size={17} /> {t("auth_connect_google")}</button>
            <button className="btn btn-ghost" style={{ width: "100%" }} onClick={() => social("FACEBOOK")} disabled={busy}><Icon name="users" size={17} /> {t("auth_connect_facebook")}</button>
            <button className="btn btn-ghost" style={{ width: "100%", marginTop: 10 }} onClick={() => setStep("mobile")}>{t("auth_change_number")}</button>
          </>
        )}

        {err && <p style={{ color: "var(--st-bad)", fontSize: 13, marginTop: 14, textAlign: "center" }}>{err}</p>}

        <div style={{ textAlign: "center", marginTop: 22, fontSize: 13, color: "var(--fg-muted)" }}>
          {register ? t("auth_have_account") : t("auth_no_account")}{" "}
          <button onClick={() => { setStep("mobile"); nav(register ? "/login" : "/register"); }} style={{ background: "none", border: "none", color: "var(--gold-pale)", cursor: "pointer", fontWeight: 600, textDecoration: "underline", fontSize: 13 }}>
            {register ? t("auth_login") : t("auth_register")}
          </button>
        </div>

        <button onClick={() => { continueAsGuest(); nav("/"); }} style={{ width: "100%", marginTop: 16, background: "none", border: "none", color: "var(--fg-faint)", fontSize: 13, cursor: "pointer", textDecoration: "underline" }}>
          {t("auth_guest")}
        </button>
      </div>
    </div>
  );
}

function Divider({ label }: { label: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, margin: "20px 0" }}>
      <div style={{ flex: 1, height: 1, background: "var(--line)" }} />
      <span className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{label}</span>
      <div style={{ flex: 1, height: 1, background: "var(--line)" }} />
    </div>
  );
}

function DemoPanel({ onPick, busy }: { onPick: (p: string) => void; busy: boolean }) {
  const { t } = useI18n();
  return (
    <div style={{ marginTop: 22, border: "1px solid var(--line)", borderRadius: "var(--r-2)", padding: "12px 14px", background: "var(--bg-1)" }}>
      <div className="mono up" style={{ fontSize: 9, color: "var(--gold)", letterSpacing: "0.12em", marginBottom: 10 }}>{t("auth_demo_title")}</div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
        {[["member", t("auth_demo_member")], ["gold", t("auth_demo_gold")], ["platinum", t("auth_demo_platinum")], ["inspector", t("auth_demo_inspector")]].map(([k, label]) => (
          <button key={k} onClick={() => onPick(k)} disabled={busy} className="mono" style={{ fontSize: 11.5, padding: "10px 8px", borderRadius: "var(--r-1)", border: "1px solid var(--line-strong)", background: "var(--bg-0)", color: "var(--fg-muted)", cursor: "pointer", fontWeight: 600 }}>{label}</button>
        ))}
      </div>
      <div className="mono" style={{ fontSize: 10, color: "var(--fg-faint)", marginTop: 10, textAlign: "center" }}>{t("auth_demo_admin")}</div>
    </div>
  );
}
