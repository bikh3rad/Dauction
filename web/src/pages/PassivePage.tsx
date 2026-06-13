import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useLot, usePassiveAuction, usePlaceBid, useStanding, useWallet } from "@/hooks/queries";
import { useSession } from "@/hooks/useSession";
import { useCountdown } from "@/hooks/useCountdown";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { Icon } from "@/components/ui/Icon";
import { Ph } from "@/components/ui/ProductArt";
import { AuctionStatusGuide } from "@/components/ui/AuctionStatusGuide";
import { CertBadge } from "@/components/ui/CertBadge";
import { Label } from "@/components/ui/Primitives";
import { LoadingScreen, ErrorState } from "@/components/ui/States";
import { toLotView } from "@/lib/lotView";
import { usdc0 } from "@/lib/format";
import type { PassiveAuction, Standing } from "@/types";

function Sep() { return <span style={{ color: "var(--fg-faint)", opacity: 0.5 }}>:</span>; }

function CountdownPill({ d, h, m, s }: { d: number; h: number; m: number; s: number }) {
  const { t } = useI18n();
  const seg = (n: number, lbl: string) => (
    <span style={{ display: "inline-flex", alignItems: "baseline", gap: 3 }}>
      <span className="mono tnum" style={{ fontSize: 15, color: "var(--gold-pale)", fontWeight: 600 }}>{String(n).padStart(2, "0")}</span>
      <span className="mono up" style={{ fontSize: 8, color: "var(--fg-faint)" }}>{lbl}</span>
    </span>
  );
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
      {seg(d, t("d_short"))}<Sep />{seg(h, t("h_short"))}<Sep />{seg(m, t("m_short"))}<Sep />{seg(s, "s")}
    </div>
  );
}

export function PassivePage() {
  const { id = "" } = useParams();
  const { t, lang } = useI18n();
  const nav = useNavigate();
  const { isGuest } = useSession();
  const { data: lotData } = useLot(id);
  const { data: auction, isLoading, isError, refetch } = usePassiveAuction(id);
  const cd = useCountdown(auction?.closesAt, true);

  if (isLoading) return <ScreenShell top={<TopBar title="…" onBack={() => nav(-1)} />}><LoadingScreen /></ScreenShell>;
  if (isError || !auction) return <ScreenShell top={<TopBar title="—" onBack={() => nav(-1)} />}><ErrorState onRetry={() => refetch()} /></ScreenShell>;

  const lot = lotData?.lot;
  const v = lot ? toLotView(lot) : null;
  const atype = auction.atype;
  const seq = Number(id.replace(/\D/g, "")) || 0;

  return (
    <ScreenShell
      top={
        <TopBar
          onBack={() => nav(-1)}
          kicker={`${t("gal_lot")} ${String(seq).padStart(2, "0")} · ${t(atype === "VICKREY" ? "mode_vickrey" : "mode_uniqbid")}`}
          title={v?.maison ?? "Timed auction"}
          right={<span className="chip" data-st="warn"><Icon name="clock" size={12} /> {cd.d}{t("d_short")}</span>}
        />
      }
    >
      <div style={{ position: "relative" }}>
        <Ph art={v?.art ?? "frame"} artW="50%" artTop="38%" ratio="4 / 3" style={{ borderRadius: 0, borderInline: 0 }} />
        <div style={{ position: "absolute", top: 14, insetInlineStart: 14 }}>
          <span className="chip" data-st="warn"><Icon name="clock" size={12} /> {t("passive_kicker")}</span>
        </div>
        <div style={{ position: "absolute", insetInlineStart: 0, insetInlineEnd: 0, bottom: 0, height: "60%", background: "linear-gradient(to top, var(--bg-void) 8%, rgba(12,8,9,0.8) 36%, transparent 100%)" }} />
        <div style={{ position: "absolute", bottom: 14, insetInlineStart: 16, insetInlineEnd: 16 }}>
          <div className="serif" style={{ fontSize: 21, color: "var(--gold-pale)", lineHeight: 1.2 }}>{v?.body ?? lot?.title}</div>
        </div>
      </div>

      <div style={{ padding: "18px 20px 40px" }}>
        {lotData?.certified && <div style={{ marginBottom: 16 }}><CertBadge /></div>}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "14px 16px", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "var(--bg-1)", marginBottom: 18 }}>
          <div className="mono up" style={{ fontSize: 10, color: "var(--gold)" }}>{t("ends_in")}</div>
          <CountdownPill d={cd.d} h={cd.h} m={cd.m} s={cd.s} />
        </div>

        {/* rule explainer */}
        <div style={{ display: "flex", gap: 12, padding: "14px 16px", borderRadius: "var(--r-2)", border: "1px solid var(--line)", background: "var(--bg-1)", marginBottom: 18 }}>
          <span style={{ color: "var(--gold)", flexShrink: 0, marginTop: 2 }}><Icon name={atype === "VICKREY" ? "file" : "hash"} size={18} /></span>
          <div>
            <div style={{ fontWeight: 600, fontSize: 13.5, marginBottom: 4 }}>{t(atype === "VICKREY" ? "mode_vickrey" : "mode_uniqbid")} · {t("auc_mode")}</div>
            <p className="muted" style={{ fontSize: 12.5, lineHeight: 1.6, margin: 0 }}>{t(atype === "VICKREY" ? "vickrey_rule" : "uniqbid_rule")}</p>
            <div className="mono" style={{ fontSize: 11.5, color: "var(--gold-pale)", marginTop: 8 }}>
              <Icon name="coins" size={13} /> {t("bid_cost_label")}: <b>{auction.bidCostCredits}</b> {auction.bidCostCredits === 1 ? t("credit") : t("credits")} / {t("bid_one")}
            </div>
          </div>
        </div>

        {/* the item's two prices: low (minimum bid) and high (appraised) */}
        <div style={{ display: "flex", gap: 10, marginBottom: 18 }}>
          <div style={{ flex: 1, padding: "12px 14px", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "var(--bg-1)" }}>
            <div className="mono up" style={{ fontSize: 9, color: "var(--gold)" }}>{t("price_low")}</div>
            <div className="mono" style={{ fontSize: 18, color: "var(--gold-pale)", marginTop: 4 }}>{usdc0(auction.reserveCents)}</div>
          </div>
          <div style={{ flex: 1, padding: "12px 14px", border: "1px solid var(--line)", borderRadius: "var(--r-2)", background: "var(--bg-1)" }}>
            <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{t("price_high")}</div>
            <div className="mono" style={{ fontSize: 18, color: "var(--fg)", marginTop: 4 }}>{usdc0(auction.highCents)}</div>
          </div>
        </div>

        {isGuest ? (
          <button className="btn btn-burg" style={{ width: "100%" }} onClick={() => nav("/login")}>
            <Icon name="user" size={17} /> {t("need_signin")}
          </button>
        ) : (
          <>
            <BidWalletStrip />
            {atype === "VICKREY" ? <VickreyPanel id={id} auction={auction} /> : <UniqBidPanel id={id} auction={auction} />}
          </>
        )}

        <div style={{ marginTop: 22 }}>
          <Label>{t("lot_about") || "About"}</Label>
          <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.7, margin: 0 }}>{lot?.description}</p>
        </div>

        <div style={{ marginTop: 20 }}><AuctionStatusGuide /></div>
      </div>
    </ScreenShell>
  );
}

function BidWalletStrip() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { data: wallet } = useWallet();
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "12px 14px", borderRadius: "var(--r-2)", border: "1px solid var(--gold-line)", background: "linear-gradient(100deg,var(--burg-deep),var(--bg-1))", marginBottom: 18 }}>
      <span style={{ color: "var(--gold)" }}><Icon name="coins" size={20} /></span>
      <div style={{ flex: 1 }}>
        <div className="mono up" style={{ fontSize: 9, color: "var(--gold)" }}>{t("bid_wallet")}</div>
        <div className="mono" style={{ fontSize: 17, color: "var(--gold-pale)", marginTop: 2 }}>{wallet?.balanceCredits ?? "—"} <span style={{ fontSize: 11, color: "var(--fg-muted)" }}>{t("bid_credits")}</span></div>
      </div>
      <button className="btn btn-ghost" style={{ padding: "8px 14px", fontSize: 12.5 }} onClick={() => nav("/bidstore")}>
        <Icon name="plus" size={15} /> {t("buy_bids")}
      </button>
    </div>
  );
}

function NeedBids() {
  const { t } = useI18n();
  const nav = useNavigate();
  return (
    <div style={{ marginTop: 14, padding: "14px 16px", border: "1px solid var(--st-warn)", borderRadius: "var(--r-2)", background: "var(--st-warn-bg)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, color: "var(--st-warn)", fontWeight: 600, fontSize: 13 }}>
        <Icon name="alert" size={16} /> {t("need_bids")}
      </div>
      <p className="muted" style={{ fontSize: 12, margin: "8px 0 12px" }}>{t("need_bids_cta")}</p>
      <button className="btn btn-gold" style={{ width: "100%" }} onClick={() => nav("/bidstore")}><Icon name="coins" size={16} /> {t("buy_bids")}</button>
    </div>
  );
}

// ---- Vickrey: one sealed bid ----
function VickreyPanel({ id, auction }: { id: string; auction: PassiveAuction }) {
  const { t } = useI18n();
  const { data: wallet } = useWallet();
  const { data: standing } = useStanding(id);
  const placeBid = usePlaceBid(id);
  const sealedCents = standing?.prices[0]?.priceCents;
  const [amt, setAmt] = useState("");
  const [edit, setEdit] = useState(true);

  const submit = async () => {
    const dollarsVal = parseInt(amt.replace(/\D/g, ""), 10);
    if (!dollarsVal) return;
    await placeBid.mutateAsync(dollarsVal * 100);
    setEdit(false);
    setAmt("");
  };
  const cost = auction.bidCostCredits;
  const below = amt !== "" && Number(amt) * 100 < auction.reserveCents;
  const sealed = sealedCents != null;
  const noBids = (wallet?.balanceCredits ?? 0) < cost && !sealed;

  if (!edit && sealed) {
    return (
      <div style={{ border: "1px solid var(--st-good)", borderRadius: "var(--r-2)", background: "var(--st-good-bg)", padding: "16px 18px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div>
            <div className="mono up" style={{ fontSize: 9, color: "var(--st-good)" }}>{t("your_sealed_bid")}</div>
            <div className="mono" style={{ fontSize: 20, color: "var(--gold-pale)", marginTop: 4 }}>{usdc0(sealedCents!)}</div>
          </div>
          <span className="chip" data-st="good"><Icon name="lock" size={12} /> SEALED</span>
        </div>
        <p className="muted" style={{ fontSize: 12, lineHeight: 1.5, margin: "12px 0 0" }}>{t("sealed_until")}</p>
        <button className="btn btn-ghost" style={{ width: "100%", marginTop: 14 }} onClick={() => setEdit(true)}>{t("update_bid")}</button>
      </div>
    );
  }
  return (
    <div>
      <Label>{t("enter_amount")}</Label>
      <input className="field field-mono" dir="ltr" inputMode="numeric" value={amt} onChange={(e) => setAmt(e.target.value.replace(/[^\d]/g, ""))} placeholder={String(Math.round(auction.reserveCents / 100))} style={{ fontSize: 18 }} />
      <div className="muted mono" style={{ fontSize: 11, margin: "8px 2px 0" }}>{t("price_low")} · {usdc0(auction.reserveCents)}</div>
      {below && <div className="mono" style={{ fontSize: 11, color: "var(--st-bad)", marginTop: 6 }}>{t("bid_below_min")} {usdc0(auction.reserveCents)}</div>}
      {noBids ? (
        <NeedBids />
      ) : (
        <button className="btn btn-gold" style={{ width: "100%", marginTop: 14 }} disabled={!amt || below || placeBid.isPending} onClick={submit}>
          <Icon name="lock" size={16} /> {t("submit_sealed")} · {cost} {cost === 1 ? t("credit") : t("credits")}
        </button>
      )}
    </div>
  );
}

// ---- UniqBid: many unique prices ----
function UniqBidPanel({ id, auction }: { id: string; auction: PassiveAuction }) {
  const { t } = useI18n();
  const { data: wallet } = useWallet();
  const { data: standing } = useStanding(id) as { data: Standing | undefined };
  const placeBid = usePlaceBid(id);
  const [price, setPrice] = useState("");

  const mine = standing?.prices ?? [];
  const youLead = mine.some((p) => p.isLowestUnique);
  const myBestUnique = mine.filter((p) => p.isLowestUnique).map((p) => p.priceCents).sort((a, b) => a - b)[0];

  const place = async () => {
    const dollarsVal = parseInt(price.replace(/\D/g, ""), 10);
    if (!dollarsVal) return;
    await placeBid.mutateAsync(dollarsVal * 100);
    setPrice("");
  };
  const cost = auction.bidCostCredits;
  const below = price !== "" && Number(price) * 100 < auction.reserveCents;
  const noBids = (wallet?.balanceCredits ?? 0) < cost;

  return (
    <div>
      <div style={{ display: "flex", gap: 10, marginBottom: 14 }}>
        <div style={{ flex: 1, padding: "12px 14px", border: "1px solid var(--line)", borderRadius: "var(--r-2)", background: "var(--bg-1)" }}>
          <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{t("lowest_unique_now")}</div>
          <div className="mono" style={{ fontSize: 17, color: "var(--gold-pale)", marginTop: 4 }}>{myBestUnique != null ? usdc0(myBestUnique) : "—"}</div>
        </div>
        <div style={{ flex: 1, padding: "12px 14px", border: "1px solid", borderColor: youLead ? "var(--st-good)" : "var(--line)", borderRadius: "var(--r-2)", background: youLead ? "var(--st-good-bg)" : "var(--bg-1)" }}>
          <div className="mono up" style={{ fontSize: 9, color: youLead ? "var(--st-good)" : "var(--fg-faint)" }}>{t("standing")}</div>
          <div style={{ fontSize: 14, fontWeight: 600, marginTop: 5, color: youLead ? "var(--st-good)" : "var(--fg-muted)" }}>{youLead ? t("you_lead") : t("you_trail")}</div>
        </div>
      </div>

      <Label>{t("enter_amount")}</Label>
      <div style={{ display: "flex", gap: 8 }}>
        <input className="field field-mono" dir="ltr" inputMode="numeric" value={price} onChange={(e) => setPrice(e.target.value.replace(/[^\d]/g, ""))} placeholder="e.g. 237" style={{ flex: 1, fontSize: 17 }} onKeyDown={(e) => e.key === "Enter" && place()} />
        <button className="btn btn-gold" disabled={!price || below || noBids || placeBid.isPending} onClick={place} style={{ whiteSpace: "nowrap" }}>
          <Icon name="coins" size={15} /> {cost}
        </button>
      </div>
      <div className="muted mono" style={{ fontSize: 11, margin: "8px 2px 0" }}>{t("price_low")}: {usdc0(auction.reserveCents)} · {cost} {cost === 1 ? t("credit") : t("credits")}/{t("bid_one")}</div>
      {below && <div className="mono" style={{ fontSize: 11, color: "var(--st-bad)", marginTop: 6 }}>{t("bid_below_min")} {usdc0(auction.reserveCents)}</div>}
      {noBids && <div style={{ marginTop: 12 }}><NeedBids /></div>}

      {mine.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <div className="mono up" style={{ fontSize: 10, color: "var(--fg-muted)", marginBottom: 8 }}>{t("your_bids")} · {mine.length}</div>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
            {[...mine].reverse().map((p, i) => (
              <span key={i} className="chip" data-st={p.isLowestUnique ? "good" : "neut"} style={{ fontSize: 11 }}>
                <span className="mono">{usdc0(p.priceCents)}</span> · {t(p.isLowestUnique ? "status_unique" : "status_taken")}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
