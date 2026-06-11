/* ============================================================
   Admin (House Operations) DTOs. These mirror the shapes the gateway's
   `/apis/admin/*` endpoints will expose (CLAUDE.md §6). Until those land on the
   backend, the admin panel is served entirely by the in-memory mock
   (see mock/adminDb + mock/adminHandlers). Money is int64 USDC cents; bid
   credits are whole units; enums are MONOSPACE_UPPERCASE.
   ============================================================ */
import type {
  AType, DocType, EscrowState, KycStatus, Role, Tier, VaultObjectState,
} from "@/types";

// Auctions span both engines; admin sees a unified state union.
export type AdminAuctionState =
  | "DRAFT" | "SCHEDULED" | "OPEN" | "CLOSING" | "HAMMER"
  | "RESOLVED" | "SETTLING" | "COMPLETED" | "ABORTED" | "CANCELLED";

export type AccountStatus = "ACTIVE" | "SUSPENDED";
export type ReviewStatus = "PENDING" | "APPROVED" | "REJECTED";
export type CertStatus = "APPRAISING" | "CERTIFIED";
export type DisputeRuling = "REFUND_BUYER" | "RELEASE_SELLER" | "SPLIT";
export type InviteAdminStatus = "ACTIVE" | "REDEEMED" | "REVOKED" | "FLAGGED";

export interface AdminStats {
  activeInvites: number;
  flaggedInvites: number;
  pendingKyc: number;
  members: number;
  lotsThisWeek: number;
  supplyCap: number;
  openAuctions: number;
  escrowLockedCents: number;
}

export interface AdminInvite {
  code: string;
  issuedBy: string;
  uses: number;
  maxUses: number;
  status: InviteAdminStatus;
  chain: string;
}

export interface AdminAccount {
  id: string;
  handle: string;
  tier: Tier;
  kycStatus: KycStatus;
  status: AccountStatus;
  walletUsdcCents: number;
  vaultCreditCents: number;
  bidCredits: number;
  invitedBy: string;
  joinedAt: string;
  /** Elevated functional roles (USER is implicit). Admins grant INSPECTOR/ADMIN. */
  roles?: Role[];
}

export interface AdminKycReview {
  id: string;
  accountId: string;
  handle: string;
  docType: DocType;
  status: ReviewStatus;
  issuedBy: string;
}

export interface AdminCertReview {
  lotId: string;
  object: string;
  maison: string;
  valueCents: number;
  status: CertStatus;
}

export interface AdminAuction {
  id: string;
  lotId: string;
  title: string;
  maison: string;
  atype: AType;
  state: AdminAuctionState;
  priceCents: number; // current price (Dutch) or floor (passive)
  participants: number;
  closesAt?: string;
}

export interface CreateAuctionReq {
  lotId: string;
  atype: AType;
  floorCents: number;
  durationDays?: number;
}

export interface AdminVaultObject {
  id: string;
  ownerHandle: string;
  title: string;
  maison: string;
  valueCents: number;
  state: VaultObjectState;
}

export interface AdminEscrowRow {
  id: string;
  lot: string;
  memberHandle: string;
  amountCents: number;
  premiumCents: number;
  state: EscrowState;
}
