import { useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { Chip } from "@/components/ui/Chip";
import { Icon } from "@/components/ui/Icon";
import { Money } from "@/components/ui/Money";
import {
  useAdminStats, useAdminAccounts, useAdminKyc, useAdminCert,
  useAdminAuctions, useAdminVault, useAdminEscrow,
  useSetAccountStatus, useSetAccountTier, useSetAccountRole, useDecideKyc, useCertify,
  useCreateAuction, useSetAuctionState, useUpdateAuction, useHoldRelease, useRuleDispute,
} from "@/hooks/adminQueries";
import type { AType } from "@/types";
import type { DisputeRuling } from "@/types/admin";
import { SecHead, Tile, DTable, GBtn, Actions, Dash, tdS, tdEnd, tdMuted } from "./adminUi";

const mono = (color = "var(--gold-pale)") => ({ fontFamily: "var(--mono)", color });

// ============================ Overview ============================
export function Overview({ go }: { go: (s: string) => void }) {
  const { t } = useI18n();
  const { data: s } = useAdminStats();
  const { data: accounts = [] } = useAdminAccounts();
  const { data: escrow = [] } = useAdminEscrow();
  const inspectors = accounts.filter((a) => (a.roles ?? []).includes("INSPECTOR")).length;
  return (
    <div className="fade-up">
      <SecHead kicker="WEEK 2026·W23" title={t("adm_overview")} />
      <div style={{ display: "flex", gap: 14, flexWrap: "wrap", marginBottom: 26 }}>
        <Tile label={t("adm_members")} value={s?.members ?? accounts.length} sub={`${inspectors} inspectors`} accent />
        <Tile label={t("adm_pending_kyc")} value={s?.pendingKyc ?? "—"} sub={`${accounts.length} accounts`} />
        <Tile label={t("adm_open_auctions")} value={s?.openAuctions ?? "—"} sub={`${s?.lotsThisWeek ?? 0} / ${s?.supplyCap ?? 32} lots`} />
        <Tile label={t("adm_escrow_locked")} value={s ? <Money cents={s.escrowLockedCents} withCents={false} /> : "—"} sub={`${escrow.length} trades`} accent />
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1.3fr 1fr", gap: 20 }}>
        <div>
          <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginBottom: 10, cursor: "pointer" }} onClick={() => go("users")}>{t("adm_accounts")} ↗</div>
          <DTable cols={[{ label: t("adm_account") }, { label: t("adm_tier") }, { label: t("adm_roles") || "Roles", end: true }]}>
            {accounts.slice(0, 4).map((a) => (
              <tr key={a.id}>
                <td style={tdS}><span style={mono()}>{a.handle}</span></td>
                <td style={tdMuted}><Chip state={a.tier === "VIP" ? "active" : "ISSUED"} label={a.tier} /></td>
                <td style={tdEnd}>{(a.roles ?? []).includes("INSPECTOR") ? <Chip state="active" label="INSPECTOR" /> : <span className="mono" style={{ fontSize: 11, color: "var(--fg-faint)" }}>USER</span>}</td>
              </tr>
            ))}
          </DTable>
        </div>
        <div>
          <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginBottom: 10, cursor: "pointer" }} onClick={() => go("escrow")}>{t("adm_escrow")} ↗</div>
          <DTable cols={[{ label: t("adm_object") }, { label: t("adm_status"), end: true }]}>
            {escrow.map((e) => (
              <tr key={e.id}>
                <td style={tdS}>{e.lot}<div className="mono" style={{ fontSize: 10.5, color: "var(--fg-faint)", marginTop: 2 }}><Money cents={e.amountCents} withCents={false} /></div></td>
                <td style={tdEnd}><Chip state={e.state} /></td>
              </tr>
            ))}
          </DTable>
        </div>
      </div>
    </div>
  );
}

// ============================ Auctions ============================
const MODES: AType[] = ["DUTCH", "VICKREY", "UNIQBID"];
export function Auctions() {
  const { t } = useI18n();
  const { data: rows = [] } = useAdminAuctions();
  const setState = useSetAuctionState();
  const create = useCreateAuction();
  const update = useUpdateAuction();
  const [form, setForm] = useState<{ lotId: string; atype: AType; floor: string; days: number; bidCost: number } | null>(null);
  const [edit, setEdit] = useState<{ id: string; title: string; isPassive: boolean; bidCost: number; floor: number; high: number } | null>(null);

  const submit = () => {
    if (!form?.lotId) return;
    create.mutate({
      lotId: form.lotId, atype: form.atype, floorCents: Math.round(Number(form.floor || 0) * 100),
      durationDays: form.atype === "DUTCH" ? undefined : form.days,
      bidCostCredits: form.atype === "DUTCH" ? undefined : form.bidCost,
    });
    setForm(null);
  };

  const saveEdit = () => {
    if (!edit) return;
    update.mutate({ id: edit.id, title: edit.title, floorCents: Math.round(edit.floor * 100), appraisedCents: Math.round(edit.high * 100), bidCostCredits: edit.isPassive ? edit.bidCost : undefined });
    setEdit(null);
  };

  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_auctions")}
        action={<GBtn kind="gold" onClick={() => setForm({ lotId: "", atype: "DUTCH", floor: "", days: 2, bidCost: 1 })}><Icon name="plus" size={13} /> {t("adm_create_auction")}</GBtn>} />

      {form && (
        <div style={{ marginBottom: 20, border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", padding: 18, background: "var(--bg-1)", display: "flex", gap: 14, flexWrap: "wrap", alignItems: "flex-end" }}>
          <Field label={t("adm_lot")}><input value={form.lotId} onChange={(e) => setForm({ ...form, lotId: e.target.value })} placeholder="lot-13" style={inputS} /></Field>
          <Field label={t("adm_mode")}>
            <select value={form.atype} onChange={(e) => setForm({ ...form, atype: e.target.value as AType })} style={inputS}>
              {MODES.map((m) => <option key={m} value={m}>{m}</option>)}
            </select>
          </Field>
          <Field label={t("adm_floor")}><input value={form.floor} onChange={(e) => setForm({ ...form, floor: e.target.value.replace(/[^0-9]/g, "") })} placeholder="100000" style={inputS} /></Field>
          {form.atype !== "DUTCH" && (
            <>
              <Field label={t("adm_duration_days")}>
                <select value={form.days} onChange={(e) => setForm({ ...form, days: Number(e.target.value) })} style={inputS}>
                  {[2, 5, 7].map((d) => <option key={d} value={d}>{d}</option>)}
                </select>
              </Field>
              <Field label={t("adm_bid_cost")}>
                <input value={form.bidCost} onChange={(e) => setForm({ ...form, bidCost: Math.max(1, Number(e.target.value.replace(/[^0-9]/g, "")) || 1) })} style={inputS} />
              </Field>
            </>
          )}
          <div style={{ display: "flex", gap: 8 }}>
            <GBtn kind="gold" onClick={submit}>{t("adm_create")}</GBtn>
            <GBtn onClick={() => setForm(null)}>{t("adm_cancel")}</GBtn>
          </div>
        </div>
      )}

      {edit && (
        <div style={{ marginBottom: 20, border: "1px solid var(--gold)", borderRadius: "var(--r-2)", padding: 18, background: "var(--bg-1)", display: "flex", gap: 14, flexWrap: "wrap", alignItems: "flex-end" }}>
          <div style={{ flexBasis: "100%", marginBottom: 4 }}>
            <span className="mono up" style={{ fontSize: 9.5, color: "var(--gold)" }}>{t("adm_edit")}</span>
          </div>
          <Field label={t("adm_title_field")}>
            <input value={edit.title} onChange={(e) => setEdit({ ...edit, title: e.target.value })} style={{ ...inputS, minWidth: 240 }} />
          </Field>
          <Field label={`${t("adm_floor")} · ${t("price_low")}`}>
            <input value={edit.floor} onChange={(e) => setEdit({ ...edit, floor: Number(e.target.value.replace(/[^0-9]/g, "")) || 0 })} style={inputS} />
          </Field>
          <Field label={`${t("adm_high")} · ${t("price_high")}`}>
            <input value={edit.high} onChange={(e) => setEdit({ ...edit, high: Number(e.target.value.replace(/[^0-9]/g, "")) || 0 })} style={inputS} />
          </Field>
          {edit.isPassive && (
            <Field label={t("adm_bid_cost")}>
              <input value={edit.bidCost} onChange={(e) => setEdit({ ...edit, bidCost: Math.max(1, Number(e.target.value.replace(/[^0-9]/g, "")) || 1) })} style={inputS} />
            </Field>
          )}
          <div style={{ display: "flex", gap: 8 }}>
            <GBtn kind="gold" onClick={saveEdit}>{t("adm_save") || "Save"}</GBtn>
            <GBtn onClick={() => setEdit(null)}>{t("adm_cancel")}</GBtn>
          </div>
        </div>
      )}

      <DTable cols={[{ label: t("adm_object") }, { label: t("adm_type") }, { label: t("adm_price") }, { label: t("adm_participants") }, { label: t("adm_status") }, { label: t("adm_action"), end: true }]}>
        {rows.map((a) => (
          <tr key={a.id}>
            <td style={tdS}>{a.title}<div className="mono" style={{ fontSize: 10.5, color: "var(--fg-faint)", marginTop: 2 }}>{a.maison}</div></td>
            <td style={tdS} className="mono">{a.atype}{a.atype !== "DUTCH" && a.bidCostCredits != null && <div style={{ fontSize: 10, color: "var(--fg-faint)", marginTop: 2 }}>{a.bidCostCredits} cr/bid</div>}</td>
            <td style={tdS} className="mono"><Money cents={a.priceCents} withCents={false} /></td>
            <td style={tdMuted} className="mono">{a.participants}</td>
            <td style={tdS}><Chip state={a.state} /></td>
            <td style={tdEnd}><Actions>
              <GBtn small onClick={() => setEdit({ id: a.id, title: a.title, isPassive: a.atype !== "DUTCH", bidCost: a.bidCostCredits ?? 1, floor: Math.round(a.priceCents / 100), high: Math.round((a.highCents ?? a.priceCents) / 100) })}>{t("adm_edit")}</GBtn>
              {a.state === "DRAFT" && <GBtn small onClick={() => setState.mutate({ id: a.id, state: "SCHEDULED" })}>{t("adm_schedule")}</GBtn>}
              {a.state === "SCHEDULED" && <GBtn kind="gold" small onClick={() => setState.mutate({ id: a.id, state: "OPEN" })}>{t("adm_open")}</GBtn>}
              {(a.state === "OPEN" || a.state === "CLOSING") && <GBtn kind="bad" small onClick={() => setState.mutate({ id: a.id, state: "ABORTED" })}>{t("adm_disable")}</GBtn>}
              {!["DRAFT", "SCHEDULED", "OPEN", "CLOSING"].includes(a.state) && <Dash />}
            </Actions></td>
          </tr>
        ))}
      </DTable>
    </div>
  );
}

// ============================ Accounts ============================
export function Accounts() {
  const { t } = useI18n();
  const { data: rows = [] } = useAdminAccounts();
  const setStatus = useSetAccountStatus();
  const setTier = useSetAccountTier();
  const setRole = useSetAccountRole();
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_accounts")} />
      <DTable cols={[{ label: t("adm_account") }, { label: t("adm_tier") }, { label: "KYC" }, { label: t("adm_roles") || "Roles" }, { label: t("adm_status") }, { label: t("adm_action"), end: true }]}>
        {rows.map((a) => {
          const isInspector = (a.roles ?? []).includes("INSPECTOR");
          return (
          <tr key={a.id}>
            <td style={tdS}><span style={mono()}>{a.id}</span><div className="mono" style={{ fontSize: 10.5, color: "var(--fg-faint)", marginTop: 2 }}>{a.handle}</div></td>
            <td style={tdS}><Chip state={a.tier === "VIP" ? "active" : "ISSUED"} label={a.tier} /></td>
            <td style={tdS}><Chip state={a.kycStatus} /></td>
            <td style={tdS}>
              {(a.roles ?? []).length
                ? <span style={{ display: "inline-flex", gap: 5, flexWrap: "wrap" }}>{(a.roles ?? []).map((r) => <Chip key={r} state="active" label={r} />)}</span>
                : <span className="mono" style={{ fontSize: 11, color: "var(--fg-faint)" }}>USER</span>}
            </td>
            <td style={tdS}><Chip state={a.status === "ACTIVE" ? "active" : "flagged"} label={a.status} /></td>
            <td style={tdEnd}><Actions>
              {isInspector
                ? <GBtn kind="bad" small onClick={() => setRole.mutate({ id: a.id, role: "INSPECTOR", grant: false })}>{t("adm_revoke_inspector") || "Revoke Inspector"}</GBtn>
                : <GBtn kind="gold" small onClick={() => setRole.mutate({ id: a.id, role: "INSPECTOR", grant: true })}>{t("adm_make_inspector") || "Make Inspector"}</GBtn>}
              {a.tier === "MEMBER"
                ? <GBtn small onClick={() => setTier.mutate({ id: a.id, tier: "VIP" })}>{t("adm_grant_vip")}</GBtn>
                : a.tier === "VIP" && <GBtn small onClick={() => setTier.mutate({ id: a.id, tier: "MEMBER" })}>{t("adm_set_member")}</GBtn>}
              {a.status === "ACTIVE"
                ? <GBtn kind="bad" small onClick={() => setStatus.mutate({ id: a.id, status: "SUSPENDED" })}>{t("adm_suspend")}</GBtn>
                : <GBtn kind="gold" small onClick={() => setStatus.mutate({ id: a.id, status: "ACTIVE" })}>{t("adm_reinstate")}</GBtn>}
            </Actions></td>
          </tr>
        );})}
      </DTable>
    </div>
  );
}

// ============================ Memberships (KYC) ============================
export function Memberships() {
  const { t } = useI18n();
  const { data: rows = [] } = useAdminKyc();
  const decide = useDecideKyc();
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_memberships")} />
      <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginBottom: 10 }}>{t("adm_pending_kyc")}</div>
      <DTable cols={[{ label: t("adm_member") }, { label: t("adm_document") }, { label: t("adm_issued_by") }, { label: t("adm_status") }, { label: t("adm_action"), end: true }]}>
        {rows.map((k) => (
          <tr key={k.id}>
            <td style={tdS}><span style={mono()}>{k.handle}</span><div className="mono" style={{ fontSize: 10.5, color: "var(--fg-faint)", marginTop: 2 }}>{k.accountId}</div></td>
            <td style={tdS}>{k.docType.replace("_", " ")}</td>
            <td style={tdMuted} className="mono">{k.issuedBy}</td>
            <td style={tdS}><Chip state={k.status} /></td>
            <td style={tdEnd}>{k.status === "PENDING"
              ? <Actions>
                  <GBtn kind="gold" small onClick={() => decide.mutate({ id: k.id, approve: true })}>{t("adm_approve")}</GBtn>
                  <GBtn kind="bad" small onClick={() => decide.mutate({ id: k.id, approve: false })}>{t("adm_reject")}</GBtn>
                </Actions>
              : <span style={{ color: "var(--gold)" }}>✓</span>}</td>
          </tr>
        ))}
      </DTable>
    </div>
  );
}

// ============================ Member vaults + certification ============================
export function Vaults() {
  const { t } = useI18n();
  const { data: vault = [] } = useAdminVault();
  const { data: cert = [] } = useAdminCert();
  const certify = useCertify();
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_vaults")} />
      <DTable cols={[{ label: t("adm_owner") }, { label: t("adm_object") }, { label: t("adm_maison") }, { label: t("adm_value") }, { label: t("adm_status"), end: true }]}>
        {vault.map((v) => (
          <tr key={v.id}>
            <td style={tdMuted} className="mono">{v.ownerHandle}</td>
            <td style={tdS}>{v.title}</td>
            <td style={tdMuted}>{v.maison}</td>
            <td style={tdS} className="mono"><Money cents={v.valueCents} withCents={false} /></td>
            <td style={tdEnd}><Chip state={v.state} /></td>
          </tr>
        ))}
      </DTable>

      <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", margin: "26px 0 10px" }}>{t("adm_certification")}</div>
      <DTable cols={[{ label: t("adm_object") }, { label: t("adm_maison") }, { label: t("adm_value") }, { label: t("adm_status") }, { label: t("adm_action"), end: true }]}>
        {cert.map((c) => (
          <tr key={c.lotId}>
            <td style={tdS}>{c.object}</td>
            <td style={tdMuted}>{c.maison}</td>
            <td style={tdS} className="mono"><Money cents={c.valueCents} withCents={false} /></td>
            <td style={tdS}><Chip state={c.status} /></td>
            <td style={tdEnd}>{c.status === "APPRAISING"
              ? <GBtn kind="gold" small onClick={() => certify.mutate(c.lotId)}>{t("adm_certify")}</GBtn>
              : <span style={{ color: "var(--gold)" }}>✓</span>}</td>
          </tr>
        ))}
      </DTable>
    </div>
  );
}

// ============================ Escrow + dispute court ============================
export function Escrow() {
  const { t } = useI18n();
  const { data: rows = [] } = useAdminEscrow();
  const hold = useHoldRelease();
  const rule = useRuleDispute();
  const disputed = rows.find((r) => r.state === "DISPUTED");
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_escrow")} />
      <DTable cols={[{ label: "ESCROW" }, { label: t("adm_object") }, { label: t("adm_member") }, { label: t("adm_value") }, { label: t("adm_premium") }, { label: t("adm_status") }, { label: t("adm_action"), end: true }]}>
        {rows.map((e) => (
          <tr key={e.id}>
            <td style={tdS}><span style={mono()}>{e.id}</span></td>
            <td style={tdS}>{e.lot}</td>
            <td style={tdMuted} className="mono">{e.memberHandle}</td>
            <td style={tdS} className="mono"><Money cents={e.amountCents} withCents={false} /></td>
            <td style={tdMuted} className="mono"><Money cents={e.premiumCents} withCents={false} /></td>
            <td style={tdS}><Chip state={e.state} /></td>
            <td style={tdEnd}>
              {e.state === "DISPUTED" ? <span className="mono" style={{ fontSize: 11, color: "var(--st-bad)" }}>IN COURT</span>
                : e.state === "HELD" ? <GBtn kind="bad" small onClick={() => hold.mutate(e.id)}>{t("adm_hold")}</GBtn>
                : <Dash />}
            </td>
          </tr>
        ))}
      </DTable>

      <div style={{ marginTop: 22, border: "1px solid var(--st-bad)", borderRadius: "var(--r-2)", overflow: "hidden" }}>
        <div style={{ padding: "14px 18px", background: "var(--st-bad-bg)", display: "flex", alignItems: "center", gap: 12, borderBottom: "1px solid var(--st-bad)" }}>
          <span style={{ color: "var(--st-bad)" }}><Icon name="scale" size={22} /></span>
          <div>
            <div className="serif" style={{ fontSize: 17, color: "var(--fg)" }}>{t("adm_dispute")} {disputed ? `· ${disputed.id}` : ""}</div>
            <div className="mono" style={{ fontSize: 11.5, color: "var(--fg-muted)", marginTop: 2 }}>{disputed ? `${disputed.lot} · buyer claims non-authentic on delivery` : t("adm_no_disputes")}</div>
          </div>
        </div>
        {disputed && (
          <div style={{ padding: 18, display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
            <span className="muted" style={{ fontSize: 13 }}>{t("adm_resolve")}:</span>
            {(["REFUND_BUYER", "RELEASE_SELLER", "SPLIT"] as DisputeRuling[]).map((r) => (
              <GBtn key={r} small onClick={() => rule.mutate({ id: disputed.id, ruling: r })}>{r}</GBtn>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ---- small form helpers ----
const inputS: React.CSSProperties = { background: "var(--bg-0)", border: "1px solid var(--line-strong)", borderRadius: "var(--r-1)", color: "var(--fg)", padding: "9px 11px", fontSize: 13, fontFamily: "var(--mono)", minWidth: 130 };
function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      <span className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{label}</span>
      {children}
    </label>
  );
}
