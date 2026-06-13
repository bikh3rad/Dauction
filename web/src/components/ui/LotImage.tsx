import { useState } from "react";
import type { CSSProperties } from "react";
import { Ph } from "./ProductArt";

// LotImage shows a real product photo when one is available, degrading to the
// category line-art placeholder (Ph) if there is no photo or the URL fails to
// load — so the gallery never shows a broken image.
export function LotImage({ src, art, ratio = "1 / 1", artW, artTop, style, label }: {
  src?: string; art: string; ratio?: string; artW?: string; artTop?: string; style?: CSSProperties; label?: string;
}) {
  const [failed, setFailed] = useState(false);
  if (!src || failed) return <Ph art={art} ratio={ratio} artW={artW} artTop={artTop} style={style} />;
  return (
    <div style={{ position: "relative", aspectRatio: ratio, overflow: "hidden", background: "var(--bg-0)", ...style }}>
      <img
        src={src}
        alt={label ?? ""}
        loading="lazy"
        onError={() => setFailed(true)}
        style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }}
      />
    </div>
  );
}
