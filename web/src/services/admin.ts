// House-Operations service facade.
//
// The backend `/apis/admin/*` routes (CLAUDE.md §6) are not wired yet, so —
// unlike the buyer services — this facade is backed directly by the in-memory
// mock rather than `withFallback`. (withFallback only falls back on network/5xx;
// a 404 from an unrouted admin path is a 4xx and would surface as an error.)
// When the admin endpoints land, swap each body for the withFallback(get/post)
// form used in services/index.ts — the signatures here already match.

import * as mock from "@/mock/adminHandlers";
import type {
  AccountStatus, AdminAccount, AdminAuction, AdminCertReview, AdminEscrowRow,
  AdminKycReview, AdminStats, AdminVaultObject, CreateAuctionReq,
  DisputeRuling,
} from "@/types/admin";
import type { Role, Tier } from "@/types";

const ok = <T>(v: T): Promise<T> => Promise.resolve(v);

export const admin = {
  stats: () => ok<AdminStats>(mock.stats()),

  accounts: () => ok<AdminAccount[]>(mock.listAccounts()),
  setAccountStatus: (id: string, status: AccountStatus) =>
    ok<AdminAccount>(mock.setAccountStatus(id, status)),
  setAccountTier: (id: string, tier: Tier) => ok<AdminAccount>(mock.setAccountTier(id, tier)),
  setAccountRole: (id: string, role: Role, grant: boolean) =>
    ok<AdminAccount>(mock.setAccountRole(id, role, grant)),

  kyc: () => ok<AdminKycReview[]>(mock.listKyc()),
  decideKyc: (id: string, approve: boolean) => ok<AdminKycReview>(mock.decideKyc(id, approve)),

  cert: () => ok<AdminCertReview[]>(mock.listCert()),
  certify: (lotId: string) => ok<AdminCertReview>(mock.certify(lotId)),

  auctions: () => ok<AdminAuction[]>(mock.listAuctions()),
  createAuction: (req: CreateAuctionReq) => ok<AdminAuction>(mock.createAuction(req)),
  setAuctionState: (id: string, state: AdminAuction["state"]) =>
    ok<AdminAuction>(mock.setAuctionState(id, state)),
  updateAuction: (id: string, patch: { bidCostCredits?: number; floorCents?: number; appraisedCents?: number; title?: string }) =>
    ok<AdminAuction>(mock.updateAuction(id, patch)),

  vault: () => ok<AdminVaultObject[]>(mock.listVault()),

  escrow: () => ok<AdminEscrowRow[]>(mock.listEscrow()),
  holdRelease: (id: string) => ok<AdminEscrowRow>(mock.holdRelease(id)),
  ruleDispute: (id: string, ruling: DisputeRuling) =>
    ok<AdminEscrowRow>(mock.ruleDispute(id, ruling)),
};
