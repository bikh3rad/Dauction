import { useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { useSettings, getSettings, saveSettings, resetSettings, type LevelCfg, type PackageCfg, type CategoryCfg, type Economics } from "@/mock/settings";
import { useInspections, useInspect } from "@/hooks/queries";
import { SecHead, GBtn } from "./adminUi";
import { Money } from "@/components/ui/Money";
import { Icon } from "@/components/ui/Icon";

const dollars = (cents: number) => (cents / 100).toString();
const toCents = (d: string) => Math.round((Number(d) || 0) * 100);

function Card({ title, children, onSave, onReset }: { title: string; children: React.ReactNode; onSave?: () => void; onReset?: () => void }) {
  return (
    <div style={{ border: "1px solid var(--line)", borderRadius: "var(--r-2)", background: "var(--bg-1)", padding: "18px 20px", marginBottom: 18 }}>
      <div style={{ display: "flex", alignItems: "center", marginBottom: 14 }}>
        <div className="mono up" style={{ fontSize: 10, color: "var(--gold)", letterSpacing: "0.12em", flex: 1 }}>{title}</div>
        {onReset && <GBtn small onClick={onReset}>Reset</GBtn>}
        {onSave && <span style={{ marginInlineStart: 8 }}><GBtn small kind="gold" onClick={onSave}>Save</GBtn></span>}
      </div>
      {children}
    </div>
  );
}

function Num({ label, value, onChange, suffix }: { label: string; value: number | string; onChange: (v: string) => void; suffix?: string }) {
  return (
    <label style={{ display: "block" }}>
      <span className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{label}</span>
      <span style={{ display: "flex", alignItems: "center", gap: 6, marginTop: 4 }}>
        <input className="field" dir="ltr" inputMode="decimal" value={String(value)} onChange={(e) => onChange(e.target.value)} style={{ width: "100%" }} />
        {suffix && <span className="mono" style={{ fontSize: 12, color: "var(--fg-muted)" }}>{suffix}</span>}
      </span>
    </label>
  );
}

// The platform's editable variables — every economic constant, membership level,
// bid package and category is editable here and persisted (mock/settings).
export function Settings() {
  const { t } = useI18n();
  const s = useSettings();
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_settings")} action={<GBtn kind="bad" small onClick={() => { if (confirm("Reset all platform settings to defaults?")) resetSettings(); }}>Reset all</GBtn>} />
      <EconomicsCard economics={s.economics} />
      <LevelsCard levels={s.levels} />
      <PackagesCard packages={s.packages} />
      <CategoriesCard categories={s.categories} />
    </div>
  );
}

function EconomicsCard({ economics }: { economics: Economics }) {
  const [d, setD] = useState<Economics>(economics);
  const set = (k: keyof Economics) => (v: string) => setD({ ...d, [k]: Number(v) || 0 });
  return (
    <Card title="Economics" onSave={() => saveSettings({ ...getSettings(), economics: d })} onReset={() => setD(economics)}>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(150px, 1fr))", gap: 14 }}>
        <Num label="Weekly supply cap" value={d.weeklyCap} onChange={set("weeklyCap")} />
        <Num label="Buyer's premium" value={d.premiumBps / 100} onChange={(v) => setD({ ...d, premiumBps: Math.round((Number(v) || 0) * 100) })} suffix="%" />
        <Num label="Buyback · cash" value={d.buybackCashPct} onChange={set("buybackCashPct")} suffix="%" />
        <Num label="Buyback · credit" value={d.buybackCreditPct} onChange={set("buybackCreditPct")} suffix="%" />
        <Num label="Reservation deposit" value={d.depositPct} onChange={set("depositPct")} suffix="%" />
        <Num label="Funding window" value={d.fundingHours} onChange={set("fundingHours")} suffix="h" />
        <Num label="OTP lifetime" value={d.otpTtlSecs} onChange={set("otpTtlSecs")} suffix="s" />
      </div>
    </Card>
  );
}

function LevelsCard({ levels }: { levels: LevelCfg[] }) {
  const [d, setD] = useState<LevelCfg[]>(levels);
  const upd = (i: number, patch: Partial<LevelCfg>) => setD(d.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  return (
    <Card title="Membership levels" onSave={() => saveSettings({ ...getSettings(), levels: d })} onReset={() => setD(levels)}>
      {d.map((l, i) => (
        <div key={l.level} style={{ display: "grid", gridTemplateColumns: "40px 1.2fr 1fr 0.8fr 2fr", gap: 10, alignItems: "center", padding: "8px 0", borderBottom: i < d.length - 1 ? "1px solid var(--line)" : "none" }}>
          <span className="mono" style={{ color: "var(--gold)" }}>L{l.level}</span>
          <input className="field" value={l.name} onChange={(e) => upd(i, { name: e.target.value })} placeholder="Name" />
          <Num label="$ / year" value={dollars(l.priceCentsYear)} onChange={(v) => upd(i, { priceCentsYear: toCents(v) })} />
          <Num label="Premium %" value={l.premiumBps / 100} onChange={(v) => upd(i, { premiumBps: Math.round((Number(v) || 0) * 100) })} />
          <input className="field" value={l.perks.join(", ")} onChange={(e) => upd(i, { perks: e.target.value.split(",").map((p) => p.trim()).filter(Boolean) })} placeholder="perk keys, comma-separated" />
        </div>
      ))}
    </Card>
  );
}

function PackagesCard({ packages }: { packages: PackageCfg[] }) {
  const [d, setD] = useState<PackageCfg[]>(packages);
  const upd = (i: number, patch: Partial<PackageCfg>) => setD(d.map((p, j) => (j === i ? { ...p, ...patch } : p)));
  const add = () => setD([...d, { id: `PKG_NEW_${d.length + 1}`, credits: 10, priceCents: 1000, bestValue: false }]);
  return (
    <Card title="Bid credit packages" onSave={() => saveSettings({ ...getSettings(), packages: d })} onReset={() => setD(packages)}>
      {d.map((p, i) => (
        <div key={p.id} style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 90px 40px", gap: 10, alignItems: "center", padding: "8px 0", borderBottom: "1px solid var(--line)" }}>
          <input className="field" value={p.id} onChange={(e) => upd(i, { id: e.target.value })} />
          <Num label="Credits" value={p.credits} onChange={(v) => upd(i, { credits: Number(v) || 0 })} />
          <Num label="$ price" value={dollars(p.priceCents)} onChange={(v) => upd(i, { priceCents: toCents(v) })} />
          <button className="mono" onClick={() => upd(i, { bestValue: !p.bestValue })} style={{ fontSize: 10, padding: "8px", borderRadius: "var(--r-1)", border: "1px solid var(--line-strong)", background: p.bestValue ? "var(--gold)" : "transparent", color: p.bestValue ? "#1B1207" : "var(--fg-muted)", cursor: "pointer" }}>BEST</button>
          <button onClick={() => setD(d.filter((_, j) => j !== i))} style={{ background: "none", border: "none", color: "var(--st-bad)", cursor: "pointer" }}><Icon name="hash" size={14} />✕</button>
        </div>
      ))}
      <div style={{ marginTop: 12 }}><GBtn small onClick={add}><Icon name="plus" size={13} /> Add package</GBtn></div>
    </Card>
  );
}

function CategoriesCard({ categories }: { categories: CategoryCfg[] }) {
  const [d, setD] = useState<CategoryCfg[]>(categories);
  const upd = (i: number, patch: Partial<CategoryCfg>) => setD(d.map((c, j) => (j === i ? { ...c, ...patch } : c)));
  return (
    <Card title="Categories" onSave={() => saveSettings({ ...getSettings(), categories: d })} onReset={() => setD(categories)}>
      {d.map((c, i) => (
        <div key={c.key} style={{ display: "grid", gridTemplateColumns: "120px 1fr 90px", gap: 10, alignItems: "center", padding: "8px 0", borderBottom: i < d.length - 1 ? "1px solid var(--line)" : "none" }}>
          <span className="mono" style={{ color: "var(--gold-pale)" }}>{c.key}</span>
          <input className="field" value={c.label} onChange={(e) => upd(i, { label: e.target.value })} />
          <button className="mono" onClick={() => upd(i, { active: !c.active })} style={{ fontSize: 10, padding: "8px", borderRadius: "var(--r-1)", border: "1px solid var(--line-strong)", background: c.active ? "var(--st-good-bg)" : "transparent", color: c.active ? "var(--st-good)" : "var(--fg-faint)", cursor: "pointer" }}>{c.active ? "ACTIVE" : "OFF"}</button>
        </div>
      ))}
    </Card>
  );
}

// ===== Inspections oversight (admin mirror of the inspector queue) =====
export function Inspections() {
  const { t } = useI18n();
  const { data: queue = [] } = useInspections();
  const { approve, reject } = useInspect();
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("insp_title")} />
      {queue.length === 0 ? (
        <p className="muted">{t("insp_empty")}</p>
      ) : (
        <div style={{ border: "1px solid var(--line)", borderRadius: "var(--r-2)", overflow: "hidden", background: "var(--bg-1)" }}>
          {queue.map((p) => (
            <div key={p.id} style={{ display: "flex", alignItems: "center", gap: 14, padding: "12px 16px", borderBottom: "1px solid var(--line)" }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 14 }}>{p.title}</div>
                <div className="mono" style={{ fontSize: 11, color: "var(--fg-faint)", marginTop: 2 }}>{p.ownerHandle} · {p.atype} · <Money cents={p.valueCents} withCents={false} /></div>
              </div>
              <GBtn small kind="gold" onClick={() => approve.mutate(p.id)}>{t("insp_approve")}</GBtn>
              <GBtn small kind="bad" onClick={() => reject.mutate(p.id)}>{t("insp_reject")}</GBtn>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
