import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useDutchAuction, useLockFull, useBuyNow } from "@/hooks/queries";
import { useDutchEngine } from "@/hooks/useDutchEngine";
import { useInterval } from "@/hooks/useInterval";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { Sheet } from "@/components/ui/Sheet";
import { Icon } from "@/components/ui/Icon";
import { Seal } from "@/components/ui/Seal";
import { Ph } from "@/components/ui/ProductArt";
import { Chip } from "@/components/ui/Chip";
import { AuctionStatusGuide } from "@/components/ui/AuctionStatusGuide";
import { CertBadge } from "@/components/ui/CertBadge";
import { MiniStat } from "@/components/ui/Primitives";
import { LoadingScreen, ErrorState } from "@/components/ui/States";
import { useLot } from "@/hooks/queries";
import { toLotView } from "@/lib/lotView";
import { dollars, usdc, usdc0 } from "@/lib/format";
import type { DutchAuction } from "@/types";

// big descending price, flashes on each drop
function PriceBig({ cents, flash, size = 46 }: { cents: number; flash: boolean; size?: number }) {
  return (
    <div className="mono tnum" style={{ fontSize: size, fontWeight: 600, lineHeight: 1, letterSpacing: "-0.02em", color: flash ? "var(--gold-bright)" : "var(--gold-pale)", transition: "color 320ms var(--ease-doc)", textShadow: flash ? "0 0 18px var(--gold-glow)" : "none" }}>
      {dollars(cents)}
    </div>
  );
}

function HeatRing({ nextIn, every, size = 224, atFloor }: { nextIn: number; every: number; size?: number; atFloor: boolean }) {
  const r = size / 2 - 8;
  const c = 2 * Math.PI * r;
  const frac = every > 0 ? 1 - nextIn / every : 1;
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} style={{ transform: "rotate(-90deg)" }}>
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--line)" strokeWidth="3" />
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--gold)" strokeWidth="3" strokeDasharray={c} strokeDashoffset={c * (1 - frac)} strokeLinecap="round" style={{ transition: "stroke-dashoffset 100ms linear", filter: atFloor ? "none" : "drop-shadow(0 0 4px var(--gold-glow))" }} />
    </svg>
  );
}

// live "room" ticker — simulated activity so the demo feels alive.
function useRoom(seed: number, active: boolean) {
  const [room, setRoom] = useState(8 + (seed % 10));
  const [watch, setWatch] = useState(180 + (seed % 280));
  const [toast, setToast] = useState<{ id: number; txt: string } | null>(null);
  const actors = ["0x9F", "0x4C", "0x11", "0xB2", "0x7E", "0xA1", "0x2D", "0xF7"];
  useInterval(() => {
    const r = Math.random();
    setWatch((w) => Math.max(120, w + (r < 0.5 ? 1 : -1)));
    if (r < 0.25) {
      const a = actors[Math.floor(Math.random() * actors.length)];
      const kinds = ["locked a deposit", "entered the room", "is watching"];
      const k = kinds[Math.floor(Math.random() * kinds.length)];
      if (k === "locked a deposit") setRoom((n) => n + 1);
      setToast({ id: Date.now(), txt: `${a} ${k}` });
    }
  }, 3000, active);
  useEffect(() => {
    if (!toast) return;
    const id = setTimeout(() => setToast(null), 2600);
    return () => clearTimeout(id);
  }, [toast]);
  return { room, watch, toast };
}

export function AuctionPage() {
  const { id = "" } = useParams();
  const { t } = useI18n();
  const nav = useNavigate();
  const { data: lotData } = useLot(id);
  const { data: auction, isLoading, isError, refetch } = useDutchAuction(id);
  const lockFull = useLockFull(id);
  const buyNow = useBuyNow(id);

  const [locked, setLocked] = useState(false);
  const [lockSheet, setLockSheet] = useState(false);
  const [won, setWon] = useState<DutchAuction | null>(null);
  const [flash, setFlash] = useState(false);

  const engine = useDutchEngine(auction, !won);
  const prevDrops = useRef(engine.drops);
  useEffect(() => {
    if (engine.drops !== prevDrops.current) {
      prevDrops.current = engine.drops;
      setFlash(true);
      const tid = setTimeout(() => setFlash(false), 360);
      return () => clearTimeout(tid);
    }
  }, [engine.drops]);

  const seed = Number(id.replace(/\D/g, "")) || 7;
  const room = useRoom(seed, !won);

  if (isLoading) return <ScreenShell top={<TopBar title="…" onBack={() => nav(-1)} />}><LoadingScreen /></ScreenShell>;
  if (isError || !auction) return <ScreenShell top={<TopBar title="—" onBack={() => nav(-1)} />}><ErrorState onRetry={() => refetch()} /></ScreenShell>;

  const lot = lotData?.lot;
  const v = lot ? toLotView(lot) : null;
  const title = v?.body ?? "Live auction";
  const maison = v?.maison ?? "Dauction";
  const art = v?.art ?? "watch";

  const onBuy = async () => {
    const res = await buyNow.mutateAsync();
    setWon(res);
  };

  if (won) {
    const hammer = won.hammerPriceCents ?? engine.priceCents;
    return (
      <WonView
        maison={maison}
        title={title}
        hammerCents={hammer}
        onBack={() => setWon(null)}
        onContinue={() => nav(`/escrow/${id}`, { state: { priceCents: hammer } })}
      />
    );
  }

  const RoomBar = (
    <div style={{ display: "flex", alignItems: "center", gap: 14, fontSize: 12, color: "var(--fg-muted)" }}>
      <span style={{ display: "flex", alignItems: "center", gap: 5 }}><Icon name="users" size={14} /><span className="mono">{room.room}</span> {t("auc_participants")}</span>
      <span style={{ display: "flex", alignItems: "center", gap: 5 }}><Icon name="eye" size={14} /><span className="mono">{room.watch}</span></span>
    </div>
  );

  const footer = (
    <div className="safe-bottom" style={{ zIndex: 25, padding: "14px 16px 18px", background: "var(--bg-1)", borderTop: "1px solid var(--gold-line)" }}>
      {!locked ? (
        <>
          <div className="muted" style={{ fontSize: 11.5, textAlign: "center", marginBottom: 9, display: "flex", gap: 6, justifyContent: "center", alignItems: "center" }}>
            <Icon name="lock" size={13} />{t("lockfull_body")}
          </div>
          <button className="btn btn-gold" style={{ width: "100%" }} onClick={() => setLockSheet(true)} disabled={lockFull.isPending}>
            <Icon name="lock" size={17} /> {t("lockfull_cta")} · {usdc0(auction.floorCents)}
          </button>
        </>
      ) : (
        <>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 8, marginBottom: 9 }}>
            <Chip state="active" label={t("full_locked") || "FULL_LOCKED"} /><span className="mono" style={{ fontSize: 11, color: "var(--fg-muted)" }}>{usdc0(auction.floorCents)}</span>
          </div>
          <button className="btn btn-gold buypulse" style={{ width: "100%", fontSize: 17, padding: "16px" }} onClick={onBuy} disabled={buyNow.isPending}>
            {t("auc_buy")} {dollars(engine.priceCents)}
          </button>
        </>
      )}
    </div>
  );

  return (
    <>
      <ScreenShell
        bg="var(--bg-void)"
        top={
          <TopBar kicker={`${t("gal_lot")} ${String(seed).padStart(2, "0")}`} title={maison} onBack={() => nav(-1)} right={<span className="chip" data-st="live"><span className="dot" />{t("auc_live")}</span>} />
        }
        footer={footer}
      >
        <div style={{ position: "relative", minHeight: "100%" }}>
          {room.toast && (
            <div key={room.toast.id} className="fade-up" style={{ position: "absolute", top: 14, insetInlineStart: 16, insetInlineEnd: 16, zIndex: 20, display: "flex", alignItems: "center", gap: 8, padding: "8px 12px", borderRadius: "var(--r-pill)", background: "rgba(20,12,16,0.92)", border: "1px solid var(--gold-line)", width: "fit-content" }}>
              <span className="dot" style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--gold)" }} />
              <span className="mono" style={{ fontSize: 11.5, color: "var(--fg)" }}>{room.toast.txt}</span>
            </div>
          )}

          {/* STAGE layout — central price + heat ring */}
          <div style={{ padding: "22px 20px 0", display: "flex", flexDirection: "column", alignItems: "center", textAlign: "center" }}>
            <div style={{ position: "relative", width: 200, height: 200, marginBottom: 18 }}>
              <Ph art={art} ratio="1 / 1" label={maison} style={{ borderRadius: "var(--r-3)", width: "100%", height: "100%" }} />
              <div style={{ position: "absolute", inset: -10, display: "flex", alignItems: "center", justifyContent: "center", pointerEvents: "none" }}>
                <HeatRing nextIn={engine.nextInSec} every={engine.intervalSec} size={224} atFloor={engine.atFloor} />
              </div>
            </div>
            <div className="serif" style={{ fontSize: 19, color: "var(--gold-pale)", lineHeight: 1.2, marginBottom: lotData?.certified ? 12 : 18, maxWidth: 280 }}>{title}</div>
            {lotData?.certified && <div style={{ marginBottom: 18 }}><CertBadge /></div>}

            <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginBottom: 6 }}>{t("auc_current")}</div>
            <PriceBig cents={engine.priceCents} flash={flash} size={52} />
            <div style={{ display: "flex", gap: 18, marginTop: 14, marginBottom: 16 }}>
              <MiniStat label={t("auc_floor")} value={dollars(auction.floorCents)} />
              <MiniStat label={t("auc_next")} value={engine.atFloor ? "—" : `${Math.ceil(engine.nextInSec)}${t("auc_secs")}`} gold />
            </div>
            {RoomBar}
          </div>
          <div style={{ padding: "22px 20px 30px" }}><AuctionStatusGuide /></div>
        </div>
      </ScreenShell>

      <Sheet open={lockSheet} onClose={() => setLockSheet(false)}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 6 }}>
          <span style={{ color: "var(--gold)" }}><Icon name="lock" size={20} /></span>
          <div className="serif" style={{ fontSize: 20, color: "var(--gold-pale)" }}>{t("lockfull_title")}</div>
        </div>
        <p className="muted" style={{ fontSize: 13, lineHeight: 1.6, margin: "0 0 18px" }}>{t("lockfull_body")}</p>
        <div className="kv" style={{ marginBottom: 20 }}>
          <div className="k">{t("req_deposit")} · 10%</div><div className="v" style={{ color: "var(--st-good)" }}>✓ {usdc(Math.round(auction.floorCents * 0.1))}</div>
          <div className="k">{t("lockfull_title")} · 100%</div><div className="v" style={{ color: "var(--gold-pale)" }}>{usdc(auction.floorCents)}</div>
        </div>
        <button
          className="btn btn-gold"
          style={{ width: "100%" }}
          disabled={lockFull.isPending}
          onClick={async () => { await lockFull.mutateAsync(); setLocked(true); setLockSheet(false); }}
        >
          <Icon name="lock" size={17} /> {t("lockfull_cta")} · {usdc0(auction.floorCents)}
        </button>
      </Sheet>
    </>
  );
}

function WonView({ maison, title, hammerCents, onBack, onContinue }: {
  maison: string; title: string; hammerCents: number; onBack: () => void; onContinue: () => void;
}) {
  const { t } = useI18n();
  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column", padding: "0 24px", background: "radial-gradient(120% 70% at 50% 0%, var(--burg), var(--bg-void) 65%)" }}>
      <TopBar title={maison} onBack={onBack} />
      <div style={{ flex: 1, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", textAlign: "center", gap: 6 }}>
        <div className="stampin"><Seal size={120} label="HAMMER" sub="DAUCTION" id="0x7A4E" date="WON" live /></div>
        <div className="mono up" style={{ fontSize: 11, color: "var(--gold)", letterSpacing: "0.2em", marginTop: 14 }}>{t("auc_won")}</div>
        <div className="serif" style={{ fontSize: 22, color: "var(--gold-pale)", lineHeight: 1.2, maxWidth: 280 }}>{title}</div>
        <div style={{ marginTop: 14 }}>
          <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)", marginBottom: 6 }}>{t("esc_hammer")}</div>
          <PriceBig cents={hammerCents} flash={false} size={42} />
        </div>
      </div>
      <div className="safe-bottom" style={{ paddingBottom: 32 }}>
        <button className="btn btn-gold" style={{ width: "100%" }} onClick={onContinue}>{t("esc_complete_title")} <Icon name="arrow-right" size={18} /></button>
      </div>
    </div>
  );
}
