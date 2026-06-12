import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useSession } from "@/hooks/useSession";
import { useVault, useWallet, useSignOut } from "@/hooks/queries";
import { levelInfo } from "@/lib/membership";
import { usdc } from "@/lib/format";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";

// Top-corner user control. Clicking opens a menu with the wallet snapshot (Vault
// Credit + bid credits) and quick links to the Vault, Profile and sign-out.
// Guests get a sign-in / register menu.
export function UserChip({ compact = false }: { compact?: boolean }) {
  const { t } = useI18n();
  const nav = useNavigate();
  const { account, isGuest, level } = useSession();
  const { data: vault } = useVault();
  const { data: wallet } = useWallet();
  const signOut = useSignOut();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  const go = (path: string) => { setOpen(false); nav(path); };

  return (
    <div ref={ref} style={{ position: "relative" }}>
      <button onClick={() => setOpen((o) => !o)} className="user-chip" style={chipStyle} aria-haspopup="menu" aria-expanded={open} title={isGuest ? t("auth_login") : `@${account?.handle}`}>
        {isGuest ? <Icon name="user" size={15} /> : <Avatar account={account} size={24} />}
        {!compact && (
          <span className="mono" style={{ fontWeight: 600, maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} dir="ltr">
            {isGuest ? t("auth_login") : `@${account?.handle}`}
          </span>
        )}
        <Icon name="chevron-down" size={14} />
      </button>

      {open && (
        <div role="menu" className="fade-up" style={menuStyle}>
          {isGuest ? (
            <>
              <MenuItem icon="arrow-right" label={t("auth_login")} onClick={() => go("/login")} />
              <MenuItem icon="user" label={t("auth_register")} onClick={() => go("/register")} />
            </>
          ) : (
            <>
              {/* identity header */}
              <button onClick={() => go("/profile")} style={headerStyle}>
                <Avatar account={account} size={38} />
                <div style={{ flex: 1, minWidth: 0, textAlign: "start" }}>
                  <div className="mono" style={{ fontSize: 13, color: "var(--gold-pale)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} dir="ltr">@{account?.handle}</div>
                  <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)", marginTop: 2 }}>{levelInfo(level)?.name} · {t("mem_level")} {level}</div>
                </div>
                <Icon name="chevron-right" size={15} />
              </button>

              {/* wallet snapshot */}
              <div style={{ display: "flex", gap: 8, padding: "10px 12px" }}>
                <Stat label={t("clo_credit")} value={vault ? usdc(vault.creditBalanceCents) : "—"} />
                <Stat label={t("bid_wallet")} value={`${wallet?.balanceCredits ?? 0} ${t("credits")}`} />
              </div>

              <div style={divider} />
              <MenuItem icon="package" label={t("clo_title")} onClick={() => go("/vault")} />
              <MenuItem icon="coins" label={t("buy_bids")} onClick={() => go("/bidstore")} />
              <MenuItem icon="user" label={t("nav_profile")} onClick={() => go("/profile")} />
              <div style={divider} />
              <MenuItem icon="arrow-left" label={t("acc_signout")} onClick={async () => { setOpen(false); await signOut.mutateAsync(); nav("/login"); }} />
            </>
          )}
        </div>
      )}
    </div>
  );
}

function MenuItem({ icon, label, onClick }: { icon: string; label: string; onClick: () => void }) {
  return (
    <button role="menuitem" onClick={onClick} style={itemStyle}
      onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-2)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}>
      <Icon name={icon} size={16} /> <span>{label}</span>
    </button>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ flex: 1, border: "1px solid var(--line)", borderRadius: "var(--r-1)", padding: "8px 10px", background: "var(--bg-0)" }}>
      <div className="mono up" style={{ fontSize: 8.5, color: "var(--fg-faint)" }}>{label}</div>
      <div className="mono" style={{ fontSize: 13, color: "var(--gold-pale)", marginTop: 3 }}>{value}</div>
    </div>
  );
}

const chipStyle: React.CSSProperties = {
  display: "inline-flex", alignItems: "center", gap: 7, cursor: "pointer",
  background: "var(--bg-1)", border: "1px solid var(--gold-line)", borderRadius: "var(--r-pill)",
  padding: "5px 10px 5px 6px", color: "var(--gold-pale)", fontSize: 13,
};
const menuStyle: React.CSSProperties = {
  position: "absolute", top: "calc(100% + 8px)", insetInlineEnd: 0, zIndex: 60,
  width: 248, background: "var(--bg-1)", border: "1px solid var(--gold-line)",
  borderRadius: "var(--r-2)", boxShadow: "0 18px 40px rgba(0,0,0,0.5)", padding: 6, overflow: "hidden",
};
const headerStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 10, width: "100%", cursor: "pointer",
  background: "transparent", border: "none", padding: "8px 10px", color: "var(--fg)",
};
const itemStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 11, width: "100%", cursor: "pointer",
  background: "transparent", border: "none", padding: "11px 12px", color: "var(--fg)",
  fontSize: 14, fontFamily: "var(--sans)", borderRadius: "var(--r-1)", textAlign: "start",
};
const divider: React.CSSProperties = { height: 1, background: "var(--line)", margin: "4px 6px" };
