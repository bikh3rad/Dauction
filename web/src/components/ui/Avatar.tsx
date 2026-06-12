import type { Account } from "@/types";

// Round profile image. Shows the account's avatar (uploaded or from the connected
// Google/Facebook account); falls back to the username initial.
export function Avatar({ account, size = 32 }: { account?: Account | null; size?: number }) {
  const url = account?.avatarUrl;
  const initial = (account?.handle?.trim()?.[0] ?? "U").toUpperCase();
  return (
    <div style={{ width: size, height: size, borderRadius: "50%", overflow: "hidden", border: "1px solid var(--gold-line)", background: "var(--burg)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--gold-pale)", flexShrink: 0 }}>
      {url
        ? <img src={url} alt="" style={{ width: "100%", height: "100%", objectFit: "cover" }} />
        : <span className="serif" style={{ fontSize: Math.round(size * 0.42), lineHeight: 1 }}>{initial}</span>}
    </div>
  );
}
