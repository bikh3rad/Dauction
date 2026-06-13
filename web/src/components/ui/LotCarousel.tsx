import { useCallback, useEffect, useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { Icon } from "@/components/ui/Icon";
import { Ph } from "@/components/ui/ProductArt";

export const MAX_LOT_IMAGES = 7;

// Per-slide framing so placeholder slides read as distinct shots of one item
// (front / detail / angle …) when a listing has no real photos yet.
const FRAMING: Array<{ artW: string; artTop: string }> = [
  { artW: "58%", artTop: "30%" },
  { artW: "82%", artTop: "20%" },
  { artW: "44%", artTop: "40%" },
  { artW: "70%", artTop: "26%" },
  { artW: "52%", artTop: "44%" },
  { artW: "90%", artTop: "16%" },
  { artW: "64%", artTop: "34%" },
];

export interface LotCarouselProps {
  /** Real image URLs/keys (≤7). When empty, placeholder art slides are shown. */
  images?: string[];
  /** Category art key for placeholder slides (e.g. "watch"). */
  art: string;
  /** Accessible label / maison name. */
  label?: string;
}

// LotCarousel renders up to 7 fully-browsable images for an auction detail page:
// a main stage, prev/next controls (RTL-aware), a thumbnail strip, dot indicators
// and an n/7 counter, with arrow-key navigation. Enforces the ≤7 cap on display.
export function LotCarousel({ images, art, label }: LotCarouselProps) {
  const { dir, t } = useI18n();
  const real = (images ?? []).slice(0, MAX_LOT_IMAGES);
  const count = real.length > 0 ? real.length : FRAMING.length; // placeholder = 7
  const [i, setI] = useState(0);
  const [failed, setFailed] = useState<Record<number, boolean>>({});

  const go = useCallback(
    (delta: number) => setI((p) => (p + delta + count) % count),
    [count],
  );

  // Keyboard arrows; in RTL the visual direction is mirrored.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight") go(dir === "rtl" ? -1 : 1);
      if (e.key === "ArrowLeft") go(dir === "rtl" ? 1 : -1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [go, dir]);

  // A real photo when present and loadable; otherwise the category line-art so a
  // broken/missing URL never leaves an empty slide.
  const slide = (idx: number) =>
    real.length > 0 && !failed[idx] ? (
      <img
        src={real[idx]}
        alt={`${label ?? ""} ${idx + 1}/${count}`}
        onError={() => setFailed((f) => ({ ...f, [idx]: true }))}
        style={{ width: "100%", height: "100%", objectFit: "cover", display: "block", aspectRatio: "1 / 1" }}
      />
    ) : (
      <Ph art={art} artW={FRAMING[idx % FRAMING.length].artW} artTop={FRAMING[idx % FRAMING.length].artTop} ratio="1 / 1" style={{ borderRadius: 0, borderInline: 0 }} />
    );

  const prevIcon = dir === "rtl" ? "chevron-right" : "chevron-left";
  const nextIcon = dir === "rtl" ? "chevron-left" : "chevron-right";

  return (
    <div>
      {/* Main stage */}
      <div style={{ position: "relative", overflow: "hidden" }}>
        {slide(i)}

        {/* Authenticated seal (inspector gate result) */}
        <div style={{ position: "absolute", top: 14, insetInlineStart: 14 }}>
          <span className="chip" data-st="good"><Icon name="check" size={12} /> {t("lot_authentic") || "Authenticated"}</span>
        </div>

        {/* Counter */}
        <div style={{ position: "absolute", top: 14, insetInlineEnd: 14, background: "rgba(12,8,9,0.7)", borderRadius: "var(--r-pill)", padding: "3px 9px", fontSize: 11 }} className="mono">
          {i + 1} / {count}
        </div>

        {count > 1 && (
          <>
            <button aria-label="previous image" onClick={() => go(-1)} style={navBtn("start")}>
              <Icon name={prevIcon} size={20} />
            </button>
            <button aria-label="next image" onClick={() => go(1)} style={navBtn("end")}>
              <Icon name={nextIcon} size={20} />
            </button>
          </>
        )}

        {/* Dots */}
        <div style={{ position: "absolute", insetInlineStart: 0, insetInlineEnd: 0, bottom: 12, display: "flex", justifyContent: "center", gap: 6 }}>
          {Array.from({ length: count }).map((_, d) => (
            <span key={d} style={{ width: d === i ? 18 : 6, height: 6, borderRadius: 3, background: d === i ? "var(--gold)" : "rgba(255,255,255,0.4)", transition: "width .2s" }} />
          ))}
        </div>
      </div>

      {/* Thumbnail strip */}
      {count > 1 && (
        <div style={{ display: "flex", gap: 8, padding: "10px 16px", overflowX: "auto" }}>
          {Array.from({ length: count }).map((_, t2) => (
            <button
              key={t2}
              onClick={() => setI(t2)}
              aria-label={`image ${t2 + 1}`}
              style={{
                flex: "0 0 auto", width: 54, height: 54, borderRadius: "var(--r-1)", overflow: "hidden", cursor: "pointer", padding: 0,
                border: t2 === i ? "2px solid var(--gold)" : "1px solid var(--line)", background: "var(--bg-1)",
              }}
            >
              {slide(t2)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function navBtn(side: "start" | "end"): React.CSSProperties {
  return {
    position: "absolute",
    top: "50%",
    transform: "translateY(-50%)",
    [side === "start" ? "insetInlineStart" : "insetInlineEnd"]: 10,
    width: 38, height: 38, borderRadius: "50%",
    background: "rgba(12,8,9,0.6)", color: "var(--gold-pale)",
    border: "1px solid var(--gold-line)", cursor: "pointer",
    display: "flex", alignItems: "center", justifyContent: "center",
  } as React.CSSProperties;
}
