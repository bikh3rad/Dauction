import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { admin } from "@/services/admin";
import type { AccountStatus, AdminAuction, CreateAuctionReq, DisputeRuling } from "@/types/admin";
import type { Role, Tier } from "@/types";

// ---- query keys ----
export const aqk = {
  stats: ["adm", "stats"] as const,
  accounts: ["adm", "accounts"] as const,
  kyc: ["adm", "kyc"] as const,
  cert: ["adm", "cert"] as const,
  auctions: ["adm", "auctions"] as const,
  vault: ["adm", "vault"] as const,
  escrow: ["adm", "escrow"] as const,
};

// Invalidate the stat tiles after any mutation that changes counts.
function useAdminMutation<V>(fn: (v: V) => Promise<unknown>, keys: readonly (readonly string[])[]) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => [aqk.stats, ...keys].forEach((k) => qc.invalidateQueries({ queryKey: k })),
  });
}

// ---- reads ----
export const useAdminStats = () => useQuery({ queryKey: aqk.stats, queryFn: admin.stats });
export const useAdminAccounts = () => useQuery({ queryKey: aqk.accounts, queryFn: admin.accounts });
export const useAdminKyc = () => useQuery({ queryKey: aqk.kyc, queryFn: admin.kyc });
export const useAdminCert = () => useQuery({ queryKey: aqk.cert, queryFn: admin.cert });
export const useAdminAuctions = () => useQuery({ queryKey: aqk.auctions, queryFn: admin.auctions });
export const useAdminVault = () => useQuery({ queryKey: aqk.vault, queryFn: admin.vault });
export const useAdminEscrow = () => useQuery({ queryKey: aqk.escrow, queryFn: admin.escrow });

// ---- mutations ----
export const useSetAccountStatus = () =>
  useAdminMutation((v: { id: string; status: AccountStatus }) => admin.setAccountStatus(v.id, v.status), [aqk.accounts]);

export const useSetAccountTier = () =>
  useAdminMutation((v: { id: string; tier: Tier }) => admin.setAccountTier(v.id, v.tier), [aqk.accounts]);

export const useSetAccountRole = () =>
  useAdminMutation((v: { id: string; role: Role; grant: boolean }) => admin.setAccountRole(v.id, v.role, v.grant), [aqk.accounts]);

export const useDecideKyc = () =>
  useAdminMutation((v: { id: string; approve: boolean }) => admin.decideKyc(v.id, v.approve), [aqk.kyc, aqk.accounts]);

export const useCertify = () =>
  useAdminMutation((lotId: string) => admin.certify(lotId), [aqk.cert]);

export const useCreateAuction = () =>
  useAdminMutation((req: CreateAuctionReq) => admin.createAuction(req), [aqk.auctions]);

export const useSetAuctionState = () =>
  useAdminMutation((v: { id: string; state: AdminAuction["state"] }) => admin.setAuctionState(v.id, v.state), [aqk.auctions]);

export const useHoldRelease = () =>
  useAdminMutation((id: string) => admin.holdRelease(id), [aqk.escrow]);

export const useRuleDispute = () =>
  useAdminMutation((v: { id: string; ruling: DisputeRuling }) => admin.ruleDispute(v.id, v.ruling), [aqk.escrow]);
