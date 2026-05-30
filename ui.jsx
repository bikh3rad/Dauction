/* ============================================================
   DAUCTION — shared UI primitives  (→ window)
   Inline-SVG icons (Lucide-style, 2px), octagonal Seal, category
   glyphs, StateChip, Money, ImagePlaceholder, small hooks.
   ============================================================ */
const { useState, useEffect, useRef, useCallback } = React;

/* ---------- ICONS (curated, Lucide-style 24-grid, 2px stroke) ---------- */
const PATHS = {
  "chevron-left":  "M15 18l-6-6 6-6",
  "chevron-right": "M9 18l6-6-6-6",
  "arrow-right":   "M5 12h14 M13 6l6 6-6 6",
  "arrow-left":    "M19 12H5 M11 6l-6 6 6 6",
  lock:    "M5 11h14v10H5z M8 11V7a4 4 0 0 1 8 0v4",
  clock:   "M12 7v5l3 2 M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
  eye:     "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z",
  users:   "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M22 21v-2a4 4 0 0 0-3-3.87 M16 3.13A4 4 0 0 1 16 11",
  check:   "M20 6L9 17l-5-5",
  x:       "M18 6L6 18 M6 6l12 12",
  shield:  "M12 3l8 4v5c0 5-3.5 8-8 9-4.5-1-8-4-8-9V7z",
  gavel:   "M14 13l-7.5 7.5a2.1 2.1 0 0 1-3-3L11 10 M16 16l5-5 M3 21h7 M12.5 6.5l5 5 3-3-5-5z",
  store:   "M3 9l1.5-5h15L21 9 M4 9v11h16V9 M3 9h18 M9 20v-6h6v6",
  user:    "M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2 M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z",
  coins:   "M9 14a6 6 0 1 0 0-12 6 6 0 0 0 0 12z M16.5 9.4A6 6 0 1 1 9.5 21",
  file:    "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z M14 3v5h5 M9 13h6 M9 17h6",
  hash:    "M4 9h16 M4 15h16 M10 3L8 21 M16 3l-2 18",
  plus:    "M12 5v14 M5 12h14",
  search:  "M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16z M21 21l-4.3-4.3",
  bell:    "M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9 M13.7 21a2 2 0 0 1-3.4 0",
  grid:    "M3 3h7v7H3z M14 3h7v7h-7z M14 14h7v7h-7z M3 14h7v7H3z",
  layers:  "M12 2l9 5-9 5-9-5 9-5z M3 12l9 5 9-5 M3 17l9 5 9-5",
  image:   "M3 3h18v18H3z M8.5 10a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3z M21 15l-5-5L5 21",
  upload:  "M12 15V3 M7 8l5-5 5 5 M5 17v2a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-2",
  package: "M21 8l-9-5-9 5 9 5 9-5z M3 8v8l9 5 9-5V8 M12 13v8",
  watch:   "M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M9 4l.5-2h5L15 4 M9 20l.5 2h5l.5-2",
  shoe:    "M2 17h17a3 3 0 0 0 3-3l-7-3-3-4H4z M2 17v2h20v-2 M9 8l1 2 M12 9l1 2",
  bottle:  "M10 2h4v3l1 3v13H9V8l1-3z M9 12h6",
  frame:   "M3 3h18v18H3z M7 7h10v10H7z",
  gem:     "M6 3h12l3 6-9 12L3 9z M3 9h18 M9 3L6 9l6 12 6-12-3-6",
  refresh: "M21 12a9 9 0 1 1-3-6.7L21 8 M21 3v5h-5",
  alert:   "M12 9v4 M12 17h.01 M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z",
  crown:   "M3 7l4 5 5-7 5 7 4-5v11H3z M3 18h18",
  scale:   "M12 3v18 M7 21h10 M5 7h14 M5 7l-3 7h6zM19 7l-3 7h6z",
  dollar:  "M12 2v20 M17 6.5a4 4 0 0 0-4-2.5H11a3.5 3.5 0 0 0 0 7h2a3.5 3.5 0 0 1 0 7h-2a4 4 0 0 1-4-2.5",
  flame:   "M12 22c4 0 7-2.5 7-6.5 0-3-2-5-3-6.5-.7 1-1.5 1.5-2.5 1.5 0-2-1-4.5-3-6 0 3-2 4-3.5 6C3 12 3 14 5 17a7 7 0 0 0 7 5z",
  menu:    "M3 6h18 M3 12h18 M3 18h18",
};
function Icon({ name, size = 20, stroke = 2, fill = false, style, className }) {
  const d = PATHS[name];
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round"
      style={{ flexShrink: 0, display: "block", ...style }} className={className} aria-hidden="true">
      {d.split("M").filter(Boolean).map((seg, i) => <path key={i} d={"M" + seg} />)}
    </svg>
  );
}

/* ---------- Octagonal attestation Seal (from the TRADE stamp glyph) ---------- */
function Seal({ size = 72, label = "CERTIFIED", sub = "DAUCTION", id = "0xA1", date = "2026·04", live = false }) {
  return (
    <svg width={size} height={size} viewBox="0 0 96 96" fill="none"
      style={{ color: "var(--gold)", filter: live ? "drop-shadow(0 0 6px var(--gold-glow))" : "none" }} aria-hidden="true">
      <polygon points="28,8 68,8 88,28 88,68 68,88 28,88 8,68 8,28" stroke="currentColor" strokeWidth="2.5" fill="none" />
      <polygon points="32,14 64,14 82,32 82,64 64,82 32,82 14,64 14,32" stroke="currentColor" strokeWidth="1" fill="none" />
      <text x="48" y="43" textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontWeight="600" fontSize="10" letterSpacing="2" fill="currentColor">{label}</text>
      <line x1="22" y1="50" x2="74" y2="50" stroke="currentColor" strokeWidth="0.8" />
      <text x="48" y="63" textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontWeight="500" fontSize="7.5" letterSpacing="1.5" fill="currentColor" opacity="0.8">{sub} · {id}</text>
      <text x="48" y="75" textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontWeight="500" fontSize="7.5" letterSpacing="1.5" fill="currentColor" opacity="0.7">{date}</text>
    </svg>
  );
}

/* ---------- Category glyph (decorative line-art over a vault tile) ---------- */
function CatGlyph({ g, size = 48 }) {
  const key = { watch:"watch", shoe:"shoe", bottle:"bottle", frame:"frame", gem:"gem" }[g] || "package";
  return <Icon name={key} size={size} stroke={1.4} style={{ color: "var(--gold-line)" }} />;
}

/* ---------- State chip (mono uppercase stamp) ---------- */
const STATE_MAP = {
  in_closet:"neut", proposed:"neut", appraising:"warn", in_auction:"active",
  live:"live", funded:"active", in_transit:"active", delivered:"active",
  completed:"good", disputed:"bad", certified:"good", sold:"neut",
  active:"good", redeemed:"neut", flagged:"bad", pending:"warn", approved:"good",
};
function Chip({ state, label, pulse, children }) {
  const st = STATE_MAP[state] || "neut";
  const txt = label || (state ? state.toUpperCase().replace(/ /g, "_") : children);
  return (
    <span className={"chip" + (pulse ? " live-pulse" : "")} data-st={st}>
      {pulse && <span className="dot" />}{txt || children}
    </span>
  );
}

/* ---------- Money ---------- */
function Money({ n, big, gold, cents = true }) {
  const D = window.DATA;
  const s = cents ? D.usd(n) : D.usd0(n);
  return <span className="mono tnum" style={{ fontWeight: big ? 600 : 500, color: gold ? "var(--gold-pale)" : "inherit", letterSpacing: big ? "-0.01em" : 0 }}>{s}</span>;
}

/* ---------- Product line-art (editorial luxury illustration per category) ----------
   Drawn as gold linework over a tonal vault backdrop; reads like a catalogue plate.
   These stand in for real photography — drop a real photo in later to replace. */
const ART = {
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
      {/* easel legs */}
      <path d="M58 168 L74 58 M142 168 L126 58 M80 150 L120 150" strokeWidth="1.6" strokeOpacity="0.55" />
      <path d="M100 60 L100 44" strokeWidth="1.6" strokeOpacity="0.55" />
      {/* ornate double frame */}
      <rect x="46" y="40" width="108" height="96" fill="currentColor" fillOpacity="0.05" />
      <rect x="46" y="40" width="108" height="96" />
      <rect x="54" y="48" width="92" height="80" strokeWidth="1.2" strokeOpacity="0.7" />
      {/* corner flourishes */}
      <path d="M46 40 L54 48 M154 40 L146 48 M46 136 L54 128 M154 136 L146 128" strokeWidth="1.2" strokeOpacity="0.6" />
      {/* canvas scene: sun + hills + brushstroke */}
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
function ProductArt({ cat, w = "74%", top = "50%" }) {
  return (
    <svg viewBox="0 0 200 200" preserveAspectRatio="xMidYMid meet" aria-hidden="true"
      style={{ position:"absolute", top, left:"50%", transform:"translate(-50%,-50%)",
        width:w, height:w, color:"var(--gold)",
        filter:"drop-shadow(0 6px 16px rgba(0,0,0,0.4))" }}>
      {ART[cat] || ART.gem}
    </svg>
  );
}

/* ---------- Image tile (renders product art; user can drop a real photo later) ---------- */
function Ph({ label, glyph, art, artW, artTop, ratio, style, children }) {
  const isArt = !!art;
  return (
    <div className={"ph" + (isArt ? " ph-art" : "")} style={{ aspectRatio: ratio || "1 / 1",
      ...(isArt ? { background:"radial-gradient(120% 100% at 50% 16%, var(--bg-3), var(--bg-0) 80%)" } : {}), ...style }}>
      {isArt
        ? <ProductArt cat={art} w={artW} top={artTop} />
        : (glyph && <div style={{ position:"absolute", opacity:0.5 }}><CatGlyph g={glyph} size={64} /></div>)}
      {label && (isArt
        ? <>
            <div style={{ position:"absolute", inset:0, background:"linear-gradient(to top, rgba(12,8,9,0.55), transparent 42%)" }} />
            <div className="mono up" style={{ position:"absolute", insetInlineStart:11, bottom:9, zIndex:2,
              fontSize:9, letterSpacing:"0.14em", color:"var(--gold-pale)" }}>{label}</div>
          </>
        : <div className="ph-label" style={{ position:"relative", zIndex:1 }}>{label}</div>)}
      {children}
    </div>
  );
}

/* ---------- small hooks ---------- */
function useInterval(cb, ms, on = true) {
  const ref = useRef(cb); ref.current = cb;
  useEffect(() => { if (!on) return; const id = setInterval(() => ref.current(), ms); return () => clearInterval(id); }, [ms, on]);
}
function fmtClock(s) {
  s = Math.max(0, Math.floor(s));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), ss = s % 60;
  const p = (x) => String(x).padStart(2, "0");
  return h > 0 ? `${h}:${p(m)}:${p(ss)}` : `${p(m)}:${p(ss)}`;
}

Object.assign(window, { Icon, Seal, CatGlyph, ProductArt, Chip, Money, Ph, useInterval, fmtClock, STATE_MAP });
