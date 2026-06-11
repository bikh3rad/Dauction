/* ============================================================
   In-memory House-Operations store. Seeded from the prototype's admin data,
   reshaped to backend DTOs (USDC cents, MONOSPACE_UPPERCASE enums). Stateful:
   admin mutations (revoke, approve, open/disable auction, suspend, rule a
   dispute…) mutate these arrays so the panel behaves like a live console while
   offline. Mirrors the style of mock/db.ts.
   ============================================================ */
import type {
  AdminAccount, AdminAuction, AdminCertReview, AdminEscrowRow, AdminInvite,
  AdminKycReview, AdminVaultObject,
} from "@/types/admin";

const c = (dollars: number) => Math.round(dollars * 100); // dollars → USDC cents

export const invites: AdminInvite[] = [
  { code: "LUX-7F2A-9KQ", issuedBy: "0x11 · Maison", uses: 0, maxUses: 1, status: "ACTIVE", chain: "Maison → ?" },
  { code: "VELT-3C8-XQ2", issuedBy: "0x7A4E", uses: 1, maxUses: 1, status: "REDEEMED", chain: "0x7A4E → 0x91" },
  { code: "NOIR-55K-A0", issuedBy: "0x2D · VIP", uses: 0, maxUses: 1, status: "ACTIVE", chain: "VIP → ?" },
  { code: "GHOST-99-ZZ", issuedBy: "unknown", uses: 3, maxUses: 1, status: "FLAGGED", chain: "⚠ over-use" },
  { code: "MAISON-04", issuedBy: "House", uses: 0, maxUses: 1, status: "ACTIVE", chain: "House → ?" },
];

export const accounts: AdminAccount[] = [
  { id: "0x7A4E", handle: "@aurelia.dxb", tier: "MEMBER", kycStatus: "APPROVED", status: "ACTIVE", walletUsdcCents: c(212400), vaultCreditCents: c(34850), bidCredits: 18, invitedBy: "Maison · 0x11", joinedAt: "2025-11-02", roles: ["INSPECTOR"] },
  { id: "0x91", handle: "@noor.auh", tier: "MEMBER", kycStatus: "PENDING", status: "ACTIVE", walletUsdcCents: c(48000), vaultCreditCents: 0, bidCredits: 50, invitedBy: "0x7A4E", joinedAt: "2026-03-14", roles: [] },
  { id: "0x2D", handle: "@khalid.vip", tier: "VIP", kycStatus: "APPROVED", status: "ACTIVE", walletUsdcCents: c(980000), vaultCreditCents: c(120000), bidCredits: 100, invitedBy: "House", joinedAt: "2025-09-01", roles: [] },
  { id: "0x4C", handle: "@sterling.ldn", tier: "MEMBER", kycStatus: "PENDING", status: "ACTIVE", walletUsdcCents: c(15000), vaultCreditCents: 0, bidCredits: 20, invitedBy: "0x2D", joinedAt: "2026-05-02", roles: [] },
  { id: "0xF7", handle: "@dana.doh", tier: "MEMBER", kycStatus: "APPROVED", status: "SUSPENDED", walletUsdcCents: c(3200), vaultCreditCents: 0, bidCredits: 0, invitedBy: "House", joinedAt: "2026-01-20", roles: [] },
];

export const kycQueue: AdminKycReview[] = [
  { id: "kyc-91", accountId: "0x91", handle: "@noor.auh", docType: "NATIONAL_ID", status: "PENDING", issuedBy: "0x7A4E" },
  { id: "kyc-4c", accountId: "0x4C", handle: "@sterling.ldn", docType: "PASSPORT", status: "PENDING", issuedBy: "0x2D" },
  { id: "kyc-f7", accountId: "0xF7", handle: "@dana.doh", docType: "NATIONAL_ID", status: "APPROVED", issuedBy: "House" },
];

export const certQueue: AdminCertReview[] = [
  { lotId: "lot-08", object: "Birkin 25 Himalaya", maison: "Hermès", valueCents: c(285000), status: "APPRAISING" },
  { lotId: "lot-09", object: "RM 11-03 Flyback", maison: "Richard Mille", valueCents: c(198000), status: "APPRAISING" },
  { lotId: "v2", object: "Classic Flap Medium", maison: "Chanel", valueCents: c(12400), status: "APPRAISING" },
  { lotId: "lot-07", object: "Nautilus 5711/1A", maison: "Patek Philippe", valueCents: c(620000), status: "CERTIFIED" },
];

// Mirrors the canonical catalog (mock/db.ts lot-07…lot-14): six live/scheduled
// Dutch lots plus the two timed passive auctions, in a mix of states so the
// open / schedule / disable controls all have something to act on.
export const auctions: AdminAuction[] = [
  { id: "lot-07", lotId: "lot-07", title: "Nautilus 5711/1A — Tiffany Blue Dial", maison: "Patek Philippe", atype: "DUTCH", state: "OPEN", priceCents: c(1085600), participants: 14, closesAt: undefined },
  { id: "lot-08", lotId: "lot-08", title: "Birkin 25 Himalaya — Niloticus", maison: "Hermès", atype: "DUTCH", state: "SCHEDULED", priceCents: c(520000), participants: 9 },
  { id: "lot-09", lotId: "lot-09", title: "RM 11-03 Flyback Chronograph", maison: "Richard Mille", atype: "DUTCH", state: "SCHEDULED", priceCents: c(395000), participants: 7 },
  { id: "lot-10", lotId: "lot-10", title: "Pumpkin (Yellow) — ed. 47/120", maison: "Yayoi Kusama", atype: "DUTCH", state: "SCHEDULED", priceCents: c(185000), participants: 5 },
  { id: "lot-11", lotId: "lot-11", title: "Air Jordan 1 High — 1 of 8,500", maison: "Nike × Dior", atype: "DUTCH", state: "SCHEDULED", priceCents: c(27000), participants: 11 },
  { id: "lot-12", lotId: "lot-12", title: "No. 1 Imperial Majesty — Baccarat", maison: "Clive Christian", atype: "DUTCH", state: "DRAFT", priceCents: c(96000), participants: 4 },
  { id: "lot-13", lotId: "lot-13", title: "Crimson Atrium — Oil on Linen", maison: "Estate Commission", atype: "VICKREY", state: "OPEN", priceCents: c(54000), participants: 41, closesAt: "2026-06-15T18:00:00Z" },
  { id: "lot-14", lotId: "lot-14", title: "Kelly Sellier 25 — Rouge Casaque", maison: "Hermès", atype: "UNIQBID", state: "CLOSING", priceCents: c(38000), participants: 184, closesAt: "2026-06-12T12:00:00Z" },
];

export const memberVault: AdminVaultObject[] = [
  { id: "v1", ownerHandle: "@aurelia.dxb", title: "Daytona 116500LN — Panda Dial", maison: "Rolex", valueCents: c(38500), state: "IN_VAULT" },
  { id: "v2", ownerHandle: "@aurelia.dxb", title: "Classic Flap Medium — Black Caviar", maison: "Chanel", valueCents: c(12400), state: "APPRAISING" },
  { id: "v3", ownerHandle: "@khalid.vip", title: "Flowers (1970) — Screenprint", maison: "Andy Warhol", valueCents: c(96000), state: "IN_AUCTION" },
  { id: "v4", ownerHandle: "@khalid.vip", title: "Love Bracelet — Pavé Diamond, Gold", maison: "Cartier", valueCents: c(28900), state: "SOLD" },
  { id: "v5", ownerHandle: "@noor.auh", title: "Gilded Horizon — Oil, home collection", maison: "Private Collection", valueCents: c(46000), state: "IN_VAULT" },
];

export const escrow: AdminEscrowRow[] = [
  { id: "esc-07", lot: "Nautilus 5711/1A", memberHandle: "0x7A4E", amountCents: c(962000), premiumCents: c(96200), state: "HELD" },
  { id: "esc-05", lot: "Flowers (1970)", memberHandle: "0x91", amountCents: c(96000), premiumCents: c(11520), state: "FULL_LOCKED" },
  { id: "esc-03", lot: "Love Bracelet", memberHandle: "0x4C", amountCents: c(28900), premiumCents: c(3757), state: "RELEASED" },
  { id: "esc-02", lot: "Air Jordan 1 × Dior", memberHandle: "0xF7", amountCents: c(11000), premiumCents: c(1430), state: "DISPUTED" },
];
