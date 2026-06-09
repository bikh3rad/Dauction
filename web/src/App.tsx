import { AppRouter } from "@/navigation/AppRouter";
import { DesktopShell } from "@/navigation/DesktopShell";
import { useIsDesktop } from "@/hooks/useMediaQuery";

// Two native shells over one set of screens: a phone-shaped column on small
// viewports, and a full-bleed top-nav layout on desktop that reuses the same
// screens inside a centered frame. The breakpoint lives in useMediaQuery.
export function App() {
  const isDesktop = useIsDesktop();

  if (isDesktop) {
    return <DesktopShell />;
  }

  return (
    <div className="app-stage">
      <div className="app-shell">
        <AppRouter />
      </div>
    </div>
  );
}
