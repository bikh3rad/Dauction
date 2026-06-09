import { AppRouter } from "@/navigation/AppRouter";

// The whole experience lives in a phone-shaped app shell: full-bleed on phones,
// a centered app-width column on larger screens — keeping the mobile-native
// interaction model everywhere.
export function App() {
  return (
    <div className="app-stage">
      <div className="app-shell">
        <AppRouter />
      </div>
    </div>
  );
}
