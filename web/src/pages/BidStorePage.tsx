import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { useBuyBids, usePackages, useWallet } from "@/hooks/queries";
import { ScreenShell } from "@/components/ui/ScreenShell";
import { TopBar } from "@/components/ui/TopBar";
import { Icon } from "@/components/ui/Icon";
import { LoadingScreen } from "@/components/ui/States";
import { dollars } from "@/lib/format";
import type { BidPackage } from "@/types";

export function BidStorePage() {
  const { t } = useI18n();
  const nav = useNavigate();
  const { data: wallet } = useWallet();
  const { data: packages, isLoading } = usePackages();
  const buy = useBuyBids();
  const [bought, setBought] = useState<BidPackage | null>(null);

  const onBuy = async (pkg: BidPackage) => {
    await buy.mutateAsync(pkg.id);
    setBought(pkg);
    setTimeout(() => setBought(null), 1800);
  };

  return (
    <ScreenShell top={<TopBar onBack={() => nav(-1)} kicker={t("bid_wallet")} title={t("bid_store")} />}>
      <div style={{ padding: "18px 20px 40px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "16px 18px", borderRadius: "var(--r-2)", border: "1px solid var(--gold-line)", background: "linear-gradient(100deg,var(--burg-deep),var(--bg-1))", marginBottom: 18 }}>
          <span style={{ color: "var(--gold)" }}><Icon name="coins" size={26} /></span>
          <div style={{ flex: 1 }}>
            <div className="mono up" style={{ fontSize: 9, color: "var(--gold)" }}>{t("bid_wallet")}</div>
            <div className="mono" style={{ fontSize: 24, color: "var(--gold-pale)", marginTop: 2 }}>{wallet?.balanceCredits ?? "—"} <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("bid_credits")}</span></div>
          </div>
        </div>
        <p className="muted" style={{ fontSize: 13, lineHeight: 1.6, margin: "0 0 18px" }}>{t("bid_store_sub")}</p>

        {isLoading ? (
          <LoadingScreen rows={2} />
        ) : (
          <div className="resp-cards" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {packages?.map((pkg) => {
              const perBid = (pkg.priceCents / pkg.credits) / 100;
              const save = perBid < 1 ? `${Math.round((1 - perBid) * 100)}%` : "—";
              return (
                <div key={pkg.id} style={{ position: "relative", border: "1px solid", borderColor: pkg.bestValue ? "var(--gold)" : "var(--line)", borderRadius: "var(--r-2)", background: pkg.bestValue ? "linear-gradient(120deg,var(--burg-deep),var(--bg-1))" : "var(--bg-1)", padding: "18px 20px", display: "flex", alignItems: "center", gap: 16 }}>
                  {pkg.bestValue && <span className="chip" data-st="live" style={{ position: "absolute", top: -10, insetInlineStart: 16 }}>{t("best_value")}</span>}
                  <div style={{ flex: 1 }}>
                    <div className="serif" style={{ fontSize: 26, color: "var(--gold-pale)", lineHeight: 1 }}>{pkg.credits} <span style={{ fontSize: 13, color: "var(--fg-muted)", fontFamily: "var(--sans)" }}>{t("pkg_bids")}</span></div>
                    <div className="mono" style={{ fontSize: 11, color: "var(--fg-faint)", marginTop: 6 }}>${perBid.toFixed(2)} {t("per_bid")}{save !== "—" ? ` · ${t("pkg_save")} ${save}` : ""}</div>
                  </div>
                  <button className={"btn " + (pkg.bestValue ? "btn-gold" : "btn-ghost")} onClick={() => onBuy(pkg)} disabled={buy.isPending} style={{ minWidth: 104, flexDirection: "column", height: "auto", padding: "10px 16px", gap: 2 }}>
                    <span className="mono" style={{ fontSize: 16, fontWeight: 600 }}>{dollars(pkg.priceCents)}</span>
                    <span style={{ fontSize: 11 }}>{t("pkg_buy")}</span>
                  </button>
                </div>
              );
            })}
          </div>
        )}

        {bought && (
          <div className="fade-up" style={{ marginTop: 18, padding: "14px 16px", border: "1px solid var(--st-good)", borderRadius: "var(--r-2)", background: "var(--st-good-bg)", display: "flex", alignItems: "center", gap: 10, color: "var(--st-good)", fontWeight: 600, fontSize: 13.5 }}>
            <Icon name="check" size={18} /> +{bought.credits} {t("bought_bids")}
          </div>
        )}
      </div>
    </ScreenShell>
  );
}
