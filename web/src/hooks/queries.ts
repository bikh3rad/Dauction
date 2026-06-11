import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  auth, bids, catalog, dutch, escrow, identity, invite, kyc, passive, vault,
} from "@/services";
import { signOut as clearSession } from "@/auth/session";
import type { AType, BuybackMode, DocType, OAuthProvider, ReleaseMode } from "@/types";

// ---- query keys ----
export const qk = {
  me: ["me"] as const,
  gallery: (week?: string) => ["gallery", week ?? "current"] as const,
  lot: (id: string) => ["lot", id] as const,
  dutch: (id: string) => ["dutch", id] as const,
  passive: (id: string) => ["passive", id] as const,
  standing: (id: string) => ["standing", id] as const,
  wallet: ["wallet"] as const,
  packages: ["packages"] as const,
  vault: ["vault"] as const,
  kycStatus: ["kyc-status"] as const,
  trade: (id: string) => ["trade", id] as const,
};

// ---------------- identity ----------------
export function useAccount() {
  return useQuery({ queryKey: qk.me, queryFn: identity.me, staleTime: 30_000 });
}

// ---------------- auth (mobile OTP + OAuth) ----------------
export function useRequestOtp() {
  return useMutation({ mutationFn: (mobile: string) => auth.requestOtp(mobile) });
}
export function useVerifyOtp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { mobile: string; code: string }) => auth.verifyOtp(v.mobile, v.code),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.me }),
  });
}
export function useOAuthLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (provider: OAuthProvider) => auth.oauth(provider),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.me }),
  });
}
export function useSignOut() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => { clearSession(); },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.me }),
  });
}

// ---------------- catalog ----------------
export function useGallery(week?: string) {
  return useQuery({ queryKey: qk.gallery(week), queryFn: () => catalog.weekly(week) });
}
export function useLot(id: string) {
  return useQuery({ queryKey: qk.lot(id), queryFn: () => catalog.lot(id), enabled: !!id });
}

// ---------------- dutch ----------------
export function useDutchAuction(id: string, enabled = true) {
  return useQuery({
    queryKey: qk.dutch(id),
    queryFn: () => dutch.get(id),
    enabled: enabled && !!id,
    refetchInterval: 15_000, // periodic resync; engine animates in between
  });
}
export function useReserve(id: string) {
  return useMutation({ mutationFn: () => dutch.reserve(id) });
}
export function useLockFull(id: string) {
  return useMutation({ mutationFn: () => dutch.lock(id) });
}
export function useBuyNow(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => dutch.buy(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.dutch(id) }),
  });
}

// ---------------- passive ----------------
export function usePassiveAuction(id: string, enabled = true) {
  return useQuery({ queryKey: qk.passive(id), queryFn: () => passive.get(id), enabled: enabled && !!id });
}
export function useStanding(id: string, enabled = true) {
  return useQuery({
    queryKey: qk.standing(id),
    queryFn: () => passive.standing(id),
    enabled: enabled && !!id,
    refetchInterval: 20_000,
  });
}
export function usePlaceBid(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (priceCents: number) => passive.bid(id, priceCents),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.standing(id) });
      qc.invalidateQueries({ queryKey: qk.wallet });
    },
  });
}

// ---------------- bids ----------------
export function useWallet() {
  return useQuery({ queryKey: qk.wallet, queryFn: bids.wallet, staleTime: 10_000 });
}
export function usePackages() {
  return useQuery({ queryKey: qk.packages, queryFn: bids.packages, staleTime: 5 * 60_000 });
}
export function useBuyBids() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (packageId: string) => bids.buy(packageId),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.wallet }),
  });
}

// ---------------- vault ----------------
export function useVault() {
  return useQuery({ queryKey: qk.vault, queryFn: vault.view });
}
export function useListObject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; atype: AType; durationDays?: number }) =>
      vault.list(v.id, v.atype, v.durationDays),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vault }),
  });
}
export function useBuyback() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; mode: BuybackMode }) => vault.buyback(v.id, v.mode),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vault }),
  });
}

// ---------------- invite ----------------
export function useRedeemInvite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => invite.redeem(code),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.me }),
  });
}

// ---------------- kyc ----------------
export function useStartKyc() {
  return useMutation({
    mutationFn: (v: { docType: DocType; docRef: string; phone: string }) =>
      kyc.start(v.docType, v.docRef, v.phone),
  });
}
export function useVerifyKyc() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => kyc.verify(code),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.kycStatus }),
  });
}
export function useKycStatus(enabled = false) {
  return useQuery({ queryKey: qk.kycStatus, queryFn: kyc.status, enabled });
}

// ---------------- escrow ----------------
export function useTrade(id: string, priceCents?: number, enabled = true) {
  return useQuery({
    queryKey: qk.trade(id),
    queryFn: () => escrow.get(id, priceCents),
    enabled: enabled && !!id,
  });
}
export function useFundTrade(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (amountCents: number) => escrow.fund(id, amountCents),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.trade(id) }),
  });
}
export function useConfirmTrade(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (mode: ReleaseMode) => escrow.confirm(id, mode),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.trade(id) }),
  });
}
