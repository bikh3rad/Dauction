import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useSession } from "@/hooks/useSession";
import { useInspections, useInspect, useSignOut, useDemoLogin } from "@/hooks/queries";
import { Icon } from "@/components/ui/Icon";
import { CategoryIcon } from "@/components/ui/CategoryIcon";
import { Ph } from "@/components/ui/ProductArt";
import { Money } from "@/components/ui/Money";
import { maisonOf, CATEGORY_META } from "@/lib/enrich";
import type { PendingInspection } from "@/mock/db";

// Inspector (Auditor) console — a dedicated full-screen view where an inspector
// confirms or rejects newly listed objects before they reach the public gallery.
// Reached at /inspector; gated on the INSPECTOR role.
export function InspectorPage() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { account, isGuest } = useSession();
  const demo = useDemoLogin();
  const signOut = useSignOut();
  const { data: queue = [], isLoading } = useInspections();
  const { approve, reject } = useInspect();
  const isInspector = (account?.roles ?? []).includes("INSPECTOR");

  if (!isInspector) {
    return (
      <Shell onSignOut={isGuest ? undefined : async () => { await signOut.mutateAsync(); nav("/login"); }}>
        <div style={{ maxWidth: 380, margin: "12vh auto 0", textAlign: "center", padding: 24 }}>
          <span style={{ color: "var(--gold)" }}><Icon name="shield" size={34} /></span>
          <h2 className="serif" style={{ fontSize: 22, color: "var(--gold-pale)", margin: "12px 0 6px" }}>{t("insp_gate_title")}</h2>
          <p className="muted" style={{ fontSize: 13.5, lineHeight: 1.6, marginBottom: 22 }}>{t("insp_gate_body")}</p>
          <button className="btn btn-gold" style={{ width: "100%" }} onClick={async () => { await demo.mutateAsync("inspector"); }}>
            <Icon name="shield" size={16} /> {t("insp_signin")}
          </button>
          <button className="btn btn-ghost" style={{ width: "100%", marginTop: 10 }} onClick={() => nav("/")}>{t("nav_gallery")}</button>
        </div>
      </Shell>
    );
  }

  return (
    <Shell onSignOut={async () => { await signOut.mutateAsync(); nav("/login"); }}>
      <div style={{ maxWidth: 1100, margin: "0 auto", padding: "28px 24px" }}>
        <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", marginBottom: 6 }}>{t("insp_sub")}</div>
        <h1 className="serif" style={{ fontSize: 30, margin: "0 0 4px", color: "var(--gold-pale)" }}>{t("insp_title")}</h1>
        <p className="muted" style={{ fontSize: 13.5, marginBottom: 24 }}>{queue.length} {t("insp_pending")}</p>

        {isLoading ? (
          <p className="muted">…</p>
        ) : queue.length === 0 ? (
          <div style={{ textAlign: "center", padding: "60px 20px", border: "1px dashed var(--line)", borderRadius: "var(--r-3)" }}>
            <span style={{ color: "var(--st-good)" }}><Icon name="check" size={30} /></span>
            <p className="muted" style={{ marginTop: 12 }}>{t("insp_empty")}</p>
          </div>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))", gap: 18 }}>
            {queue.map((p) => (
              <InspectionCard key={p.id} p={p}
                onApprove={() => approve.mutate(p.id)} onReject={() => reject.mutate(p.id)}
                busy={approve.isPending || reject.isPending} />
            ))}
          </div>
        )}
      </div>
    </Shell>
  );
}

function InspectionCard({ p, onApprove, onReject, busy }: { p: PendingInspection; onApprove: () => void; onReject: () => void; busy: boolean }) {
  const { t } = useI18n();
  const body = p.title.split(/\s+[—–-]\s+/).slice(1).join(" — ") || p.title;
  const hasImg = p.imageRefs && p.imageRefs.length > 0;
  return (
    <div className="fade-up" style={{ border: "1px solid var(--line)", borderRadius: "var(--r-3)", overflow: "hidden", background: "var(--bg-1)" }}>
      <div style={{ position: "relative", aspectRatio: "4 / 3", background: "var(--bg-0)" }}>
        {hasImg
          ? <img src={p.imageRefs![0]} alt={body} style={{ width: "100%", height: "100%", objectFit: "cover" }} />
          : <Ph art={p.category ? CATEGORY_META[p.category].art : "watch"} ratio="4 / 3" />}
        {p.category && (
          <div style={{ position: "absolute", top: 10, insetInlineEnd: 10, width: 30, height: 30, borderRadius: "50%", background: "rgba(12,8,9,0.66)", border: "1px solid var(--gold-line)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--gold-pale)" }}>
            <CategoryIcon category={p.category} size={17} />
          </div>
        )}
        <span className="chip" data-st="warn" style={{ position: "absolute", top: 10, insetInlineStart: 10 }}>{p.atype}</span>
      </div>
      {hasImg && p.imageRefs!.length > 1 && (
        <div style={{ display: "flex", gap: 6, padding: "8px 12px 0", overflowX: "auto" }}>
          {p.imageRefs!.slice(0, 7).map((src, i) => (
            <img key={i} src={src} alt="" style={{ width: 40, height: 40, borderRadius: 6, objectFit: "cover", border: "1px solid var(--line)", flexShrink: 0 }} />
          ))}
        </div>
      )}
      <div style={{ padding: "12px 14px" }}>
        <div className="mono up" style={{ fontSize: 9, color: "var(--gold)" }}>{maisonOf(p.title)}</div>
        <div className="serif" style={{ fontSize: 16, color: "var(--fg)", margin: "3px 0 8px", lineHeight: 1.2 }}>{body}</div>
        <div className="kv" style={{ marginBottom: 12 }}>
          <div className="k">{t("insp_owner")}</div><div className="v mono" dir="ltr">{p.ownerHandle}</div>
          <div className="k">{t("insp_value")}</div><div className="v"><Money cents={p.valueCents} withCents={false} /></div>
        </div>
        <div style={{ display: "flex", gap: 10 }}>
          <button className="btn btn-gold" style={{ flex: 1 }} disabled={busy} onClick={onApprove}><Icon name="check" size={16} /> {t("insp_approve")}</button>
          <button className="btn btn-ghost" style={{ flex: 1, color: "var(--st-bad)", borderColor: "var(--st-bad)" }} disabled={busy} onClick={onReject}>{t("insp_reject")}</button>
        </div>
      </div>
    </div>
  );
}

function Shell({ children, onSignOut }: { children: React.ReactNode; onSignOut?: () => void }) {
  const { t } = useI18n();
  const nav = useNavigate();
  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 50, display: "flex", flexDirection: "column", background: "var(--bg-void)", color: "var(--fg)", overflow: "auto" }}>
      <div style={{ height: 56, flexShrink: 0, borderBottom: "1px solid var(--gold-line)", display: "flex", alignItems: "center", padding: "0 20px", gap: 14, background: "rgba(20,12,16,0.7)", backdropFilter: "blur(12px)", position: "sticky", top: 0, zIndex: 2 }}>
        <span style={{ color: "var(--gold)" }}><Icon name="shield" size={20} /></span>
        <span className="serif" style={{ fontSize: 18, color: "var(--gold-pale)" }}>{t("brand")}</span>
        <span className="mono up" style={{ fontSize: 9.5, color: "var(--fg-faint)", border: "1px solid var(--line)", padding: "3px 8px", borderRadius: "var(--r-1)" }}>{t("insp_badge")}</span>
        <div style={{ flex: 1 }} />
        <button className="btn btn-ghost" style={{ fontSize: 13, padding: "8px 12px" }} onClick={() => nav("/")}><Icon name="arrow-left" size={15} /> {t("nav_gallery")}</button>
        {onSignOut && <button className="btn btn-ghost" style={{ fontSize: 13, padding: "8px 12px" }} onClick={onSignOut}>{t("acc_signout")}</button>}
      </div>
      <div style={{ flex: 1 }}>{children}</div>
    </div>
  );
}
