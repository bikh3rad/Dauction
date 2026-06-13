import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { Chip } from "@/components/ui/Chip";
import { Icon } from "@/components/ui/Icon";
import { CategoryIcon } from "@/components/ui/CategoryIcon";
import { Money } from "@/components/ui/Money";
import { LotImage } from "@/components/ui/LotImage";
import { categoryLabel } from "@/lib/enrich";
import { toLotView } from "@/lib/lotView";
import type { Lot } from "@/types";

function priceCents(lot: Lot, live: boolean): number {
  // live shows a current-ish price (ceiling already descended a little); else floor.
  if (live) return Math.round(lot.appraisedValueCents * 0.92);
  return lot.reserveCents;
}

function Meta({ lot }: { lot: Lot }) {
  const { t, lang } = useI18n();
  const v = toLotView(lot);
  const seq = Number(lot.id.replace(/\D/g, "")) || 0;
  return (
    <div className="mono up" style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 9, color: "var(--gold)", letterSpacing: "0.14em" }}>
      <span style={{ display: "inline-flex", color: "var(--gold-pale)" }}><CategoryIcon category={v.category} size={12} /></span>
      {t("gal_lot")} {String(seq).padStart(2, "0")} · {categoryLabel(v.category, lang)}
    </div>
  );
}

// A round category badge overlaid on a card image so the type reads at a glance.
function CatBadge({ category }: { category: Parameters<typeof CategoryIcon>[0]["category"] }) {
  return (
    <div style={{ position: "absolute", top: 12, insetInlineEnd: 12, width: 32, height: 32, borderRadius: "50%", background: "rgba(12,8,9,0.66)", border: "1px solid var(--gold-line)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--gold-pale)", backdropFilter: "blur(4px)" }}>
      <CategoryIcon category={category} size={18} />
    </div>
  );
}

// Magazine (hero) card — the editorial default.
export function LotCardMagazine({ lot, watching }: { lot: Lot; watching: number }) {
  const { t, dir } = useI18n();
  const nav = useNavigate();
  const v = toLotView(lot);
  return (
    <button
      onClick={() => nav(`/${v.route}/${lot.id}`)}
      className="fade-up"
      style={{ textAlign: "start", background: "var(--bg-1)", border: "1px solid var(--line)", borderRadius: "var(--r-3)", overflow: "hidden", cursor: "pointer", color: "var(--fg)", padding: 0, width: "100%" }}
    >
      <div style={{ position: "relative" }}>
        <LotImage src={lot.imageRefs?.[0]} art={v.art} artW="54%" artTop="38%" ratio="4 / 3" label={v.maison} />
        <CatBadge category={v.category} />
        <div style={{ position: "absolute", top: 12, insetInlineStart: 12 }}>
          {v.isLive ? (
            <Chip state="live" label={t("auc_live")} pulse />
          ) : v.isPassive ? (
            <span className="chip" data-st="warn"><Icon name="clock" size={11} /> {t(lot.atype === "VICKREY" ? "mode_vickrey" : "mode_uniqbid")}</span>
          ) : (
            <Chip state="proposed" label={t("gal_upcoming").toUpperCase()} />
          )}
        </div>
        <div style={{ position: "absolute", insetInlineStart: 0, insetInlineEnd: 0, bottom: 0, height: "62%", background: "linear-gradient(to top, var(--bg-1) 6%, rgba(12,8,9,0.82) 34%, transparent 100%)" }} />
        <div style={{ position: "absolute", bottom: 14, insetInlineStart: 14, insetInlineEnd: 14 }}>
          <Meta lot={lot} />
          <div className="serif" style={{ fontSize: 20, lineHeight: 1.2, margin: "5px 0 0", color: "var(--gold-pale)", display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>{v.body}</div>
          <div className="mono up" style={{ fontSize: 9, color: "var(--fg-muted)", marginTop: 5, letterSpacing: "0.1em" }}>{v.maison}</div>
        </div>
      </div>
      <div style={{ padding: "14px 16px", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{v.isLive ? t("auc_current") : v.isPassive ? t("list_floor") : t("gal_floor")}</div>
          <div style={{ marginTop: 3 }}><Money cents={priceCents(lot, v.isLive)} big gold withCents={false} /></div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 5, color: "var(--fg-muted)", fontSize: 12 }}>
          <Icon name="eye" size={14} /><span className="mono">{watching}</span>
          <span style={{ color: "var(--gold)", marginInlineStart: 6, display: "inline-flex" }}><Icon name={dir === "rtl" ? "arrow-left" : "arrow-right"} size={16} /></span>
        </div>
      </div>
    </button>
  );
}

// Compact grid card.
export function LotCardGrid({ lot, watching }: { lot: Lot; watching: number }) {
  const { t } = useI18n();
  const nav = useNavigate();
  const v = toLotView(lot);
  return (
    <button
      onClick={() => nav(`/${v.route}/${lot.id}`)}
      className="fade-up"
      style={{ textAlign: "start", background: "var(--bg-1)", border: "1px solid var(--line)", borderRadius: "var(--r-2)", overflow: "hidden", cursor: "pointer", color: "var(--fg)", padding: 0, width: "100%" }}
    >
      <div style={{ position: "relative" }}>
        <LotImage src={lot.imageRefs?.[0]} art={v.art} ratio="1 / 1" label={v.maison} />
        <CatBadge category={v.category} />
        {v.isLive && <div style={{ position: "absolute", top: 8, insetInlineStart: 8 }}><Chip state="live" label={t("auc_live")} pulse /></div>}
        {v.isPassive && <div style={{ position: "absolute", top: 8, insetInlineStart: 8 }}><span className="chip" data-st="warn"><Icon name="clock" size={11} /> {t(lot.atype === "VICKREY" ? "mode_vickrey" : "mode_uniqbid")}</span></div>}
        <div style={{ position: "absolute", bottom: 8, insetInlineEnd: 8, display: "flex", alignItems: "center", gap: 4, background: "rgba(12,8,9,0.7)", padding: "3px 7px", borderRadius: "var(--r-pill)", fontSize: 10, color: "var(--fg-muted)" }}>
          <Icon name="eye" size={12} /><span className="mono">{watching}</span>
        </div>
      </div>
      <div style={{ padding: "10px 11px 12px" }}>
        <Meta lot={lot} />
        <div className="serif" style={{ fontSize: 14, lineHeight: 1.2, margin: "5px 0 8px", color: "var(--fg)", display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>{v.body}</div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
          <span className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{v.isLive ? t("auc_current") : v.isPassive ? t("list_floor") : t("gal_floor")}</span>
          <Money cents={priceCents(lot, v.isLive)} gold withCents={false} />
        </div>
      </div>
    </button>
  );
}
