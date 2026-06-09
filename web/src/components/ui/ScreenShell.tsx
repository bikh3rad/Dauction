import type { CSSProperties, ReactNode } from "react";

// A screen = fixed header · scrolling body · optional fixed footer.
export function ScreenShell({
  top,
  footer,
  children,
  bg,
}: {
  top?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  bg?: string;
}) {
  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column", background: bg || "transparent" }}>
      {top && <div style={{ flexShrink: 0 }}>{top}</div>}
      <div className="screen-body noscroll">{children}</div>
      {footer && <div style={{ flexShrink: 0 }}>{footer}</div>}
    </div>
  );
}

export const sectionPad: CSSProperties = { padding: "18px 20px 40px" };
