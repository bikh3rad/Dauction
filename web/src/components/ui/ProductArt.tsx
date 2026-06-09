import type { CSSProperties, ReactNode } from "react";

// Editorial luxury line-art per category — gold linework over a tonal vault
// backdrop, standing in for real photography (drop a photo in later to replace).
const ART: Record<string, ReactNode> = {
  watch: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <path d="M82 20 L118 20 L114 58 L86 58 Z" fill="currentColor" fillOpacity="0.06" />
      <path d="M86 142 L114 142 L118 180 L82 180 Z" fill="currentColor" fillOpacity="0.06" />
      <circle cx="100" cy="100" r="47" fill="currentColor" fillOpacity="0.05" />
      <circle cx="100" cy="100" r="47" />
      <circle cx="100" cy="100" r="39" strokeWidth="1.3" strokeOpacity="0.7" />
      <path d="M148 90 L156 90 L156 110 L148 110" />
      <path d="M100 100 L100 70" />
      <path d="M100 100 L124 110" strokeWidth="1.8" />
      <circle cx="100" cy="129" r="8" strokeWidth="1.3" strokeOpacity="0.7" />
      <path d="M100 60 L100 66 M140 100 L134 100 M100 140 L100 134 M60 100 L66 100" strokeWidth="1.6" />
    </g>
  ),
  bag: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <path d="M56 94 L144 94 L137 168 L63 168 Z" fill="currentColor" fillOpacity="0.06" />
      <path d="M56 94 C56 80 66 70 78 70 L122 70 C134 70 144 80 144 94" />
      <path d="M62 94 L62 78 M138 94 L138 78" strokeOpacity="0.6" strokeWidth="1.4" />
      <path d="M76 80 C76 56 96 56 96 80" />
      <path d="M104 80 C104 56 124 56 124 80" />
      <rect x="92" y="92" width="16" height="20" rx="2" fill="currentColor" fillOpacity="0.12" />
      <path d="M100 112 L100 124" strokeWidth="1.6" />
      <circle cx="100" cy="128" r="4" strokeWidth="1.6" />
    </g>
  ),
  bottle: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <rect x="86" y="24" width="28" height="22" rx="2" fill="currentColor" fillOpacity="0.1" />
      <path d="M94 46 L94 56 L106 56 L106 46" />
      <path d="M78 78 C78 64 86 56 100 56 C114 56 122 64 122 78 L122 162 C122 172 116 178 106 178 L94 178 C84 178 78 172 78 162 Z" fill="currentColor" fillOpacity="0.05" />
      <path d="M78 120 L122 120 L122 162 C122 172 116 178 106 178 L94 178 C84 178 78 172 78 162 Z" fill="currentColor" fillOpacity="0.1" stroke="none" />
      <path d="M88 100 L112 100" strokeWidth="1.3" strokeOpacity="0.7" />
      <path d="M88 110 L112 110" strokeWidth="1.3" strokeOpacity="0.7" />
      <path d="M82 70 L96 60 M118 70 L104 60" strokeWidth="1.2" strokeOpacity="0.5" />
    </g>
  ),
  sneaker: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <path d="M26 150 Q26 164 40 164 L150 164 Q164 164 160 150 L154 142 L32 142 Z" fill="currentColor" fillOpacity="0.08" />
      <path d="M32 142 C34 116 52 102 86 102 L116 104 C132 106 142 124 146 142" fill="currentColor" fillOpacity="0.05" />
      <path d="M68 138 C90 116 116 112 142 120" strokeWidth="2" />
      <path d="M88 104 L84 124 M100 105 L96 126 M112 107 L108 128" strokeWidth="1.5" strokeOpacity="0.7" />
      <path d="M116 104 C126 98 138 104 142 116" />
      <path d="M30 156 L156 156" strokeWidth="1.3" strokeOpacity="0.6" />
    </g>
  ),
  frame: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <rect x="40" y="38" width="120" height="124" fill="currentColor" fillOpacity="0.04" />
      <rect x="40" y="38" width="120" height="124" />
      <rect x="52" y="50" width="96" height="100" strokeWidth="1.4" strokeOpacity="0.7" />
      <circle cx="118" cy="78" r="14" fill="currentColor" fillOpacity="0.14" stroke="none" />
      <circle cx="118" cy="78" r="14" strokeWidth="1.6" />
      <path d="M58 138 C78 104 96 150 118 116 C130 98 140 120 142 132" strokeWidth="2" />
      <path d="M58 124 L84 96" strokeWidth="1.4" strokeOpacity="0.6" />
    </g>
  ),
  painting: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <path d="M58 168 L74 58 M142 168 L126 58 M80 150 L120 150" strokeWidth="1.6" strokeOpacity="0.55" />
      <path d="M100 60 L100 44" strokeWidth="1.6" strokeOpacity="0.55" />
      <rect x="46" y="40" width="108" height="96" fill="currentColor" fillOpacity="0.05" />
      <rect x="46" y="40" width="108" height="96" />
      <rect x="54" y="48" width="92" height="80" strokeWidth="1.2" strokeOpacity="0.7" />
      <path d="M46 40 L54 48 M154 40 L146 48 M46 136 L54 128 M154 136 L146 128" strokeWidth="1.2" strokeOpacity="0.6" />
      <circle cx="124" cy="68" r="9" fill="currentColor" fillOpacity="0.16" stroke="none" />
      <circle cx="124" cy="68" r="9" strokeWidth="1.4" />
      <path d="M54 112 C72 88 90 116 110 96 C124 82 138 100 146 110" strokeWidth="2" />
      <path d="M54 122 L146 122" strokeWidth="1.2" strokeOpacity="0.5" />
    </g>
  ),
  ring: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <ellipse cx="100" cy="132" rx="42" ry="30" fill="currentColor" fillOpacity="0.04" />
      <ellipse cx="100" cy="132" rx="42" ry="30" />
      <ellipse cx="100" cy="132" rx="33" ry="22" strokeWidth="1.4" strokeOpacity="0.6" />
      <path d="M80 86 L100 60 L120 86 L100 104 Z" fill="currentColor" fillOpacity="0.16" />
      <path d="M80 86 L100 60 L120 86 L100 104 Z" />
      <path d="M80 86 L120 86 M100 60 L100 104 M88 73 L100 86 L112 73" strokeWidth="1.3" strokeOpacity="0.7" />
      <path d="M84 100 L92 110 M116 100 L108 110" strokeWidth="1.6" />
    </g>
  ),
  gem: (
    <g fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinejoin="round" strokeLinecap="round">
      <path d="M60 78 L140 78 L100 156 Z" fill="currentColor" fillOpacity="0.1" />
      <path d="M60 78 L140 78 L100 156 Z" />
      <path d="M60 78 L80 56 L120 56 L140 78 M80 56 L100 78 L120 56 M60 78 L100 78 L140 78 M82 78 L100 156 M118 78 L100 156" strokeWidth="1.3" strokeOpacity="0.7" />
    </g>
  ),
};

export function ProductArt({ cat, w = "74%", top = "50%" }: { cat: string; w?: string; top?: string }) {
  return (
    <svg
      viewBox="0 0 200 200"
      preserveAspectRatio="xMidYMid meet"
      aria-hidden="true"
      style={{
        position: "absolute", top, left: "50%", transform: "translate(-50%,-50%)",
        width: w, height: w, color: "var(--gold)",
        filter: "drop-shadow(0 6px 16px rgba(0,0,0,0.4))",
      }}
    >
      {ART[cat] ?? ART.gem}
    </svg>
  );
}

// Image tile — renders the category line-art; swap in a real photo later.
export interface PhProps {
  label?: string;
  art?: string;
  artW?: string;
  artTop?: string;
  ratio?: string | null;
  style?: CSSProperties;
  children?: ReactNode;
}
export function Ph({ label, art, artW, artTop, ratio, style, children }: PhProps) {
  const isArt = !!art;
  return (
    <div
      className={"ph" + (isArt ? " ph-art" : "")}
      style={{
        aspectRatio: ratio === null ? undefined : ratio || "1 / 1",
        ...(isArt ? { background: "radial-gradient(120% 100% at 50% 16%, var(--bg-3), var(--bg-0) 80%)" } : {}),
        ...style,
      }}
    >
      {isArt && <ProductArt cat={art!} w={artW} top={artTop} />}
      {label &&
        (isArt ? (
          <>
            <div style={{ position: "absolute", inset: 0, background: "linear-gradient(to top, rgba(12,8,9,0.55), transparent 42%)" }} />
            <div className="mono up" style={{ position: "absolute", insetInlineStart: 11, bottom: 9, zIndex: 2, fontSize: 9, letterSpacing: "0.14em", color: "var(--gold-pale)" }}>{label}</div>
          </>
        ) : (
          <div className="ph-label" style={{ position: "relative", zIndex: 1 }}>{label}</div>
        ))}
      {children}
    </div>
  );
}
