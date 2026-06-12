import type { ReactNode } from "react";
import type { Category } from "@/types";

// One distinct line-SVG icon per product category. Shown in the vault add-object
// picker, on each vault object, and on the gallery card so end users can tell an
// item's type at a glance (the design directive: unique icon per category).
const PATHS: Record<Category, ReactNode> = {
  horology: (
    <>
      <circle cx="12" cy="12" r="5.6" />
      <path d="M12 9.4V12l1.8 1" />
      <path d="M9.2 6.7 8.6 3.4h6.8l-.6 3.3" />
      <path d="M9.2 17.3l-.6 3.3h6.8l-.6-3.3" />
    </>
  ),
  bag: (
    <>
      <path d="M5 8h14l-1.1 11.4a1 1 0 0 1-1 .9H7.1a1 1 0 0 1-1-.9L5 8z" />
      <path d="M8.5 8V6.4a3.5 3.5 0 0 1 7 0V8" />
    </>
  ),
  sneaker: (
    <>
      <path d="M2.5 15.4c0-.5.4-1 1-1h2l2.6-3 1.9 1 2.6-1.6 1.5 1.6c2 .4 4.8 1.2 6.3 2.6.6.6.9 1.3.9 2.2H3.5a1 1 0 0 1-1-1v-.8z" />
      <path d="M7.6 11.6 9 13.1M11.6 11l1.1 1.5" />
    </>
  ),
  perfume: (
    <>
      <rect x="8" y="9" width="8" height="11" rx="2" />
      <path d="M10 9V6h4v3" />
      <rect x="9.4" y="2.8" width="5.2" height="3.2" rx="0.6" />
      <path d="M10 14h4" />
    </>
  ),
  art: (
    <>
      <rect x="4" y="4" width="16" height="16" rx="1.2" />
      <circle cx="9" cy="9" r="1.3" />
      <path d="M4 16.2l4.6-4 3 2.6L16 9.2l4 4.4" />
    </>
  ),
  painting: (
    <>
      <path d="M12 3.4c-4.8 0-8.6 3.4-8.6 7.7 0 3.4 2.8 5.4 5 5.4 1.2 0 1.9.8 1.9 1.7 0 1.2 1 2.3 2.4 2.3 4 0 7.9-3.7 7.9-8.5 0-4.8-3.8-8.6-8.6-8.6z" />
      <circle cx="8" cy="11" r="1" />
      <circle cx="12" cy="8" r="1" />
      <circle cx="16" cy="11" r="1" />
    </>
  ),
  jewel: (
    <>
      <path d="M6.5 3.5h11l3 4.6-8.5 12.4L3.5 8.1z" />
      <path d="M3.6 8.1h16.8" />
      <path d="M9 3.5 12 8.1l3-4.6M12 8.1 9.5 20.5M12 8.1l2.5 12.4" />
    </>
  ),
};

export function CategoryIcon({ category, size = 20, stroke = 1.7 }: { category: Category; size?: number; stroke?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {PATHS[category]}
    </svg>
  );
}
