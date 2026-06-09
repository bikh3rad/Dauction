import type { CSSProperties } from "react";

// Curated Lucide-style icon set (24-grid, 2px stroke) — ported from the prototype.
const PATHS: Record<string, string> = {
  "chevron-left": "M15 18l-6-6 6-6",
  "chevron-right": "M9 18l6-6-6-6",
  "arrow-right": "M5 12h14 M13 6l6 6-6 6",
  "arrow-left": "M19 12H5 M11 6l-6 6 6 6",
  lock: "M5 11h14v10H5z M8 11V7a4 4 0 0 1 8 0v4",
  clock: "M12 7v5l3 2 M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
  eye: "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z",
  users: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M22 21v-2a4 4 0 0 0-3-3.87 M16 3.13A4 4 0 0 1 16 11",
  check: "M20 6L9 17l-5-5",
  x: "M18 6L6 18 M6 6l12 12",
  shield: "M12 3l8 4v5c0 5-3.5 8-8 9-4.5-1-8-4-8-9V7z",
  gavel: "M14 13l-7.5 7.5a2.1 2.1 0 0 1-3-3L11 10 M16 16l5-5 M3 21h7 M12.5 6.5l5 5 3-3-5-5z",
  store: "M3 9l1.5-5h15L21 9 M4 9v11h16V9 M3 9h18 M9 20v-6h6v6",
  user: "M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2 M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z",
  coins: "M9 14a6 6 0 1 0 0-12 6 6 0 0 0 0 12z M16.5 9.4A6 6 0 1 1 9.5 21",
  file: "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z M14 3v5h5 M9 13h6 M9 17h6",
  hash: "M4 9h16 M4 15h16 M10 3L8 21 M16 3l-2 18",
  plus: "M12 5v14 M5 12h14",
  search: "M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16z M21 21l-4.3-4.3",
  bell: "M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9 M13.7 21a2 2 0 0 1-3.4 0",
  grid: "M3 3h7v7H3z M14 3h7v7h-7z M14 14h7v7h-7z M3 14h7v7H3z",
  layers: "M12 2l9 5-9 5-9-5 9-5z M3 12l9 5 9-5 M3 17l9 5 9-5",
  image: "M3 3h18v18H3z M8.5 10a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3z M21 15l-5-5L5 21",
  upload: "M12 15V3 M7 8l5-5 5 5 M5 17v2a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-2",
  package: "M21 8l-9-5-9 5 9 5 9-5z M3 8v8l9 5 9-5V8 M12 13v8",
  watch: "M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M9 4l.5-2h5L15 4 M9 20l.5 2h5l.5-2",
  shoe: "M2 17h17a3 3 0 0 0 3-3l-7-3-3-4H4z M2 17v2h20v-2 M9 8l1 2 M12 9l1 2",
  bottle: "M10 2h4v3l1 3v13H9V8l1-3z M9 12h6",
  frame: "M3 3h18v18H3z M7 7h10v10H7z",
  gem: "M6 3h12l3 6-9 12L3 9z M3 9h18 M9 3L6 9l6 12 6-12-3-6",
  refresh: "M21 12a9 9 0 1 1-3-6.7L21 8 M21 3v5h-5",
  alert: "M12 9v4 M12 17h.01 M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z",
  crown: "M3 7l4 5 5-7 5 7 4-5v11H3z M3 18h18",
  scale: "M12 3v18 M7 21h10 M5 7h14 M5 7l-3 7h6zM19 7l-3 7h6z",
  dollar: "M12 2v20 M17 6.5a4 4 0 0 0-4-2.5H11a3.5 3.5 0 0 0 0 7h2a3.5 3.5 0 0 1 0 7h-2a4 4 0 0 1-4-2.5",
  flame: "M12 22c4 0 7-2.5 7-6.5 0-3-2-5-3-6.5-.7 1-1.5 1.5-2.5 1.5 0-2-1-4.5-3-6 0 3-2 4-3.5 6C3 12 3 14 5 17a7 7 0 0 0 7 5z",
  menu: "M3 6h18 M3 12h18 M3 18h18",
};

export interface IconProps {
  name: string;
  size?: number;
  stroke?: number;
  style?: CSSProperties;
  className?: string;
}

export function Icon({ name, size = 20, stroke = 2, style, className }: IconProps) {
  const d = PATHS[name] ?? PATHS.crown;
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={stroke}
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{ flexShrink: 0, display: "block", ...style }}
      className={className}
      aria-hidden="true"
    >
      {d.split("M").filter(Boolean).map((seg, i) => (
        <path key={i} d={"M" + seg} />
      ))}
    </svg>
  );
}
