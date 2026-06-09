import { AnimatePresence, motion } from "framer-motion";
import { Route, Routes, useLocation } from "react-router-dom";
import { BottomNav, showBottomNav } from "./BottomNav";
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
import { InvitePage } from "@/pages/InvitePage";
import { KycPage } from "@/pages/KycPage";

// App-like page transition: subtle slide + fade, swapped on path change.
const variants = {
  initial: { opacity: 0, x: 18 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: -12 },
};

export function AppRouter() {
  const location = useLocation();
  const withNav = showBottomNav(location.pathname);

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
              <Route path="/" element={<GalleryPage />} />
              <Route path="/lot/:id" element={<LotPage />} />
              <Route path="/auction/:id" element={<AuctionPage />} />
              <Route path="/passive/:id" element={<PassivePage />} />
              <Route path="/bidstore" element={<BidStorePage />} />
              <Route path="/escrow/:id" element={<EscrowPage />} />
              <Route path="/vault" element={<VaultPage />} />
              <Route path="/membership" element={<MembershipPage />} />
              <Route path="/account" element={<AccountPage />} />
              <Route path="/invite" element={<InvitePage />} />
              <Route path="/kyc" element={<KycPage />} />
              <Route path="*" element={<GalleryPage />} />
            </Routes>
          </motion.div>
        </AnimatePresence>
      </div>
      {withNav && <BottomNav />}
    </>
  );
}
