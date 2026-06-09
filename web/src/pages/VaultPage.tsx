import { useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { useBuyback, useListObject, useVault } from "@/hooks/queries";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { LangPill } from "@/components/ui/LangPill";
import { Sheet } from "@/components/ui/Sheet";
import { Icon } from "@/components/ui/Icon";
import { Money } from "@/components/ui/Money";
import { Ph } from "@/components/ui/ProductArt";
import { Chip } from "@/components/ui/Chip";
import { Label } from "@/components/ui/Primitives";
import { LoadingScreen, ErrorState } from "@/components/ui/States";
import { artOf } from "@/lib/enrich";
import { maisonOf } from "@/lib/enrich";
import { usdc0 } from "@/lib/format";
import type { AType, VaultObject } from "@/types";

export function VaultPage() {
  const { t } = useI18n();
  const { data, isLoading, isError, refetch } = useVault();
  const [bb, setBb] = useState<VaultObject | null>(null);
  const [listItem, setListItem] = useState<VaultObject | null>(null);
  const [listed, setListed] = useState(false);
  const [mag] = useState(false);

  if (isLoading) return <ScreenShell top={<TopBar title={t("clo_title")} right={<LangPill />} />}><LoadingScreen /></ScreenShell>;
  if (isError || !data) return <ScreenShell top={<TopBar title={t("clo_title")} right={<LangPill />} />}><ErrorState onRetry={() => refetch()} /></ScreenShell>;

  const objs = data.objects;
  const counts = { in_closet: 0, in_auction: 0, sold: 0 };
  objs.forEach((o) => {
    if (o.state === "SOLD" || o.state === "BOUGHT_BACK") counts.sold++;
    else if (o.state === "IN_AUCTION") counts.in_auction++;
    else counts.in_closet++;
  });
  const totalValue = objs.reduce((a, o) => a + o.appraisedValueCents, 0);

  return (
    <>
      <ScreenShell top={<TopBar kicker={t("clo_sub")} title={t("clo_title")} right={<LangPill />} />}>
        <div style={{ padding: "16px 16px 24px" }}>
          <div style={{ display: "flex", gap: 10, marginBottom: 16 }}>
            <div style={{ flex: 1, padding: "14px 16px", border: "1px solid var(--gold-line)", borderRadius: "var(--r-2)", background: "linear-gradient(120deg,var(--burg-deep),var(--bg-1))" }}>
              <div className="mono up" style={{ fontSize: 9, color: "var(--gold)" }}>{t("clo_credit")}</div>
              <div style={{ marginTop: 6 }}><Money cents={data.creditBalanceCents} big gold withCents={false} /></div>
            </div>
            <div style={{ flex: 1, padding: "14px 16px", border: "1px solid var(--line)", borderRadius: "var(--r-2)", background: "var(--bg-1)" }}>
              <div className="mono up" style={{ fontSize: 9, color: "var(--fg-faint)" }}>{t("clo_value")}</div>
              <div style={{ marginTop: 6 }}><Money cents={totalValue} big withCents={false} /></div>
            </div>
          </div>

          <div style={{ display: "flex", gap: 14, marginBottom: 14, fontSize: 11.5 }}>
            {([["clo_incloset", counts.in_closet], ["clo_inauction", counts.in_auction], ["clo_sold", counts.sold]] as const).map(([k, n]) => (
              <span key={k} className="muted">{t(k)} <b className="mono" style={{ color: "var(--fg)" }}>{n}</b></span>
            ))}
          </div>

          <div style={{ display: "grid", gridTemplateColumns: mag ? "1fr" : "1fr 1fr", gap: 12 }}>
            {objs.map((o) => {
              const inVault = o.state === "IN_VAULT";
              return (
                <div key={o.id} className="fade-up" style={{ background: "var(--bg-1)", border: "1px solid var(--line)", borderRadius: "var(--r-2)", overflow: "hidden" }}>
                  <div style={{ position: "relative" }}>
                    <Ph art={artOf(o)} ratio={mag ? "5 / 3" : "1 / 1"} label={maisonOf(o.title)} />
                    <div style={{ position: "absolute", top: 8, insetInlineStart: 8 }}><Chip state={o.state} label={t(stateKey(o.state))} /></div>
                  </div>
                  <div style={{ padding: "10px 12px" }}>
                    <div className="serif" style={{ fontSize: 13.5, color: "var(--fg)", marginBottom: 4, lineHeight: 1.2 }}>{o.title.split(/\s+[—–-]\s+/).slice(1).join(" — ") || o.title}</div>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
                      <Money cents={o.appraisedValueCents} withCents={false} gold />
                      {inVault && (
                        <div style={{ display: "flex", gap: 10 }}>
                          <button onClick={() => setListItem(o)} className="mono" style={{ fontSize: 10, color: "var(--gold-pale)", background: "none", border: "none", cursor: "pointer", fontWeight: 600 }}>{t("list_title")}</button>
                          <button onClick={() => setBb(o)} className="mono" style={{ fontSize: 10, color: "var(--gold)", background: "none", border: "none", cursor: "pointer", fontWeight: 600 }}>{t("clo_buyback")}</button>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>

          <button className="btn btn-ghost" style={{ width: "100%", margin: "16px 0", borderStyle: "dashed", borderColor: "var(--gold-line)", color: "var(--gold-pale)" }}>
            <Icon name="plus" size={18} /> {t("clo_add")}
          </button>
        </div>
      </ScreenShell>

      <BuybackSheet item={bb} onClose={() => setBb(null)} />
      <ListToAuctionSheet item={listItem} onClose={() => setListItem(null)} onSubmitted={() => { setListItem(null); setListed(true); setTimeout(() => setListed(false), 2200); }} />
      {listed && (
        <div className="fade-up toast" style={{ bottom: 96, zIndex: 80, background: "var(--st-good-bg)", border: "1px solid var(--st-good)", color: "var(--st-good)" }}>
          <Icon name="check" size={16} /> {t("list_submitted")}
        </div>
      )}
    </>
  );
}

function stateKey(state: VaultObject["state"]): string {
  switch (state) {
    case "IN_VAULT": return "st_in_closet";
    case "APPRAISING": return "st_appraising";
    case "IN_AUCTION": return "st_live";
    case "SOLD": case "BOUGHT_BACK": return "st_completed";
    default: return "st_in_closet";
  }
}

function BuybackSheet({ item, onClose }: { item: VaultObject | null; onClose: () => void }) {
  const { t } = useI18n();
  const buyback = useBuyback();
  const open = !!item;
  const run = async (mode: "CASH" | "CREDIT") => {
    if (!item) return;
    await buyback.mutateAsync({ id: item.id, mode });
    onClose();
  };
  return (
    <Sheet open={open} onClose={onClose}>
      <div className="serif" style={{ fontSize: 20, color: "var(--gold-pale)", marginBottom: 4 }}>{t("clo_buyback")}</div>
      <p className="muted" style={{ fontSize: 13, lineHeight: 1.6, margin: "0 0 18px" }}>{t("clo_buyback_body")}</p>
      <div style={{ display: "flex", gap: 12 }}>
        <button className="btn btn-ghost" disabled={buyback.isPending} onClick={() => run("CASH")} style={{ flex: 1, flexDirection: "column", padding: "16px", height: "auto", gap: 4 }}>
          <span style={{ fontSize: 11, color: "var(--fg-muted)" }} className="mono up">{t("clo_cash")}</span>
          <Money cents={Math.round((item?.appraisedValueCents ?? 0) * 0.5)} big withCents={false} />
          <span className="mono" style={{ fontSize: 10, color: "var(--fg-faint)" }}>50%</span>
        </button>
        <button className="btn btn-gold" disabled={buyback.isPending} onClick={() => run("CREDIT")} style={{ flex: 1, flexDirection: "column", padding: "16px", height: "auto", gap: 4 }}>
          <span style={{ fontSize: 11 }} className="mono up">{t("clo_creditopt")}</span>
          <span className="mono" style={{ fontSize: 18, fontWeight: 600 }}>{usdc0(Math.round((item?.appraisedValueCents ?? 0) * 0.85))}</span>
          <span className="mono" style={{ fontSize: 10 }}>85% · Vault Credit</span>
        </button>
      </div>
    </Sheet>
  );
}

function ListToAuctionSheet({ item, onClose, onSubmitted }: { item: VaultObject | null; onClose: () => void; onSubmitted: () => void }) {
  const { t } = useI18n();
  const listObject = useListObject();
  const [atype, setAtype] = useState<AType>("DUTCH");
  const [dur, setDur] = useState(5);
  const open = !!item;
  const timed = atype !== "DUTCH";
  const types: Array<{ k: AType; icon: string }> = [
    { k: "DUTCH", icon: "flame" },
    { k: "VICKREY", icon: "file" },
    { k: "UNIQBID", icon: "hash" },
  ];

  const submit = async () => {
    if (!item) return;
    await listObject.mutateAsync({ id: item.id, atype, durationDays: timed ? dur : undefined });
    onSubmitted();
  };

  return (
    <Sheet open={open} onClose={onClose}>
      <div className="serif" style={{ fontSize: 20, color: "var(--gold-pale)" }}>{t("list_title")}</div>
      <div className="muted" style={{ fontSize: 12.5, margin: "4px 0 16px" }}>{item?.title} · {t("list_sub")}</div>

      <Label>{t("list_choose_type")}</Label>
      <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 18 }}>
        {types.map((ty) => {
          const on = atype === ty.k;
          return (
            <button key={ty.k} onClick={() => setAtype(ty.k)} style={{ textAlign: "start", cursor: "pointer", border: "1px solid", borderColor: on ? "var(--gold)" : "var(--line-strong)", borderRadius: "var(--r-2)", background: on ? "linear-gradient(110deg,var(--burg-deep),var(--bg-1))" : "var(--bg-0)", padding: "13px 15px", display: "flex", alignItems: "center", gap: 12, color: "var(--fg)" }}>
              <span style={{ color: on ? "var(--gold)" : "var(--fg-muted)" }}><Icon name={ty.icon} size={18} /></span>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 600, fontSize: 14 }}>{t("mode_" + ty.k.toLowerCase())}</div>
              </div>
              <span style={{ width: 18, height: 18, borderRadius: "50%", border: "2px solid", borderColor: on ? "var(--gold)" : "var(--line-strong)", display: "flex", alignItems: "center", justifyContent: "center" }}>
                {on && <span style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--gold)" }} />}
              </span>
            </button>
          );
        })}
      </div>

      <div style={{ opacity: timed ? 1 : 0.4, pointerEvents: timed ? "auto" : "none", transition: "opacity .2s" }}>
        <Label>{t("list_duration")} · {t("set_by_owner")}</Label>
        <div style={{ display: "flex", gap: 8, marginBottom: 18 }}>
          {[2, 5, 7].map((d) => (
            <button key={d} onClick={() => setDur(d)} className="mono" style={{ flex: 1, padding: "13px", borderRadius: "var(--r-1)", cursor: "pointer", border: "1px solid", borderColor: dur === d && timed ? "var(--gold)" : "var(--line-strong)", background: dur === d && timed ? "var(--gold)" : "transparent", color: dur === d && timed ? "#1B1207" : "var(--fg-muted)", fontWeight: 600, fontSize: 14 }}>
              {t("dur_" + d + "d")}
            </button>
          ))}
        </div>
      </div>

      <button className="btn btn-gold" style={{ width: "100%" }} onClick={submit} disabled={listObject.isPending}>{t("list_confirm")}</button>
    </Sheet>
  );
}
