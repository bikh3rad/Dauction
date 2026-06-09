// Octagonal attestation seal — the house's certification stamp.
export interface SealProps {
  size?: number;
  label?: string;
  sub?: string;
  id?: string;
  date?: string;
  live?: boolean;
}

export function Seal({
  size = 72,
  label = "CERTIFIED",
  sub = "DAUCTION",
  id = "0xA1",
  date = "2026",
  live = false,
}: SealProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 96 96"
      fill="none"
      style={{ color: "var(--gold)", filter: live ? "drop-shadow(0 0 6px var(--gold-glow))" : "none" }}
      aria-hidden="true"
    >
      <polygon points="28,8 68,8 88,28 88,68 68,88 28,88 8,68 8,28" stroke="currentColor" strokeWidth="2.5" fill="none" />
      <polygon points="32,14 64,14 82,32 82,64 64,82 32,82 14,64 14,32" stroke="currentColor" strokeWidth="1" fill="none" />
      <text x="48" y="43" textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontWeight="600" fontSize="10" letterSpacing="2" fill="currentColor">{label}</text>
      <line x1="22" y1="50" x2="74" y2="50" stroke="currentColor" strokeWidth="0.8" />
      <text x="48" y="63" textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontWeight="500" fontSize="7.5" letterSpacing="1.5" fill="currentColor" opacity="0.8">{sub} · {id}</text>
      <text x="48" y="75" textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontWeight="500" fontSize="7.5" letterSpacing="1.5" fill="currentColor" opacity="0.7">{date}</text>
    </svg>
  );
}
