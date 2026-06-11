/* ============================================================
   Dauction — backend DTO types.
   These mirror the Go services' JSON shapes EXACTLY (field names from
   `json:"..."` struct tags, swagger schemas). Money is int64 USDC cents;
   bid credits are int64 whole credits; enums are MONOSPACE_UPPERCASE;
   dates are ISO-8601 UTC strings. Never use floats for money.
   ============================================================ */

// ---- shared enums (string unions match backend constants) ----
export type Tier = "GUEST" | "MEMBER" | "VIP";
export type KycStatus = "PENDING" | "APPROVED" | "REJECTED";
export type AType = "DUTCH" | "VICKREY" | "UNIQBID";
export type LotState = "DRAFT" | "CERTIFIED" | "SCHEDULED" | "REJECTED";
export type AttestationResult = "PASS" | "FAIL";
export type VaultObjectState =
  | "IN_VAULT" | "APPRAISING" | "IN_AUCTION" | "SOLD" | "BOUGHT_BACK";
export type DutchState =
  | "DRAFT" | "APPRAISING" | "SCHEDULED" | "OPEN" | "HAMMER"
  | "SETTLING" | "COMPLETED" | "CANCELLED" | "ABORTED";
export type PassiveState =
  | "DRAFT" | "APPRAISING" | "SCHEDULED" | "OPEN" | "CLOSING"
  | "RESOLVED" | "SETTLING" | "COMPLETED" | "ABORTED";
export type ReservationKind = "DEPOSIT_10" | "FULL_LOCK";
export type ReservationState = "REQUESTED" | "LOCKED" | "RELEASED";
export type TradeKind = "DUTCH" | "PASSIVE";
export type EscrowState =
  | "UNLOCKED" | "DEPOSIT_LOCKED" | "FULL_LOCKED" | "HELD"
  | "RELEASED" | "REFUNDED" | "FORFEITED" | "DISPUTED";
export type ReleaseMode = "CASH" | "VAULT_CREDIT";
export type BuybackMode = "CASH" | "CREDIT";
export type InviteStatus = "ISSUED" | "REDEEMED" | "REVOKED" | "FLAGGED";
export type DocType = "PASSPORT" | "NATIONAL_ID";
export type KycSubmissionState =
  | "STARTED" | "OTP_VERIFIED" | "SUBMITTED" | "APPROVED" | "REJECTED";

// ---- error envelope ----
export interface ApiError {
  message: string;
  details?: string;
  code?: string;
}

// ---- catalog ----
export interface Lot {
  id: string;
  objectId: string;
  sellerAccountId: string;
  title: string;
  description: string;
  atype: AType;
  durationDays: number | null;
  reserveCents: number;
  appraisedValueCents: number;
  state: LotState;
  isoWeek: string;
  createdAt: string;
  scheduledAt?: string;
  /** Catalog category code (WATCHES|JEWELRY|…); client localizes the label. */
  categoryCode?: string;
  /** Presentation category (drives the icon/glyph) when set explicitly. */
  category?: Category;
  /** Ordered carousel images (≤7); imageRefs[0] is the cover. */
  imageRefs?: string[];
  /** Inspector seal (populated after an APPROVED inspection). */
  certified?: boolean;
  authenticity?: "GENUINE" | "COUNTERFEIT" | "INCONCLUSIVE";
  conditionGrade?: "MINT" | "EXCELLENT" | "GOOD" | "FAIR" | "POOR";
  /** Owner-authored 4-language title/description, returned whole. */
  titleI18n?: Partial<Record<"en" | "fa" | "ar" | "tr", string>>;
  descriptionI18n?: Partial<Record<"en" | "fa" | "ar" | "tr", string>>;
}
export interface Attestation {
  id: string;
  lotId: string;
  inspectorId: string;
  result: AttestationResult;
  notesRef?: string;
  recordedAt: string;
}
export interface WeeklyGallery {
  week: string;
  supplyCap: number;
  lots: Lot[];
}
export interface LotDetail {
  lot: Lot;
  certified: boolean;
  attestations: Attestation[];
}

// ---- identity ----
export type Role = "USER" | "INSPECTOR" | "ADMIN";
export type AccountStatus = "REGISTERED" | "ACTIVE" | "SUSPENDED" | "BANNED";

export interface Account {
  id: string;
  tier: Tier;
  kycStatus: KycStatus;
  eligible: boolean;
  createdAt: string;
  updatedAt: string;
  /** Display name chosen at registration (optional). */
  handle?: string;
  /** Elevated functional roles (USER is implicit). */
  roles?: Role[];
  status?: AccountStatus;
  mobileE164?: string;
  /** Paid membership level (1 = free Member on sign-in; 2+ are purchased). */
  membershipLevel?: number;
}

// ---- auth (mobile OTP + OAuth) ----
export type OAuthProvider = "GOOGLE" | "FACEBOOK" | "APPLE";
export interface RequestOtpResp { expiresInSecs: number; devCode?: string; }
export interface SessionResp { token: string; created: boolean; account: Account; }
export interface Access {
  id: string;
  tier: Tier;
  kycStatus: KycStatus;
  eligible: boolean;
}

// ---- invite ----
export interface RedeemInviteReq { code: string; }
export interface RedeemInviteResp {
  code: string;
  redeemedBy: string;
  issuedBy: string;
}

// ---- kyc ----
export interface StartKycReq { docType: DocType; docRef: string; phone: string; }
export interface StartKycResp {
  submissionId: string;
  challengeId: string;
  state: string;
  expiresAt: string;
  devCode?: string;
}
export interface VerifyKycReq { code: string; }
export interface KycSubmission {
  id: string;
  accountId: string;
  docType: DocType;
  docRef: string;
  phone: string;
  state: KycSubmissionState;
  rejectionReason?: string;
  submittedAt: string;
  decidedAt?: string;
}

// ---- vault ----
export interface VaultObject {
  id: string;
  title: string;
  description: string;
  appraisedValueCents: number;
  state: VaultObjectState;
  createdAt: string;
  updatedAt: string;
  /** Product category — drives the one icon shown for this object everywhere. */
  category?: Category;
  /** Up to 7 images of the object (imageRefs[0] is the cover). */
  imageRefs?: string[];
}
export interface VaultView {
  objects: VaultObject[];
  creditBalanceCents: number;
}
export interface CreateObjectReq {
  maison?: string;
  title: string;
  description?: string;
  appraisedValueCents: number;
  /** Category drives the object's icon (one icon, compatible with the type). */
  category?: Category;
  /** Up to 7 images (imageRefs[0] is the cover). */
  imageRefs?: string[];
}
export interface ListObjectReq { atype: AType; durationDays?: number; }
export interface BuybackReq { mode: BuybackMode; }
export interface BuybackResp {
  object: VaultObject;
  mode: BuybackMode;
  payoutCents: number;
  balanceCents: number;
}

// ---- bids ----
export interface BidPackage {
  id: string;
  credits: number;
  priceCents: number;
  bestValue: boolean;
}
export interface Purchase {
  id: string;
  packageId: string;
  creditsGranted: number;
  usdcChargedCents: number;
  createdAt: string;
}
export interface Debit {
  id: string;
  amountCredits: number;
  idempotencyKey: string;
  auctionId: string;
  createdAt: string;
}
export interface Wallet {
  accountId: string;
  balanceCredits: number;
  updatedAt: string;
  purchases: Purchase[];
  debits: Debit[];
}
export interface BuyBidsReq { packageId: string; idempotencyKey?: string; }
export interface BuyBidsResp {
  creditsGranted: number;
  usdcChargedCents: number;
  balanceCredits: number;
}

// ---- auction-dutch ----
export interface DutchAuction {
  id: string;
  lotId: string;
  state: DutchState;
  ceilingCents: number;
  floorCents: number;
  dropStepCents: number;
  dropIntervalSeconds: number;
  currentPriceCents: number;
  openAt?: string;
  nextDropAt?: string;
  hammerAt?: string;
  winnerAccountId?: string;
  hammerPriceCents?: number | null;
  createdAt: string;
}
export interface Reservation {
  id: string;
  auctionId: string;
  accountId: string;
  kind: ReservationKind;
  state: ReservationState;
  amountCents: number;
  escrowRef: string;
  createdAt: string;
}

// ---- auction-passive ----
export interface PassiveAuction {
  id: string;
  lotId: string;
  atype: Extract<AType, "VICKREY" | "UNIQBID">;
  state: PassiveState;
  closesAt: string;
  reserveCents: number;
  participantCount: number;
  winnerAccountId?: string;
  clearedPriceCents?: number;
}
export interface PlaceBidReq { priceCents: number; requestId?: string; }
export interface BidResp {
  id: string;
  auctionId: string;
  priceCents: number;
  placedAt: string;
}
export interface StandingPrice {
  priceCents: number;
  isLowestUnique: boolean;
  placedAt: string;
}
export interface Standing {
  auctionId: string;
  atype: Extract<AType, "VICKREY" | "UNIQBID">;
  state: PassiveState;
  closesAt: string;
  prices: StandingPrice[];
}

// ---- escrow ----
export interface Balance { participantId: string; balanceCents: number; }
export interface Conservation {
  inflowsCents: number;
  disbursedCents: number;
  balanced: boolean;
}
export interface Trade {
  id: string;
  lotId: string;
  buyerId: string;
  sellerId: string;
  kind: TradeKind;
  state: EscrowState;
  priceCents: number;
  premiumCents: number;
  feeCents: number;
  inspectorFeeCents: number;
  obligationCents: number;
  releaseMode?: ReleaseMode;
  fundingDeadline?: string;
  createdAt: string;
  updatedAt: string;
  balances: Balance[];
  conservation: Conservation;
}
export interface FundReq { amountCents: number; }
export interface ConfirmReq { mode: ReleaseMode; }
export interface TradeState {
  id: string;
  state: EscrowState;
  kind: TradeKind;
  releaseMode?: ReleaseMode;
}

// ---- client-side view enrichment ----
// The backend lot is intentionally minimal/language-neutral. The client derives
// presentation hints (category glyph, accent) it owns — see lib/enrich.ts.
export type Category =
  | "horology" | "bag" | "sneaker" | "perfume" | "art" | "painting" | "jewel";
