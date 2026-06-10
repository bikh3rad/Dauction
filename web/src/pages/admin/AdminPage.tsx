import { useNavigate, useSearchParams } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "@/components/ui/Icon";
import { Overview, Auctions, Accounts, Memberships, Vaults, Invites, Escrow } from "./sections";

// House Operations console. A full-viewport desktop shell (status bar · 240px
// sidebar · scrolling content) that overlays whichever buyer shell is mounted —
// reached via /admin. Sections mirror the prototype's admin.jsx, expanded with
// auction control, account management, memberships and member-vault views.

type SectionKey = "overview" | "auctions" | "accounts" | "memberships" | "vaults" | "invites" | "escrow";
const SECTIONS: SectionKey[] = ["overview", "auctions", "accounts", "memberships", "vaults", "invites", "escrow"];

export function AdminPage() {
  const { t } = useI18n();
  const nav = useNavigate();
  // Section lives in the URL (?s=auctions) so it is deep-linkable and survives reload.
  const [params, setParams] = useSearchParams();
  const sec = (SECTIONS.includes(params.get("s") as SectionKey) ? params.get("s") : "overview") as SectionKey;
  const setSec = (s: SectionKey) => setParams(s === "overview" ? {} : { s }, { replace: true });

  const navItems: { k: SectionKey; icon: string; label: string }[] = [
    { k: "overview", icon: "grid", label: t("adm_overview") },
    { k: "auctions", icon: "gavel", label: t("adm_auctions") },
    { k: "accounts", icon: "users", label: t("adm_accounts") },
    { k: "memberships", icon: "shield", label: t("adm_memberships") },
    { k: "vaults", icon: "package", label: t("adm_vaults") },
    { k: "invites", icon: "hash", label: t("adm_invites") },
    { k: "escrow", icon: "scale", label: t("adm_escrow") },
  ];

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", flexDirection: "column", background: "var(--bg-void)", color: "var(--fg)" }}>
      {/* status bar */}
      <div style={{ height: 56, flexShrink: 0, borderBottom: "1px solid var(--gold-line)", display: "flex", alignItems: "center", padding: "0 20px", gap: 14, background: "rgba(20,12,16,0.7)", backdropFilter: "blur(12px)" }}>
        <span style={{ color: "var(--gold)" }}><Icon name="crown" size={20} /></span>
        <span className="serif" style={{ fontSize: 18, color: "var(--gold-pale)", letterSpacing: "0.04em" }}>{t("brand")}</span>
        <span className="mono up" style={{ fontSize: 9.5, color: "var(--fg-faint)", border: "1px solid var(--line)", padding: "3px 8px", borderRadius: "var(--r-1)" }}>HOUSE OPS</span>
        <div style={{ flex: 1 }} />
        <span className="chip" data-st="live"><span className="dot" />WEEK 23 · LIVE</span>
        <span className="mono" style={{ fontSize: 12, color: "var(--fg-muted)" }}>registrar · 0x11</span>
      </div>

      <div style={{ flex: 1, display: "flex", minHeight: 0 }}>
        {/* sidebar */}
        <div style={{ width: 240, flexShrink: 0, borderInlineEnd: "1px solid var(--line)", padding: "16px 12px", background: "var(--bg-0)", display: "flex", flexDirection: "column" }}>
          {navItems.map((n) => {
            const on = sec === n.k;
            return (
              <button key={n.k} onClick={() => setSec(n.k)} style={{ width: "100%", textAlign: "start", display: "flex", alignItems: "center", gap: 11, padding: "11px 12px", borderRadius: "var(--r-2)", marginBottom: 4, cursor: "pointer", border: "none", background: on ? "var(--bg-2)" : "transparent", color: on ? "var(--gold-pale)" : "var(--fg-muted)", borderInlineStart: "2px solid", borderInlineStartColor: on ? "var(--gold)" : "transparent", fontSize: 14, fontFamily: "var(--sans)", fontWeight: on ? 600 : 500 }}>
                <Icon name={n.icon} size={18} stroke={on ? 2.1 : 1.8} />
                {n.label}
              </button>
            );
          })}
          <div style={{ flex: 1 }} />
          <button className="btn btn-ghost" style={{ width: "100%", fontSize: 13, padding: "10px" }} onClick={() => nav("/")}>
            <Icon name="arrow-left" size={15} /> {t("adm_exit")}
          </button>
        </div>

        {/* content */}
        <div style={{ flex: 1, overflow: "auto", padding: "28px 32px" }}>
          {sec === "overview" && <Overview go={(s) => setSec(s as SectionKey)} />}
          {sec === "auctions" && <Auctions />}
          {sec === "accounts" && <Accounts />}
          {sec === "memberships" && <Memberships />}
          {sec === "vaults" && <Vaults />}
          {sec === "invites" && <Invites />}
          {sec === "escrow" && <Escrow />}
        </div>
      </div>
    </div>
  );
}
