import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useSession } from "@/hooks/useSession";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";

// The signed-in user's avatar + @username, shown in the application's top corner.
// Click → the merged Profile (account + membership). Guests see "Sign in".
export function UserChip({ compact = false }: { compact?: boolean }) {
  const { t } = useI18n();
  const nav = useNavigate();
  const { account, isGuest } = useSession();

  if (isGuest) {
    return (
      <button onClick={() => nav("/login")} className="user-chip" style={chipStyle}>
        <Icon name="user" size={15} />
        <span style={{ fontWeight: 600 }}>{t("auth_login")}</span>
      </button>
    );
  }

  return (
    <button onClick={() => nav("/profile")} className="user-chip" style={chipStyle} aria-label={t("nav_profile")} title={`@${account?.handle}`}>
      <Avatar account={account} size={24} />
      {!compact && <span className="mono" style={{ fontWeight: 600, maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} dir="ltr">@{account?.handle}</span>}
    </button>
  );
}

const chipStyle: React.CSSProperties = {
  display: "inline-flex", alignItems: "center", gap: 8, cursor: "pointer",
  background: "var(--bg-1)", border: "1px solid var(--gold-line)", borderRadius: "var(--r-pill)",
  padding: "5px 12px 5px 6px", color: "var(--gold-pale)", fontSize: 13,
};
