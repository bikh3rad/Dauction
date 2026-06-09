import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useStartKyc, useVerifyKyc } from "@/hooks/queries";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { Icon } from "@/components/ui/Icon";
import { Seal } from "@/components/ui/Seal";
import { Chip } from "@/components/ui/Chip";
import { Label, OtpInput, Stepper } from "@/components/ui/Primitives";

export function KycPage() {
  const { t } = useI18n();
  const nav = useNavigate();
  const startKyc = useStartKyc();
  const verifyKyc = useVerifyKyc();

  const [step, setStep] = useState(0); // 0 phone, 1 otp, 2 doc, 3 pending
  const [phone, setPhone] = useState("");
  const [otp, setOtp] = useState("");
  const [doc, setDoc] = useState(false);

  const sendOtp = async () => {
    if (!phone) return;
    await startKyc.mutateAsync({ docType: "EMIRATES_ID", docRef: "scan-pending", phone });
    setStep(1);
  };
  const verify = async () => {
    await verifyKyc.mutateAsync(otp);
    setStep(2);
  };
  const submit = async () => {
    setStep(3);
  };

  return (
    <ScreenShell top={<TopBar kicker={t("kyc_kicker")} title={t("kyc_title")} onBack={() => nav(-1)} />}>
      <div style={{ padding: "18px 22px 40px" }}>
        <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, margin: "0 0 22px" }}>{t("kyc_body")}</p>

        <Stepper steps={["1", "2", "3"]} active={Math.min(step, 2)} />

        {step <= 1 && (
          <div className="fade-up" style={{ marginTop: 24 }}>
            <Label>{t("kyc_phone")}</Label>
            <div style={{ display: "flex", gap: 8 }}>
              <input className="field" dir="ltr" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+971 5X XXX XXXX" style={{ flex: 1 }} />
              <button className="btn btn-ghost" onClick={sendOtp} disabled={!phone || startKyc.isPending}>{t("kyc_send_otp")}</button>
            </div>
            {step === 1 && (
              <div className="fade-up" style={{ marginTop: 18 }}>
                <Label>{t("kyc_otp")}</Label>
                <OtpInput value={otp} onChange={setOtp} />
                <button className="btn btn-gold" style={{ width: "100%", marginTop: 16 }} disabled={otp.length < 4 || verifyKyc.isPending} onClick={verify}>
                  {t("common_continue")} <Icon name="arrow-right" size={18} />
                </button>
              </div>
            )}
          </div>
        )}

        {step === 2 && (
          <div className="fade-up" style={{ marginTop: 24 }}>
            <Label>{t("kyc_doc")}</Label>
            <button onClick={() => setDoc(true)} style={{ width: "100%", cursor: "pointer", textAlign: "start", border: "1px dashed var(--gold-line)", background: doc ? "var(--bg-2)" : "var(--bg-1)", borderRadius: "var(--r-2)", padding: "22px 18px", color: "var(--fg)", display: "flex", gap: 14, alignItems: "center" }}>
              <div style={{ color: doc ? "var(--st-good)" : "var(--gold)" }}><Icon name={doc ? "check" : "upload"} size={26} /></div>
              <div>
                <div style={{ fontWeight: 600, fontSize: 14.5 }}>{doc ? "emirates-id.scan.pdf" : t("kyc_doc")}</div>
                <div className="muted" style={{ fontSize: 12, marginTop: 2 }}>{doc ? "1.2 MB · encrypted" : t("kyc_doc_hint")}</div>
              </div>
            </button>
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 18 }} disabled={!doc} onClick={submit}>{t("kyc_submit")}</button>
          </div>
        )}

        {step === 3 && (
          <div className="fade-up" style={{ marginTop: 30, textAlign: "center" }}>
            <Seal size={96} label="PENDING" sub="KYC" id="UAE·23" date="review" />
            <div style={{ marginTop: 16 }}><Chip state="pending" label="UNDER_REVIEW" /></div>
            <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, margin: "16px auto 0", maxWidth: 280 }}>{t("kyc_pending")}</p>
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 24 }} onClick={() => nav("/")}>
              {t("nav_gallery")} <Icon name="arrow-right" size={18} />
            </button>
          </div>
        )}
      </div>
    </ScreenShell>
  );
}
