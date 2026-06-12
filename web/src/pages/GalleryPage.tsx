import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useGallery } from "@/hooks/queries";
import { useSession } from "@/hooks/useSession";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Icon } from "@/components/ui/Icon";
import { LoadingScreen, ErrorState, EmptyState } from "@/components/ui/States";
import { LotCardGrid, LotCardMagazine } from "@/components/LotCard";
import { toLotView } from "@/lib/lotView";
import type { Lot } from "@/types";

// deterministic "watching" count so a lot always shows the same number.
function watchersOf(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return 90 + (h % 410);
}

type Filter = "all" | "live" | "upcoming";

export function GalleryPage() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { isGuest } = useSession();
  const { data, isLoading, isError, refetch } = useGallery();
  const [filter, setFilter] = useState<Filter>("all");
  const [mag, setMag] = useState(true);

  const lots = data?.lots ?? [];
  const shown = useMemo(() => {
    // Live first: an open auction you can participate in (Dutch live or a timed
    // passive auction accepting bids) sorts to the top of the list; upcoming
    // (scheduled) lots follow, soonest first.
    const rank = (v: ReturnType<typeof toLotView>) => (v.isLive ? 0 : v.isPassive ? 1 : 2);
    return lots
      .map((l) => ({ l, v: toLotView(l) }))
      .filter(({ v }) => (filter === "live" ? v.isLive || v.isPassive : filter === "upcoming" ? !v.isLive && !v.isPassive : true))
      .sort((a, b) => rank(a.v) - rank(b.v) || a.v.opensInSec - b.v.opensInSec)
      .map(({ l }) => l);
  }, [lots, filter]);

  const supplyCap = data?.supplyCap ?? 32;
  const admitted = lots.length;
  const pct = Math.round((admitted / supplyCap) * 100);

  return (
    <ScreenShell top={<TopBar kicker={t("gal_kicker")} title={t("gal_title")} right={<LangPill />} />}>
      {isGuest && (
        <button
          onClick={() => nav("/login")}
          style={{ width: "calc(100% - 32px)", margin: "14px 16px 0", cursor: "pointer", textAlign: "start", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "linear-gradient(110deg,var(--burg),var(--bg-1) 80%)", padding: "12px 14px", display: "flex", alignItems: "center", gap: 12, color: "var(--fg)" }}
        >
          <span style={{ color: "var(--gold)" }}><Icon name="crown" size={20} /></span>
          <span style={{ flex: 1, fontSize: 12.5, lineHeight: 1.45 }}>{t("guest_banner")}</span>
          <span className="mono up" style={{ fontSize: 10, color: "var(--gold-pale)", whiteSpace: "nowrap" }}>{t("enter_invite")}</span>
        </button>
      )}

      {/* supply scarcity banner */}
      <div style={{ margin: "14px 16px 4px", padding: "12px 14px", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "linear-gradient(90deg,var(--burg-deep),transparent)", display: "flex", alignItems: "center", gap: 12 }}>
        <div className="serif" style={{ fontSize: 30, color: "var(--gold-pale)", lineHeight: 1 }}>{supplyCap}</div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 12.5, color: "var(--fg)" }}>{t("gal_supply")}</div>
          <div style={{ height: 5, background: "var(--bg-0)", borderRadius: 3, marginTop: 7, overflow: "hidden" }}>
            <div style={{ width: `${pct}%`, height: "100%", background: "var(--gold)" }} />
          </div>
        </div>
        <div className="mono" style={{ fontSize: 11, color: "var(--gold)" }}>{String(admitted).padStart(2, "0")} / {supplyCap}</div>
      </div>

      {/* filters + layout toggle */}
      <div style={{ display: "flex", gap: 8, padding: "12px 16px 4px", alignItems: "center" }}>
        <div style={{ display: "flex", gap: 8, overflowX: "auto", flex: 1 }} className="noscroll">
          {([["all", t("gal_all")], ["live", t("gal_live")], ["upcoming", t("gal_upcoming")]] as const).map(([k, lbl]) => (
            <button
              key={k}
              onClick={() => setFilter(k)}
              className="mono"
              style={{ cursor: "pointer", padding: "7px 14px", borderRadius: "var(--r-pill)", fontSize: 11.5, letterSpacing: "0.06em", whiteSpace: "nowrap", border: "1px solid", borderColor: filter === k ? "var(--gold)" : "var(--line-strong)", background: filter === k ? "var(--gold)" : "transparent", color: filter === k ? "#1B1207" : "var(--fg-muted)", fontWeight: 600 }}
            >
              {k === "live" && <span style={{ display: "inline-block", width: 6, height: 6, borderRadius: "50%", background: "currentColor", marginInlineEnd: 6 }} />}
              {lbl}
            </button>
          ))}
        </div>
        <button onClick={() => setMag((m) => !m)} aria-label="layout" className="iconbtn">
          <Icon name={mag ? "grid" : "image"} size={18} />
        </button>
      </div>

      {isLoading ? (
        <LoadingScreen />
      ) : isError ? (
        <ErrorState onRetry={() => refetch()} />
      ) : shown.length === 0 ? (
        <EmptyState label={t("gal_upcoming")} />
      ) : (
        <div className={mag ? "resp-grid mag" : "resp-grid"} style={{ padding: "8px 16px 24px", display: "grid", gridTemplateColumns: mag ? "1fr" : "1fr 1fr", gap: mag ? 18 : 12 }}>
          {shown.map((l: Lot) =>
            mag ? (
              <LotCardMagazine key={l.id} lot={l} watching={watchersOf(l.id)} />
            ) : (
              <LotCardGrid key={l.id} lot={l} watching={watchersOf(l.id)} />
            ),
          )}
        </div>
      )}
    </ScreenShell>
  );
}
