import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "./Icon";

// Shown on an item page once an Inspector has verified it and a certificate of
// authenticity has been issued.
export function CertBadge({ compact = false }: { compact?: boolean }) {
  const { t } = useI18n();
  if (compact) {
    return (
      <span className="chip" data-st="good"><Icon name="award" size={12} /> {t("cert_issued")}</span>
    );
  }
  return (
    <div style={{ display: "inline-flex", alignItems: "center", gap: 9, padding: "8px 14px", borderRadius: "var(--r-pill)", border: "1px solid var(--gold-line)", background: "linear-gradient(100deg,var(--burg-deep),var(--bg-1))", color: "var(--gold-pale)" }}>
      <span style={{ color: "var(--gold)" }}><Icon name="award" size={17} /></span>
      <span style={{ fontSize: 13, fontWeight: 600 }}>{t("cert_issued")}</span>
    </div>
  );
}
