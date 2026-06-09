import { useLocation, useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "@/components/ui/Icon";
import { AppRouter } from "./AppRouter";

// Desktop nav mirrors the mobile bottom-nav tabs, but lives in a full-bleed
// top bar. The routed screens themselves are reused unchanged inside a
// centered phone-width frame (see desktop.css).
const TABS = [
  { path: "/", icon: "layers", key: "nav_gallery", match: ["/", "/lot", "/auction", "/passive", "/escrow", "/bidstore"] },
  { path: "/vault", icon: "package", key: "nav_closet", match: ["/vault"] },
  { path: "/membership", icon: "crown", key: "nav_membership", match: ["/membership"] },
  { path: "/account", icon: "user", key: "nav_account", match: ["/account", "/invite", "/kyc"] },
];

export function DesktopShell() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { pathname } = useLocation();

  const isActive = (tab: (typeof TABS)[number]) =>
    tab.match.some((m) => (m === "/" ? pathname === "/" : pathname.startsWith(m)));

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
      </header>
      <main className="desk-main">
        <div className="desk-center">
          <div className="frame">
            <AppRouter showNav={false} />
          </div>
        </div>
      </main>
    </div>
  );
}
