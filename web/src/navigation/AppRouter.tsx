import { AnimatePresence, motion } from "framer-motion";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { BottomNav, showBottomNav } from "./BottomNav";
import { hasSession } from "@/auth/session";
import { AuthPage } from "@/pages/AuthPage";
import { MockBanner } from "@/components/ui/MockBanner";
import { GalleryPage } from "@/pages/GalleryPage";
import { LotPage } from "@/pages/LotPage";
import { AuctionPage } from "@/pages/AuctionPage";
import { PassivePage } from "@/pages/PassivePage";
import { BidStorePage } from "@/pages/BidStorePage";
import { EscrowPage } from "@/pages/EscrowPage";
import { VaultPage } from "@/pages/VaultPage";
import { MembershipPage } from "@/pages/MembershipPage";
import { AccountPage } from "@/pages/AccountPage";
import { KycPage } from "@/pages/KycPage";
import { AdminPage } from "@/pages/admin/AdminPage";

// App-like page transition: subtle slide + fade, swapped on path change.
const variants = {
  initial: { opacity: 0, x: 18 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: -12 },
};

export function AppRouter({ showNav = true }: { showNav?: boolean } = {}) {
  const location = useLocation();
  // First run: no session at all -> land on the sign-in page (mobile OTP / OAuth).
  // Once the user signs in OR chooses "browse as guest", a session exists and the
  // app is reachable normally.
  if (!hasSession() && location.pathname !== "/login") {
    return <Navigate to="/login" replace />;
  }
  // On desktop the top nav drives navigation, so the bottom nav is suppressed.
  const withNav = showNav && showBottomNav(location.pathname);

  return (
    <>
      <MockBanner />
      <div className="app-viewport">
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={location.pathname}
            className="screen"
            variants={variants}
            initial="initial"
            animate="animate"
            exit="exit"
            transition={{ duration: 0.22, ease: [0.2, 0.65, 0.2, 1] }}
          >
            <Routes location={location}>
              <Route path="/login" element={<AuthPage />} />
              <Route path="/" element={<GalleryPage />} />
              <Route path="/lot/:id" element={<LotPage />} />
              <Route path="/auction/:id" element={<AuctionPage />} />
              <Route path="/passive/:id" element={<PassivePage />} />
              <Route path="/bidstore" element={<BidStorePage />} />
              <Route path="/escrow/:id" element={<EscrowPage />} />
              <Route path="/vault" element={<VaultPage />} />
              <Route path="/membership" element={<MembershipPage />} />
              <Route path="/account" element={<AccountPage />} />
              {/* invite system removed — any lingering link lands on sign-in */}
              <Route path="/invite" element={<Navigate to="/login" replace />} />
              <Route path="/kyc" element={<KycPage />} />
              <Route path="/admin" element={<AdminPage />} />
              <Route path="*" element={<GalleryPage />} />
            </Routes>
          </motion.div>
        </AnimatePresence>
      </div>
      {withNav && <BottomNav />}
    </>
  );
}
