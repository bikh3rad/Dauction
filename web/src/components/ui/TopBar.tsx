import type { ReactNode } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "./Icon";
import { UserChip } from "./UserChip";
import { iconBtnStyle } from "./Primitives";

// Sticky frosted header. A back chevron (when onBack given) else the house crown.
export function TopBar({
  title,
  kicker,
  onBack,
  right,
}: {
  title: ReactNode;
  kicker?: ReactNode;
  onBack?: () => void;
  right?: ReactNode;
}) {
  const { t, dir } = useI18n();
  return (
    <div
      className="safe-top"
      style={{
        position: "relative", zIndex: 8, background: "rgba(20,12,16,0.86)",
        backdropFilter: "blur(14px)", WebkitBackdropFilter: "blur(14px)",
        borderBottom: "1px solid var(--line)", padding: "16px 16px 12px",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        {onBack ? (
          <button className="iconbtn" onClick={onBack} aria-label={t("common_back")} style={iconBtnStyle}>
            <Icon name={dir === "rtl" ? "chevron-right" : "chevron-left"} size={20} />
          </button>
        ) : (
          <div style={{ width: 38, height: 38, display: "flex", alignItems: "center", justifyContent: "center", color: "var(--gold)" }}>
            <Icon name="crown" size={20} />
          </div>
        )}
        <div style={{ flex: 1, minWidth: 0 }}>
          {kicker && <div className="mono up" style={{ fontSize: 9, color: "var(--gold)", marginBottom: 2 }}>{kicker}</div>}
          <div className="serif" style={{ fontSize: 19, lineHeight: 1.1, color: "var(--fg)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{title}</div>
        </div>
        {right}
        {/* the user's @username lives in the top corner (hidden on desktop, where
            the desk-nav carries it instead) */}
        <span className="tb-user"><UserChip compact /></span>
      </div>
    </div>
  );
}
