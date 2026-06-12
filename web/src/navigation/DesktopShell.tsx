import { useLocation, useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "@/components/ui/Icon";
import { UserChip } from "@/components/ui/UserChip";
import { AppRouter } from "./AppRouter";

// Desktop nav mirrors the mobile bottom-nav tabs, but lives in a full-bleed
// top bar. The routed screens themselves are reused unchanged inside a
// centered phone-width frame (see desktop.css).
// Just the gallery in the top nav — the Vault and Profile live in the user menu.
const TABS = [
  { path: "/", icon: "layers", key: "nav_gallery", match: ["/", "/lot", "/auction", "/passive", "/escrow", "/bidstore"] },
];

export function DesktopShell() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { pathname } = useLocation();

  const isActive = (tab: (typeof TABS)[number]) =>
    tab.match.some((m) => (m === "/" ? pathname === "/" : pathname.startsWith(m)));

  // Browse screens (gallery, vault) fan out full-width; forms and detail
  // flows read better in a contained column.
  const wide = pathname === "/" || pathname.startsWith("/vault");

  return (
    <div className="desk-stage">
      <header className="desk-nav">
        <button className="brand" onClick={() => nav("/")}>
          <Icon name="gavel" size={20} stroke={2} />
          <strong>{t("brand")}</strong>
        </button>
        <div className="nav-links">
          {TABS.map((tab) => (
            <button
              key={tab.path}
              className={isActive(tab) ? "on" : ""}
              onClick={() => nav(tab.path)}
            >
              {t(tab.key)}
            </button>
          ))}
        </div>
        <div style={{ flex: 1 }} />
        <UserChip />
      </header>
      <main className="desk-main">
        <div className={`desk-center ${wide ? "wide" : "readable"}`}>
          <div className="frame">
            <AppRouter showNav={false} />
          </div>
        </div>
      </main>
    </div>
  );
}
