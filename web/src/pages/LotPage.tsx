import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useAccount, useLot, useReserve } from "@/hooks/queries";
import { useSession } from "@/hooks/useSession";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { Sheet } from "@/components/ui/Sheet";
import { Icon } from "@/components/ui/Icon";
import { Seal } from "@/components/ui/Seal";
import { Money } from "@/components/ui/Money";
import { LotCarousel } from "@/components/ui/LotCarousel";
import { Label, Stat, iconBtnStyle } from "@/components/ui/Primitives";
import { LoadingScreen, ErrorState } from "@/components/ui/States";
import { categoryLabel } from "@/lib/enrich";
import { toLotView } from "@/lib/lotView";
import { dollars, fmtClock, shortRef, usdc, usdc0 } from "@/lib/format";
import type { Lot } from "@/types";

const PREMIUM_PCT = 0.1; // platform buyer's premium (escrow computes the real figure)

export function LotPage() {
  const { id = "" } = useParams();
  const { t, lang, dir } = useI18n();
  const nav = useNavigate();
  const { canParticipate, parts, setPart } = useSession();
  const { data, isLoading, isError, refetch } = useLot(id);
  const reserve = useReserve(id);
  const [certOpen, setCertOpen] = useState(false);
  const [reserveOpen, setReserveOpen] = useState(false);

  if (isLoading) return <ScreenShell top={<TopBar title="…" onBack={() => nav(-1)} />}><LoadingScreen /></ScreenShell>;
  if (isError || !data) return <ScreenShell top={<TopBar title="—" onBack={() => nav(-1)} />}><ErrorState onRetry={() => refetch()} /></ScreenShell>;

  const lot = data.lot;
  const v = toLotView(lot);
  const seq = Number(lot.id.replace(/\D/g, "")) || 0;
  const ref = shortRef(lot.isoWeek, seq);
  const deposit = Math.round(lot.reserveCents * 0.1);
  const registered = !!parts[lot.id];
  const inspector = data.attestations[0]?.inspectorId ?? "House Inspector";

  const onReserve = async () => {
    try {
      await reserve.mutateAsync();
      setPart(lot.id, "REQUESTED");
    } finally {
      setReserveOpen(false);
    }
  };

  const footer = (
    <div className="safe-bottom" style={{ padding: "12px 16px 18px", background: "var(--bg-1)", borderTop: "1px solid var(--line)" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10, padding: "0 4px" }}>
        <span className="mono up" style={{ fontSize: 10, color: "var(--fg-muted)" }}>{v.isLive ? t("auc_live") : t("await_live")}</span>
        <span className="mono" style={{ fontSize: 12, color: "var(--gold-pale)" }}>{v.isLive ? "—" : fmtClock(v.opensInSec)}</span>
      </div>
      {v.isLive ? (
        <button className="btn btn-gold" style={{ width: "100%" }} onClick={() => nav(`/auction/${lot.id}`)}>
          {t("lot_enter") || "Enter live auction"} <Icon name={dir === "rtl" ? "arrow-left" : "arrow-right"} size={18} />
        </button>
      ) : v.isPassive ? (
        <button className="btn btn-gold" style={{ width: "100%" }} onClick={() => nav(`/passive/${lot.id}`)}>
          <Icon name="clock" size={17} /> {t("passive_kicker")}
        </button>
      ) : !canParticipate ? (
        <button className="btn btn-burg" style={{ width: "100%" }} onClick={() => nav("/login")}>
          <Icon name="crown" size={17} /> {t("need_member")}
        </button>
      ) : registered ? (
        <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 10, padding: "13px", border: "1px solid var(--st-good)", borderRadius: "var(--r-1)", background: "var(--st-good-bg)", color: "var(--st-good)", fontWeight: 600 }}>
          <Icon name="check" size={18} /> {t("registered")} · <span className="mono">{usdc0(deposit)}</span>
        </div>
      ) : (
        <button className="btn btn-gold" style={{ width: "100%" }} onClick={() => setReserveOpen(true)} disabled={reserve.isPending}>
          <Icon name="lock" size={17} /> {t("req_participate")} · {usdc0(deposit)}
        </button>
      )}
    </div>
  );

  return (
    <>
      <ScreenShell
        top={<TopBar kicker={`${t("gal_lot")} ${String(seq).padStart(2, "0")} · ${categoryLabel(v.category, lang)}`} title={v.maison} onBack={() => nav(-1)} right={<button className="iconbtn" style={iconBtnStyle} aria-label="watch"><Icon name="eye" size={18} /></button>} />}
        footer={footer}
      >
        <LotCarousel images={lot.imageRefs} art={v.art} label={v.maison} />

        <div style={{ padding: "18px 20px 0" }}>
          <h1 className="serif" style={{ fontSize: 24, lineHeight: 1.18, margin: "0 0 4px", color: "var(--gold-pale)" }}>{v.body}</h1>
          <div className="muted" style={{ fontSize: 13, marginBottom: 18 }}>{t(lot.atype === "DUTCH" ? "mode_dutch" : lot.atype === "VICKREY" ? "mode_vickrey" : "mode_uniqbid")}</div>

          <div style={{ display: "flex", gap: 10, marginBottom: 20 }}>
            <Stat label={t("gal_start")} value={<Money cents={lot.appraisedValueCents} withCents={false} />} />
            <Stat label={t("gal_floor")} value={<Money cents={lot.reserveCents} withCents={false} />} gold />
          </div>

          <button onClick={() => setCertOpen(true)} style={{ width: "100%", cursor: "pointer", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "var(--bg-1)", padding: "14px 16px", display: "flex", alignItems: "center", gap: 14, color: "var(--fg)", textAlign: "start" }}>
            <Seal size={48} label="CERT" sub="" id={String(inspector).slice(0, 4)} date={lot.isoWeek} />
            <div style={{ flex: 1 }}>
              <div style={{ fontWeight: 600, fontSize: 14 }}>{t("lot_cert") || "Certificate of authenticity"}</div>
              <div className="muted mono" style={{ fontSize: 11, marginTop: 3 }}>{ref}</div>
            </div>
            <Icon name={dir === "rtl" ? "chevron-left" : "chevron-right"} size={18} style={{ color: "var(--gold)" }} />
          </button>

          <div className="kv" style={{ marginTop: 20 }}>
            <div className="k">{t("lot_brand") || "Maison"}</div><div className="v">{v.maison}</div>
            <div className="k">{t("auc_mode")}</div><div className="v">{lot.atype}</div>
            <div className="k">{t("gal_floor")}</div><div className="v">{dollars(lot.reserveCents)}</div>
            <div className="k">{t("lot_inspected") || "Inspected by"}</div><div className="v" style={{ fontSize: 11 }}>{inspector}</div>
            <div className="k">{t("lot_ref") || "Reference"}</div><div className="v" style={{ fontSize: 11 }}>{ref}</div>
            <div className="k">{t("auc_premium")}</div><div className="v">{Math.round(PREMIUM_PCT * 100)}%</div>
          </div>

          <div style={{ marginTop: 18, marginBottom: 24 }}>
            <Label>{t("lot_about") || "About"}</Label>
            <p className="muted" style={{ fontSize: 14, lineHeight: 1.7, margin: 0 }}>{lot.description}</p>
          </div>
        </div>
      </ScreenShell>

      <CertModal open={certOpen} onClose={() => setCertOpen(false)} lot={lot} ref_={ref} inspector={inspector} body={v.body} />
      <ReserveSheet open={reserveOpen} onClose={() => setReserveOpen(false)} onReserve={onReserve} lot={lot} body={v.body} maison={v.maison} deposit={deposit} pending={reserve.isPending} />
    </>
  );
}

function ReserveSheet({ open, onClose, onReserve, lot, body, maison, deposit, pending }: {
  open: boolean; onClose: () => void; onReserve: () => void; lot: Lot; body: string; maison: string; deposit: number; pending: boolean;
}) {
  const { t } = useI18n();
  const { account } = useSession();
  return (
    <Sheet open={open} onClose={onClose}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 6 }}>
        <span style={{ color: "var(--gold)" }}><Icon name="lock" size={20} /></span>
        <div className="serif" style={{ fontSize: 20, color: "var(--gold-pale)" }}>{t("req_title")}</div>
      </div>
      <p className="muted" style={{ fontSize: 13, lineHeight: 1.6, margin: "0 0 18px" }}>{t("req_body")}</p>
      <div className="kv" style={{ marginBottom: 20 }}>
        <div className="k">{maison}</div><div className="v" style={{ fontSize: 11 }}>{body}</div>
        <div className="k">{t("gal_floor")}</div><div className="v">{usdc(lot.reserveCents)}</div>
        <div className="k">{t("req_deposit")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{usdc(deposit)}</div>
        <div className="k">{t("esc_balance")}</div><div className="v">{account ? t("nav_account") : "—"}</div>
      </div>
      <button className="btn btn-gold" style={{ width: "100%" }} onClick={onReserve} disabled={pending}>
        <Icon name="lock" size={17} /> {t("req_cta")} · {usdc0(deposit)}
      </button>
    </Sheet>
  );
}

function CertModal({ open, onClose, lot, ref_, inspector, body }: {
  open: boolean; onClose: () => void; lot: Lot; ref_: string; inspector: string; body: string;
}) {
  const { t } = useI18n();
  if (!open) return null;
  return (
    <div onClick={onClose} style={{ position: "absolute", inset: 0, zIndex: 60, background: "rgba(8,5,6,0.7)", backdropFilter: "blur(6px)", WebkitBackdropFilter: "blur(6px)", display: "flex", alignItems: "center", justifyContent: "center", padding: 20 }}>
      <div onClick={(e) => e.stopPropagation()} className="doc grain fade-up" dir="ltr" style={{ width: "100%", maxWidth: 340, borderRadius: 2 }}>
        <span className="tick-bl" /><span className="tick-br" />
        <div style={{ textAlign: "center" }}>
          <div style={{ display: "inline-flex", color: "var(--burg)" }}><Seal size={64} label="CERTIFIED" sub="DAUCTION" id={String(inspector).slice(0, 4)} date={lot.isoWeek} /></div>
          <div className="serif" style={{ fontSize: 18, color: "var(--ink)", marginTop: 8, fontWeight: 700 }}>Certificate of Authenticity</div>
          <div className="mono" style={{ fontSize: 10, color: "var(--ink-muted)", letterSpacing: "0.1em", marginTop: 4 }}>{ref_}</div>
        </div>
        <div style={{ height: 1, background: "var(--ink-line)", margin: "16px 0" }} />
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12.5, color: "var(--ink)" }}>
          <tbody>
            {[["Object", body], ["Auction type", lot.atype], ["Reserve", dollars(lot.reserveCents)], ["Appraised", dollars(lot.appraisedValueCents)], ["Inspector", inspector], ["Week", lot.isoWeek]].map(([k, val], i) => (
              <tr key={i}>
                <td style={{ color: "var(--ink-muted)", padding: "7px 0", verticalAlign: "top", width: "40%" }}>{k}</td>
                <td style={{ padding: "7px 0", fontFamily: "var(--mono)", textAlign: "right" }}>{val}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <div style={{ height: 1, background: "var(--ink-line)", margin: "14px 0" }} />
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end" }}>
          <div>
            <div style={{ fontFamily: "var(--serif)", fontStyle: "italic", fontSize: 18, color: "var(--burg)", borderBottom: "1px solid var(--ink-line)", paddingBottom: 2 }}>D. Auction</div>
            <div className="mono" style={{ fontSize: 8.5, color: "var(--ink-muted)", marginTop: 4 }}>HOUSE REGISTRAR</div>
          </div>
          <div className="mono" style={{ fontSize: 9, color: "var(--ink-muted)", textAlign: "right" }}>SIGNED ON-CHAIN<br />{lot.objectId.slice(0, 6)}…</div>
        </div>
        <button className="btn" onClick={onClose} style={{ width: "100%", marginTop: 18, background: "var(--burg)", color: "var(--paper)" }}>{t("common_close")}</button>
      </div>
    </div>
  );
}
