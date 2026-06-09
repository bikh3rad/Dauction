import { useEffect, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useTrade, useFundTrade, useConfirmTrade, useLot } from "@/hooks/queries";
import { useInterval } from "@/hooks/useInterval";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { Icon } from "@/components/ui/Icon";
import { Seal } from "@/components/ui/Seal";
import { Money } from "@/components/ui/Money";
import { Ph } from "@/components/ui/ProductArt";
import { Chip } from "@/components/ui/Chip";
import { LoadingScreen, ErrorState } from "@/components/ui/States";
import { Fragment } from "react";
import { fmtClock, usdc, usdc0 } from "@/lib/format";
import { toLotView } from "@/lib/lotView";

type Phase = "complete" | "funded" | "release" | "done";

function phaseFromState(state: string): Phase {
  if (state === "RELEASED") return "done";
  if (state === "HELD") return "release";
  return "complete";
}

export function EscrowPage() {
  const { id = "" } = useParams();
  const { t } = useI18n();
  const nav = useNavigate();
  const location = useLocation();
  const priceCents = (location.state as { priceCents?: number } | null)?.priceCents;

  const { data: lotData } = useLot(id);
  const { data: trade, isLoading, isError, refetch } = useTrade(id, priceCents);
  const fund = useFundTrade(id);
  const confirm = useConfirmTrade(id);

  const [phase, setPhase] = useState<Phase>("complete");
  const [secs, setSecs] = useState(24 * 3600 - 12);
  useInterval(() => setSecs((s) => s - 37), 1000, phase === "complete"); // accelerated demo clock

  useEffect(() => {
    if (trade) setPhase(phaseFromState(trade.state));
  }, [trade]);

  if (isLoading) return <ScreenShell top={<TopBar title={t("esc_title")} onBack={() => nav(-1)} />}><LoadingScreen /></ScreenShell>;
  if (isError || !trade) return <ScreenShell top={<TopBar title={t("esc_title")} onBack={() => nav(-1)} />}><ErrorState onRetry={() => refetch()} /></ScreenShell>;

  const lot = lotData?.lot;
  const v = lot ? toLotView(lot) : null;
  const deposit = trade.balances[0]?.balanceCents ?? Math.round(trade.priceCents * 0.1);
  const dueNow = trade.obligationCents - deposit;

  const flow: Phase[] = ["complete", "funded", "release", "done"];
  const idx = flow.indexOf(phase);

  const doFund = async () => {
    await fund.mutateAsync(dueNow);
    setPhase("funded");
  };
  const doConfirm = async () => {
    await confirm.mutateAsync("CASH");
    setPhase("done");
  };

  return (
    <ScreenShell top={<TopBar kicker={v?.maison ?? lot?.title} title={t("esc_title")} onBack={() => nav(-1)} />}>
      <div style={{ padding: "16px 20px 40px" }}>
        {/* escrow state diagram */}
        <div style={{ display: "flex", gap: 6, marginBottom: 22, overflowX: "auto" }} className="noscroll">
          {([["FUNDED", "funded"], ["IN_TRANSIT", "release"], ["DELIVERED", "release"], ["COMPLETED", "done"]] as const).map(([s, ph], i) => {
            const reached = flow.indexOf(ph) <= idx || (i === 0 && idx >= 0);
            return (
              <Fragment key={i}>
                <span className="chip" data-st={reached ? (s === "COMPLETED" ? "good" : "active") : "neut"} style={{ opacity: reached ? 1 : 0.45 }}>{s}</span>
                {i < 3 && <span style={{ color: "var(--fg-faint)", alignSelf: "center" }}>›</span>}
              </Fragment>
            );
          })}
        </div>

        {phase === "complete" && (
          <div className="fade-up">
            <h2 className="serif" style={{ fontSize: 22, color: "var(--gold-pale)", margin: "0 0 8px" }}>{t("esc_complete_title")}</h2>
            <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, margin: "0 0 18px" }}>{t("esc_complete_body")}</p>
            <div style={{ border: "1px solid var(--st-warn)", borderRadius: "var(--r-2)", padding: "14px 16px", marginBottom: 18, background: "var(--st-warn-bg)", display: "flex", alignItems: "center", gap: 12 }}>
              <Icon name="clock" size={20} style={{ color: "var(--st-warn)" }} />
              <div style={{ flex: 1 }}>
                <div className="mono up" style={{ fontSize: 9, color: "var(--st-warn)" }}>{t("esc_remaining")}</div>
                <div className="mono" style={{ fontSize: 20, color: "var(--fg)" }}>{fmtClock(secs)}</div>
              </div>
            </div>
            <div className="kv" style={{ marginBottom: 20 }}>
              <div className="k">{t("esc_hammer")}</div><div className="v">{usdc(trade.priceCents)}</div>
              <div className="k">{t("esc_locked")} · 10%</div><div className="v" style={{ color: "var(--st-good)" }}>− {usdc(deposit)}</div>
              <div className="k">{t("auc_premium")}</div><div className="v">{usdc(trade.premiumCents)}</div>
              <div className="k" style={{ fontWeight: 700, color: "var(--fg)" }}>{t("esc_total")}</div><div className="v" style={{ color: "var(--gold-pale)", fontSize: 15 }}>{usdc(dueNow)}</div>
            </div>
            <button className="btn btn-gold" style={{ width: "100%" }} onClick={doFund} disabled={fund.isPending}>{t("esc_fund")} · {usdc0(dueNow)}</button>
          </div>
        )}

        {phase === "funded" && (
          <div className="fade-up" style={{ textAlign: "center", paddingTop: 10 }}>
            <Seal size={104} label="FUNDED" sub="ESCROW" id="0x7A4E" date="HELD" live />
            <div style={{ marginTop: 14 }}><Chip state="funded" label="FUNDED" /></div>
            <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, margin: "14px auto 0", maxWidth: 280 }}>{t("esc_funded")}.</p>
            <div className="kv" style={{ marginTop: 20, textAlign: "start" }}>
              <div className="k">{t("esc_total")}</div><div className="v" style={{ color: "var(--gold-pale)" }}>{usdc(trade.obligationCents)}</div>
              <div className="k">{t("esc_release_title")}</div><div className="v"><span className="chip" data-st="active">IN_TRANSIT</span></div>
            </div>
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 22 }} onClick={() => setPhase("release")}>{t("common_continue")} <Icon name="arrow-right" size={18} /></button>
          </div>
        )}

        {phase === "release" && (
          <div className="fade-up">
            <h2 className="serif" style={{ fontSize: 22, color: "var(--gold-pale)", margin: "0 0 8px" }}>{t("esc_release_title")}</h2>
            <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, margin: "0 0 18px" }}>{t("esc_release_body")}</p>
            <div style={{ position: "relative", marginBottom: 18 }}>
              <Ph art={v?.art ?? "watch"} ratio="4 / 3" label={`${v?.maison ?? ""} · in hand`} style={{ borderRadius: "var(--r-2)" }} />
              <div style={{ position: "absolute", top: 12, insetInlineStart: 12 }}><span className="chip" data-st="active"><span className="dot" />IN_TRANSIT</span></div>
            </div>
            <button className="btn btn-gold" style={{ width: "100%" }} onClick={doConfirm} disabled={confirm.isPending}>
              <Icon name="check" size={18} /> {t("esc_release")}
            </button>
          </div>
        )}

        {phase === "done" && (
          <div className="fade-up" style={{ textAlign: "center", paddingTop: 10 }}>
            <div className="stampin"><Seal size={112} label="RELEASED" sub="DAUCTION" id="0x7A4E" date="COMPLETE" live /></div>
            <div style={{ marginTop: 14 }}><Chip state="completed" label="COMPLETED" /></div>
            <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, margin: "14px auto 0", maxWidth: 290 }}>{t("esc_released")}. {t("esc_release_body")}</p>
            <div style={{ marginTop: 18, padding: "14px 16px", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "var(--bg-1)" }}>
              <div className="mono up" style={{ fontSize: 9, color: "var(--gold)", marginBottom: 6 }}>seller payout · 110% {t("clo_creditopt")}</div>
              <Money cents={Math.round(trade.priceCents * 1.1)} big gold withCents={false} />
            </div>
            <button className="btn btn-gold" style={{ width: "100%", marginTop: 22 }} onClick={() => nav("/")}>{t("nav_gallery")}</button>
          </div>
        )}
      </div>
    </ScreenShell>
  );
}
