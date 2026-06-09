import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "./Icon";

// Loading skeleton for a screen body.
export function LoadingScreen({ rows = 3 }: { rows?: number }) {
  return (
    <div style={{ padding: "18px 20px", display: "flex", flexDirection: "column", gap: 14 }}>
      <div className="skel" style={{ height: 180 }} />
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skel" style={{ height: 64 }} />
      ))}
    </div>
  );
}

// Inline error with retry.
export function ErrorState({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  const { t } = useI18n();
  return (
    <div style={{ padding: "40px 24px", textAlign: "center" }}>
      <div style={{ color: "var(--st-bad)", display: "inline-flex", marginBottom: 12 }}>
        <Icon name="alert" size={32} />
      </div>
      <p className="muted" style={{ fontSize: 14, lineHeight: 1.6, margin: "0 0 18px" }}>
        {message || "Something went wrong."}
      </p>
      {onRetry && (
        <button className="btn btn-ghost" onClick={onRetry}>
          <Icon name="refresh" size={16} /> {t("common_continue")}
        </button>
      )}
    </div>
  );
}

// Empty placeholder.
export function EmptyState({ label }: { label: string }) {
  return (
    <div style={{ padding: "60px 24px", textAlign: "center" }}>
      <div style={{ color: "var(--fg-faint)", display: "inline-flex", marginBottom: 12 }}>
        <Icon name="package" size={30} />
      </div>
      <p className="muted" style={{ fontSize: 14, margin: 0 }}>{label}</p>
    </div>
  );
}
