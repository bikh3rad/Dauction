import { useLocation, useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "@/components/ui/Icon";

// The nav is just the gallery — the Vault and Profile live in the top-corner
// user menu (UserChip), so the bottom of the screen stays focused on browsing.
const TABS = [
  { path: "/", icon: "layers", key: "nav_gallery", match: ["/", "/lot", "/auction", "/passive", "/escrow", "/bidstore"] },
];

export function BottomNav() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { pathname } = useLocation();

  const isActive = (tab: (typeof TABS)[number]) =>
    tab.match.some((m) => (m === "/" ? pathname === "/" : pathname.startsWith(m)));

  return (
    <nav className="bottom-nav">
      {TABS.map((tab) => {
        const on = isActive(tab);
        return (
          <button key={tab.path} className={on ? "on" : ""} onClick={() => nav(tab.path)}>
            <Icon name={tab.icon} size={21} stroke={on ? 2.2 : 1.8} />
            <span className="nav-label" style={{ fontWeight: on ? 600 : 500 }}>{t(tab.key)}</span>
          </button>
        );
      })}
    </nav>
  );
}

// Whether the bottom nav should show for a given path (tab roots only).
export function showBottomNav(pathname: string): boolean {
  return ["/", "/vault", "/profile"].includes(pathname);
}
