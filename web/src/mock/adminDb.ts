/* ============================================================
   In-memory House-Operations store. Seeded from the prototype's admin data,
   reshaped to backend DTOs (USDC cents, MONOSPACE_UPPERCASE enums). Stateful:
   admin mutations (revoke, approve, open/disable auction, suspend, rule a
   dispute…) mutate these arrays so the panel behaves like a live console while
   offline. Mirrors the style of mock/db.ts.

   The `auctions` rows are DERIVED from the shared catalog (mock/db.ts) at load,
   so the admin console and the buyer-facing gallery start from one source of
   truth. Admin edits (floor / high / bid-cost) write back through mock/db.ts —
   see adminHandlers.updateAuction — so an edit here changes the live auction.
   ============================================================ */
import * as cat from "./db";
import { maisonOf } from "@/lib/enrich";
import type {
  AdminAccount, AdminAuction, AdminCertReview, AdminEscrowRow,
  AdminKycReview, AdminVaultObject,
} from "@/types/admin";

const c = (dollars: number) => Math.round(dollars * 100); // dollars → USDC cents

export const accounts: AdminAccount[] = [
  { id: "0x7A4E", handle: "@aurelia.dxb", tier: "MEMBER", kycStatus: "APPROVED", status: "ACTIVE", walletUsdcCents: c(212400), vaultCreditCents: c(34850), bidCredits: 18, joinedAt: "2025-11-02", roles: ["INSPECTOR"] },
  { id: "0x91", handle: "@noor.auh", tier: "MEMBER", kycStatus: "PENDING", status: "ACTIVE", walletUsdcCents: c(48000), vaultCreditCents: 0, bidCredits: 50, joinedAt: "2026-03-14", roles: [] },
  { id: "0x2D", handle: "@khalid.vip", tier: "VIP", kycStatus: "APPROVED", status: "ACTIVE", walletUsdcCents: c(980000), vaultCreditCents: c(120000), bidCredits: 100, joinedAt: "2025-09-01", roles: [] },
  { id: "0x4C", handle: "@sterling.ldn", tier: "MEMBER", kycStatus: "PENDING", status: "ACTIVE", walletUsdcCents: c(15000), vaultCreditCents: 0, bidCredits: 20, joinedAt: "2026-05-02", roles: [] },
  { id: "0xF7", handle: "@dana.doh", tier: "MEMBER", kycStatus: "APPROVED", status: "SUSPENDED", walletUsdcCents: c(3200), vaultCreditCents: 0, bidCredits: 0, joinedAt: "2026-01-20", roles: [] },
];

export const kycQueue: AdminKycReview[] = [
  { id: "kyc-91", accountId: "0x91", handle: "@noor.auh", docType: "NATIONAL_ID", status: "PENDING", issuedBy: "0x7A4E" },
  { id: "kyc-4c", accountId: "0x4C", handle: "@sterling.ldn", docType: "PASSPORT", status: "PENDING", issuedBy: "0x2D" },
  { id: "kyc-f7", accountId: "0xF7", handle: "@dana.doh", docType: "NATIONAL_ID", status: "APPROVED", issuedBy: "House" },
];

// ---- auctions: derived from the shared catalog (one source of truth) ----
const bodyOf = (title: string) => title.split(/\s+[—–-]\s+/).slice(1).join(" — ") || title;
const partsOf = (id: string) => { let h = 0; for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0; return 4 + (h % 18); };

function buildAuctions(): AdminAuction[] {
  return cat.lots.map((l) => {
    const passive = cat.passiveAuction(l.id);
    const dutch = cat.dutchAuction(l.id);
    const state: AdminAuction["state"] = passive ? "OPEN" : dutch?.state === "OPEN" ? "OPEN" : "SCHEDULED";
    return {
      id: l.id, lotId: l.id,
      title: bodyOf(l.title),
      maison: maisonOf(l.title),
      atype: l.atype,
      state,
      priceCents: passive ? passive.reserveCents : dutch?.currentPriceCents ?? l.reserveCents,
      highCents: l.appraisedValueCents,
      participants: passive?.participantCount ?? partsOf(l.id),
      closesAt: passive?.closesAt,
      bidCostCredits: passive?.bidCostCredits,
    };
  });
}
export const auctions: AdminAuction[] = buildAuctions();

// Certification queue mirrors the catalog's inspector gate.
export const certQueue: AdminCertReview[] = [
  { lotId: "lot-08", object: "Birkin 30 — Gold Togo", maison: "Hermès", valueCents: c(48000), status: "APPRAISING" },
  { lotId: "lot-11", object: "Diamond Solitaire 3.01ct", maison: "Tiffany & Co.", valueCents: c(64000), status: "APPRAISING" },
  { lotId: "v2", object: "Capucines MM — Taurillon", maison: "Louis Vuitton", valueCents: c(12400), status: "APPRAISING" },
  { lotId: "lot-07", object: "Royal Oak ‘Jumbo’ 15202ST", maison: "Audemars Piguet", valueCents: c(92000), status: "CERTIFIED" },
];

export const memberVault: AdminVaultObject[] = [
  { id: "v1", ownerHandle: "@aurelia.dxb", title: "Daytona 116500LN — Panda Dial", maison: "Rolex", valueCents: c(38500), state: "IN_VAULT" },
  { id: "v2", ownerHandle: "@aurelia.dxb", title: "Capucines MM — Taurillon", maison: "Louis Vuitton", valueCents: c(12400), state: "APPRAISING" },
  { id: "v3", ownerHandle: "@khalid.vip", title: "Flowers (1970) — Screenprint", maison: "Andy Warhol", valueCents: c(96000), state: "IN_AUCTION" },
  { id: "v4", ownerHandle: "@khalid.vip", title: "Love Bracelet — Pavé Diamond, Gold", maison: "Cartier", valueCents: c(28900), state: "SOLD" },
  { id: "v5", ownerHandle: "@noor.auh", title: "Gilded Horizon — Oil", maison: "Private Collection", valueCents: c(46000), state: "IN_VAULT" },
];

export const escrow: AdminEscrowRow[] = [
  { id: "esc-07", lot: "Royal Oak ‘Jumbo’", memberHandle: "0x7A4E", amountCents: c(78000), premiumCents: c(7800), state: "HELD" },
  { id: "esc-05", lot: "Flowers (1970)", memberHandle: "0x91", amountCents: c(96000), premiumCents: c(11520), state: "FULL_LOCKED" },
  { id: "esc-03", lot: "Love Bracelet", memberHandle: "0x4C", amountCents: c(28900), premiumCents: c(3757), state: "RELEASED" },
  { id: "esc-02", lot: "Air Jordan 1 ‘Chicago’", memberHandle: "0xF7", amountCents: c(8500), premiumCents: c(1105), state: "DISPUTED" },
];
