import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useOAuthLogin, useRequestOtp, useVerifyOtp } from "@/hooks/queries";
import { continueAsGuest } from "@/auth/session";
import { COUNTRIES, DEFAULT_COUNTRY } from "@/auth/countries";
import { Icon } from "@/components/ui/Icon";
import type { OAuthProvider } from "@/types";

type Mode = "login" | "register";

// Onboarding. Two distinct entry points over the same verified-by-mobile/OAuth
// flow: Login (returning) and Register (new — also asks a display name). Either
// way the account is verified and granted Member · Level 1 (free) on success.
export function AuthPage({ mode }: { mode: Mode }) {
  const { t } = useI18n();
  const nav = useNavigate();
  const requestOtp = useRequestOtp();
  const verifyOtp = useVerifyOtp();
  const oauth = useOAuthLogin();
  const register = mode === "register";

  const [step, setStep] = useState<"mobile" | "otp">("mobile");
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
    } catch {
      setErr(t("auth_err_send"));
    }
  };

  const verify = async () => {
    setErr("");
    try {
      await verifyOtp.mutateAsync({ mobile: e164, code, handle: register ? name.trim() : undefined });
      nav("/"); // verified -> Member Level 1, into the app
    } catch {
      setErr(t("auth_err_code"));
    }
  };

  const social = async (provider: OAuthProvider) => {
    setErr("");
    try {
      await oauth.mutateAsync(provider);
      nav("/");
    } catch {
      setErr(t("auth_err_oauth"));
    }
  };

  return (
    <div style={{ minHeight: "100%", display: "flex", flexDirection: "column", justifyContent: "center", padding: "32px 22px", background: "var(--bg-0)" }}>
      <div className="fade-up" style={{ width: "100%", maxWidth: 400, margin: "0 auto" }}>
        <div style={{ textAlign: "center", marginBottom: 24 }}>
          <span style={{ color: "var(--gold)", display: "inline-flex" }}><Icon name="crown" size={30} /></span>
          <h1 className="serif" style={{ fontSize: 26, color: "var(--gold-pale)", margin: "10px 0 4px" }}>
            {register ? t("auth_register_title") : t("auth_login_title")}
          </h1>
          <p className="muted" style={{ fontSize: 13, margin: 0 }}>{register ? t("auth_register_sub") : t("auth_login_sub")}</p>
        </div>

        {step === "mobile" ? (
          <>
            {register && (
              <>
                <label className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("auth_name")}</label>
                <input className="field" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("auth_name_ph")} style={{ width: "100%", marginTop: 8, marginBottom: 14 }} />
              </>
            )}

            <label className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("auth_mobile")}</label>
            <div style={{ display: "flex", gap: 8, marginTop: 8 }} dir="ltr">
              <select className="field" value={dial} onChange={(e) => setDial(e.target.value)} style={{ flex: "0 0 128px" }} aria-label="country code">
                {COUNTRIES.map((c) => (<option key={c.iso} value={c.dial}>{c.flag} {c.dial}</option>))}
              </select>
              <input className="field" dir="ltr" inputMode="tel" value={local} onChange={(e) => setLocal(e.target.value)} placeholder="50 123 4567" style={{ flex: 1 }}
                onKeyDown={(e) => { if (e.key === "Enter" && mobileValid) send(); }} />
            </div>
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 14 }} onClick={send} disabled={!mobileValid || requestOtp.isPending}>
              <Icon name="arrow-right" size={17} /> {t("auth_send_code")}
            </button>

            <div style={{ display: "flex", alignItems: "center", gap: 10, margin: "20px 0" }}>
              <div style={{ flex: 1, height: 1, background: "var(--line)" }} />
              <span className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{t("auth_or")}</span>
              <div style={{ flex: 1, height: 1, background: "var(--line)" }} />
            </div>

            <button className="btn btn-ghost" style={{ width: "100%", marginBottom: 10 }} onClick={() => social("GOOGLE")} disabled={oauth.isPending}>
              <Icon name="user" size={17} /> {t("auth_google")}
            </button>
            <button className="btn btn-ghost" style={{ width: "100%" }} onClick={() => social("FACEBOOK")} disabled={oauth.isPending}>
              <Icon name="users" size={17} /> {t("auth_facebook")}
            </button>
          </>
        ) : (
          <>
            <label className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em" }}>{t("auth_enter_code")}</label>
            <p className="muted" style={{ fontSize: 12, margin: "6px 0 10px" }} dir="ltr">{e164}</p>
            <input className="field" dir="ltr" inputMode="numeric" value={code} maxLength={6}
              onChange={(e) => setCode(e.target.value.replace(/[^\d]/g, ""))}
              placeholder="000000" style={{ width: "100%", textAlign: "center", letterSpacing: "0.4em", fontSize: 20 }}
              onKeyDown={(e) => { if (e.key === "Enter" && code.length >= 4) verify(); }} />
            {devCode && <p className="mono" style={{ fontSize: 11, color: "var(--fg-faint)", marginTop: 8, textAlign: "center" }}>demo code · {devCode}</p>}
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 14 }} onClick={verify} disabled={code.length < 4 || verifyOtp.isPending}>
              <Icon name="check" size={17} /> {t("auth_verify")}
            </button>
            <button className="btn btn-ghost" style={{ width: "100%", marginTop: 10 }} onClick={() => setStep("mobile")}>
              {t("auth_change_number")}
            </button>
          </>
        )}

        {err && <p style={{ color: "var(--st-bad)", fontSize: 13, marginTop: 14, textAlign: "center" }}>{err}</p>}

        {/* cross-link between login and register */}
        <div style={{ textAlign: "center", marginTop: 22, fontSize: 13, color: "var(--fg-muted)" }}>
          {register ? t("auth_have_account") : t("auth_no_account")}{" "}
          <button onClick={() => nav(register ? "/login" : "/register")} style={{ background: "none", border: "none", color: "var(--gold-pale)", cursor: "pointer", fontWeight: 600, textDecoration: "underline", fontSize: 13 }}>
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
