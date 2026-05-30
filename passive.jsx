/* ============================================================
   DAUCTION — PASSIVE auctions + bid economy
   - Vickrey  : sealed second-price (one hidden bid; 2nd-highest wins, ties → earliest)
   - UniqBid  : lowest unique price wins
   - Bid credits: $1 each, bought in packages; each submitted bid spends 1 credit
   Works in mobile (ScreenShell) and desktop (inside CenterCol).
   ============================================================ */

/* live countdown for a timed auction */
function useCountdown(seconds, on = true) {
  const target = useRef(Date.now() + seconds * 1000);
  const [now, setNow] = useState(Date.now());
  useInterval(() => setNow(Date.now()), 1000, on);
  const left = Math.max(0, Math.floor((target.current - now) / 1000));
  const d = Math.floor(left / 86400), h = Math.floor((left % 86400) / 3600),
        m = Math.floor((left % 3600) / 60), s = left % 60;
  return { d, h, m, s, left, done: left <= 0 };
}
function CountdownPill({ cd, app, big }) {
  const { t } = app;
  const seg = (n, lbl) => (
    <span style={{ display:"inline-flex", alignItems:"baseline", gap:3 }}>
      <span className="mono tnum" style={{ fontSize:big?22:15, color:"var(--gold-pale)", fontWeight:600 }}>{String(n).padStart(2,"0")}</span>
      <span className="mono up" style={{ fontSize:big?9:8, color:"var(--fg-faint)" }}>{lbl}</span>
    </span>
  );
  return (
    <div style={{ display:"flex", alignItems:"center", gap:big?14:10 }}>
      {seg(cd.d, t("d_short"))}<Sep/>{seg(cd.h, t("h_short"))}<Sep/>{seg(cd.m, t("m_short"))}{!big && <><Sep/>{seg(cd.s,"s")}</>}
    </div>
  );
}
function Sep() { return <span style={{ color:"var(--fg-faint)", opacity:0.5 }}>:</span>; }

/* deterministic simulated competing bids (so the demo feels alive) */
function simTaken(lot) {
  let seed = lot.no * 97 + 13;
  const rnd = () => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed / 0x7fffffff; };
  const s = new Set();
  const n = Math.min(lot.bidsPlaced || 40, 70);
  for (let i = 0; i < n; i++) s.add(1 + Math.floor(rnd() * 480)); // $1–$480
  return s;
}

/* ---------- bid wallet strip ---------- */
function BidWalletStrip({ app }) {
  const { t, go } = app;
  return (
    <div style={{ display:"flex", alignItems:"center", gap:12, padding:"12px 14px", borderRadius:"var(--r-2)",
      border:"1px solid var(--gold-line)", background:"linear-gradient(100deg,var(--burg-deep),var(--bg-1))", marginBottom:18 }}>
      <span style={{ color:"var(--gold)" }}><Icon name="coins" size={20}/></span>
      <div style={{ flex:1 }}>
        <div className="mono up" style={{ fontSize:9, color:"var(--gold)" }}>{t("bid_wallet")}</div>
        <div className="mono" style={{ fontSize:17, color:"var(--gold-pale)", marginTop:2 }}>{app.bidWallet} <span style={{ fontSize:11, color:"var(--fg-muted)" }}>{t("bid_credits")}</span></div>
      </div>
      <button className="btn btn-ghost" style={{ padding:"8px 14px", fontSize:12.5 }} onClick={()=>go("bidstore")}>
        <Icon name="plus" size={15}/> {t("buy_bids")}</button>
    </div>
  );
}

/* ---------- rule explainer ---------- */
function RuleCard({ app, atype }) {
  const { t } = app;
  return (
    <div style={{ display:"flex", gap:12, padding:"14px 16px", borderRadius:"var(--r-2)", border:"1px solid var(--line)",
      background:"var(--bg-1)", marginBottom:18 }}>
      <span style={{ color:"var(--gold)", flexShrink:0, marginTop:2 }}><Icon name={atype==="vickrey"?"file":"hash"} size={18}/></span>
      <div>
        <div style={{ fontWeight:600, fontSize:13.5, marginBottom:4 }}>{t(atype==="vickrey"?"mode_vickrey":"mode_uniqbid")} · {t("auc_mode")}</div>
        <p className="muted" style={{ fontSize:12.5, lineHeight:1.6, margin:0 }}>{t(atype==="vickrey"?"vickrey_rule":"uniqbid_rule")}</p>
      </div>
    </div>
  );
}

/* ============================================================
   PASSIVE AUCTION SCREEN
   ============================================================ */
function PassiveAuctionScreen({ app }) {
  const { t, back, go, lang } = app;
  const D = window.DATA;
  const lot = D.lot(app.nav.params.lotId) || D.LOTS.find(l => l.status === "passive") || D.LOTS[0];
  const atype = lot.atype || "vickrey";
  const cd = useCountdown(lot.closesIn || 5 * 86400, true);
  const canBid = app.tier !== "guest";

  return (
    <ScreenShell top={
      <TopBar app={app} onBack={back}
        kicker={`${t("gal_lot")} ${String(lot.no).padStart(2,"0")} · ${D.ATYPES[atype][lang]}`}
        title={lot.maison}
        right={<span className="chip" data-st="warn" style={{ alignSelf:"center" }}><Icon name="clock" size={12}/> {cd.d}{t("d_short")}</span>} />
    }>
      <div style={{ position:"relative" }}>
        <Ph art={D.CATS[lot.cat].g} artW="50%" artTop="38%" ratio="4 / 3" style={{ borderRadius:0, borderInline:0 }} />
        <div style={{ position:"absolute", top:14, insetInlineStart:14 }}>
          <span className="chip" data-st="warn"><Icon name="clock" size={12}/> {t("passive_kicker")}</span>
        </div>
        <div style={{ position:"absolute", insetInlineStart:0, insetInlineEnd:0, bottom:0, height:"60%", background:"linear-gradient(to top, var(--bg-void) 8%, rgba(12,8,9,0.8) 36%, transparent 100%)" }} />
        <div style={{ position:"absolute", bottom:14, insetInlineStart:16, insetInlineEnd:16 }}>
          <div className="serif" style={{ fontSize:21, color:"var(--gold-pale)", lineHeight:1.2 }}>{D.tt(lot.title, lang)}</div>
        </div>
      </div>

      <div style={{ padding:"18px 20px 40px" }}>
        {/* countdown band */}
        <div style={{ display:"flex", alignItems:"center", justifyContent:"space-between", padding:"14px 16px",
          border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", background:"var(--bg-1)", marginBottom:18 }}>
          <div className="mono up" style={{ fontSize:10, color:"var(--gold)" }}>{t("ends_in")}</div>
          <CountdownPill cd={cd} app={app} />
        </div>

        <RuleCard app={app} atype={atype} />

        {!canBid ? (
          <button className="btn btn-burg" style={{ width:"100%" }} onClick={()=>go("invite")}>
            <Icon name="crown" size={17}/> {t("need_member")}</button>
        ) : (
          <>
            <BidWalletStrip app={app} />
            {atype === "vickrey"
              ? <VickreyPanel app={app} lot={lot} />
              : <UniqBidPanel app={app} lot={lot} />}
          </>
        )}

        <div style={{ marginTop:22 }}>
          <Label>{t("lot_about")}</Label>
          <p className="muted" style={{ fontSize:13.5, lineHeight:1.7, margin:0 }}>{D.tt(lot.desc, lang)}</p>
        </div>
      </div>
    </ScreenShell>
  );
}

/* ---------- Vickrey: one sealed bid ---------- */
function VickreyPanel({ app, lot }) {
  const { t } = app; const D = window.DATA;
  const sealed = app.sealed[lot.id];
  const [amt, setAmt] = useState(sealed ? String(sealed) : "");
  const [edit, setEdit] = useState(!sealed);
  const submit = () => {
    const v = parseInt(amt.replace(/\D/g, ""), 10);
    if (!v) return;
    const ok = app.submitSealed(lot.id, v);
    if (ok) setEdit(false);
  };
  const noBids = app.bidWallet <= 0 && !sealed;
  return (
    <div>
      {!edit && sealed ? (
        <div style={{ border:"1px solid var(--st-good)", borderRadius:"var(--r-2)", background:"var(--st-good-bg)", padding:"16px 18px" }}>
          <div style={{ display:"flex", alignItems:"center", justifyContent:"space-between" }}>
            <div>
              <div className="mono up" style={{ fontSize:9, color:"var(--st-good)" }}>{t("your_sealed_bid")}</div>
              <div className="mono" style={{ fontSize:20, color:"var(--gold-pale)", marginTop:4 }}>{D.usd0(sealed)}</div>
            </div>
            <span className="chip" data-st="good"><Icon name="lock" size={12}/> SEALED</span>
          </div>
          <p className="muted" style={{ fontSize:12, lineHeight:1.5, margin:"12px 0 0" }}>{t("sealed_until")}</p>
          <button className="btn btn-ghost" style={{ width:"100%", marginTop:14 }} onClick={()=>setEdit(true)}>{t("update_bid")}</button>
        </div>
      ) : (
        <>
          <Label>{t("enter_amount")}</Label>
          <input className="field field-mono" dir="ltr" inputMode="numeric" value={amt}
            onChange={e=>setAmt(e.target.value.replace(/[^\d]/g,""))} placeholder={String(lot.floor)} style={{ fontSize:18 }} />
          <div className="muted mono" style={{ fontSize:11, margin:"8px 2px 0" }}>{t("list_floor")}: {D.usd0(lot.floor)}</div>
          {noBids
            ? <NeedBids app={app} />
            : <button className="btn btn-gold" style={{ width:"100%", marginTop:14 }} disabled={!amt} onClick={submit}>
                <Icon name="lock" size={16}/> {t("submit_sealed")} · 1 {t("credit")}</button>}
        </>
      )}
    </div>
  );
}

/* ---------- UniqBid: many unique prices ---------- */
function UniqBidPanel({ app, lot }) {
  const { t } = app; const D = window.DATA;
  const taken = useRef(simTaken(lot)).current;
  const mine = app.placed[lot.id] || [];
  const [price, setPrice] = useState("");

  // combined multiplicity
  const counts = new Map();
  taken.forEach(p => counts.set(p, (counts.get(p) || 0) + 1));
  mine.forEach(p => counts.set(p, (counts.get(p) || 0) + 1));
  const uniques = [...counts.entries()].filter(([, c]) => c === 1).map(([p]) => p).sort((a, b) => a - b);
  const lowestUnique = uniques.length ? uniques[0] : null;
  const myUnique = mine.filter(p => counts.get(p) === 1).sort((a, b) => a - b);
  const myBest = myUnique.length ? myUnique[0] : null;
  const youLead = lowestUnique != null && myBest != null && myBest === lowestUnique;

  const place = () => {
    const v = parseInt(price.replace(/\D/g, ""), 10);
    if (!v) return;
    if (app.placeBid(lot.id, v)) setPrice("");
  };
  const noBids = app.bidWallet <= 0;
  return (
    <div>
      {/* standing */}
      <div style={{ display:"flex", gap:10, marginBottom:14 }}>
        <div style={{ flex:1, padding:"12px 14px", border:"1px solid var(--line)", borderRadius:"var(--r-2)", background:"var(--bg-1)" }}>
          <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{t("lowest_unique_now")}</div>
          <div className="mono" style={{ fontSize:17, color:"var(--gold-pale)", marginTop:4 }}>{lowestUnique != null ? D.usd0(lowestUnique) : "—"}</div>
        </div>
        <div style={{ flex:1, padding:"12px 14px", border:"1px solid", borderColor: youLead?"var(--st-good)":"var(--line)",
          borderRadius:"var(--r-2)", background: youLead?"var(--st-good-bg)":"var(--bg-1)" }}>
          <div className="mono up" style={{ fontSize:9, color: youLead?"var(--st-good)":"var(--fg-faint)" }}>{t("standing")}</div>
          <div style={{ fontSize:14, fontWeight:600, marginTop:5, color: youLead?"var(--st-good)":"var(--fg-muted)" }}>{youLead ? t("you_lead") : t("you_trail")}</div>
        </div>
      </div>

      {/* place */}
      <Label>{t("enter_amount")}</Label>
      <div style={{ display:"flex", gap:8 }}>
        <input className="field field-mono" dir="ltr" inputMode="numeric" value={price}
          onChange={e=>setPrice(e.target.value.replace(/[^\d]/g,""))} placeholder="e.g. 237" style={{ flex:1, fontSize:17 }}
          onKeyDown={e=>e.key==="Enter" && place()} />
        <button className="btn btn-gold" disabled={!price || noBids} onClick={place} style={{ whiteSpace:"nowrap" }}>
          <Icon name="coins" size={15}/> 1</button>
      </div>
      {noBids && <div style={{ marginTop:12 }}><NeedBids app={app} /></div>}

      {/* my bids */}
      {mine.length > 0 && (
        <div style={{ marginTop:16 }}>
          <div className="mono up" style={{ fontSize:10, color:"var(--fg-muted)", marginBottom:8 }}>{t("your_bids")} · {mine.length}</div>
          <div style={{ display:"flex", flexWrap:"wrap", gap:8 }}>
            {[...mine].reverse().map((p, i) => {
              const uniq = counts.get(p) === 1;
              return (
                <span key={i} className="chip" data-st={uniq?"good":"neut"} style={{ fontSize:11 }}>
                  <span className="mono">{D.usd0(p)}</span> · {t(uniq?"status_unique":"status_taken")}
                </span>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function NeedBids({ app }) {
  const { t, go } = app;
  return (
    <div style={{ marginTop:14, padding:"14px 16px", border:"1px solid var(--st-warn)", borderRadius:"var(--r-2)", background:"var(--st-warn-bg)" }}>
      <div style={{ display:"flex", alignItems:"center", gap:10, color:"var(--st-warn)", fontWeight:600, fontSize:13 }}>
        <Icon name="alert" size={16}/> {t("need_bids")}</div>
      <p className="muted" style={{ fontSize:12, margin:"8px 0 12px" }}>{t("need_bids_cta")}</p>
      <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>go("bidstore")}><Icon name="coins" size={16}/> {t("buy_bids")}</button>
    </div>
  );
}

/* ============================================================
   BID STORE — buy bid-credit packages
   ============================================================ */
function BidStoreScreen({ app }) {
  const { t, back } = app; const D = window.DATA;
  const [bought, setBought] = useState(null);
  const buy = (pkg) => { app.buyBids(pkg.bids); setBought(pkg); setTimeout(()=>setBought(null), 1800); };
  return (
    <ScreenShell top={<TopBar app={app} onBack={back} kicker={t("bid_wallet")} title={t("bid_store")} />}>
      <div style={{ padding:"18px 20px 40px" }}>
        <div style={{ display:"flex", alignItems:"center", gap:12, padding:"16px 18px", borderRadius:"var(--r-2)",
          border:"1px solid var(--gold-line)", background:"linear-gradient(100deg,var(--burg-deep),var(--bg-1))", marginBottom:18 }}>
          <span style={{ color:"var(--gold)" }}><Icon name="coins" size={26}/></span>
          <div style={{ flex:1 }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--gold)" }}>{t("bid_wallet")}</div>
            <div className="mono" style={{ fontSize:24, color:"var(--gold-pale)", marginTop:2 }}>{app.bidWallet} <span style={{ fontSize:12, color:"var(--fg-muted)" }}>{t("bid_credits")}</span></div>
          </div>
        </div>
        <p className="muted" style={{ fontSize:13, lineHeight:1.6, margin:"0 0 18px" }}>{t("bid_store_sub")}</p>

        <div style={{ display:"flex", flexDirection:"column", gap:12 }}>
          {D.BID_PACKAGES.map(pkg => (
            <div key={pkg.id} style={{ position:"relative", border:"1px solid", borderColor: pkg.best?"var(--gold)":"var(--line)",
              borderRadius:"var(--r-2)", background: pkg.best?"linear-gradient(120deg,var(--burg-deep),var(--bg-1))":"var(--bg-1)", padding:"18px 20px",
              display:"flex", alignItems:"center", gap:16 }}>
              {pkg.best && <span className="chip" data-st="live" style={{ position:"absolute", top:-10, insetInlineStart:16 }}>{t("best_value")}</span>}
              <div style={{ flex:1 }}>
                <div className="serif" style={{ fontSize:26, color:"var(--gold-pale)", lineHeight:1 }}>{pkg.bids} <span style={{ fontSize:13, color:"var(--fg-muted)", fontFamily:"var(--sans)" }}>{t("pkg_bids")}</span></div>
                <div className="mono" style={{ fontSize:11, color:"var(--fg-faint)", marginTop:6 }}>${pkg.perBid.toFixed(2)} {t("per_bid")}{pkg.save!=="—" ? ` · ${t("pkg_save")} ${pkg.save}` : ""}</div>
              </div>
              <button className={"btn "+(pkg.best?"btn-gold":"btn-ghost")} onClick={()=>buy(pkg)} style={{ minWidth:104, flexDirection:"column", height:"auto", padding:"10px 16px", gap:2 }}>
                <span className="mono" style={{ fontSize:16, fontWeight:600 }}>${pkg.price}</span>
                <span style={{ fontSize:11 }}>{t("pkg_buy")}</span>
              </button>
            </div>
          ))}
        </div>

        {bought && (
          <div className="fade-up" style={{ marginTop:18, padding:"14px 16px", border:"1px solid var(--st-good)", borderRadius:"var(--r-2)",
            background:"var(--st-good-bg)", display:"flex", alignItems:"center", gap:10, color:"var(--st-good)", fontWeight:600, fontSize:13.5 }}>
            <Icon name="check" size={18}/> +{bought.bids} {t("bought_bids")}
          </div>
        )}
      </div>
    </ScreenShell>
  );
}

/* ============================================================
   LIST-TO-AUCTION SHEET — owner sets type + duration
   ============================================================ */
function ListToAuctionSheet({ app, item, onClose, onSubmit }) {
  const { t, lang } = app; const D = window.DATA;
  const [atype, setAtype] = useState("dutch");
  const [dur, setDur] = useState(5);
  const types = [
    { k:"dutch",   icon:"flame" },
    { k:"vickrey", icon:"file" },
    { k:"uniqbid", icon:"hash" },
  ];
  const durs = [2, 5, 7];
  const timed = atype !== "dutch";
  return (
    <div onClick={onClose} style={{ position:"absolute", inset:0, zIndex:70, background:"rgba(8,5,6,0.65)",
      backdropFilter:"blur(6px)", WebkitBackdropFilter:"blur(6px)", display:"flex", alignItems:"flex-end" }}>
      <div onClick={e=>e.stopPropagation()} className="fade-up" style={{ width:"100%", background:"var(--bg-1)",
        borderTop:"1px solid var(--gold-line)", borderRadius:"16px 16px 0 0", padding:"22px 20px 34px", maxHeight:"92%", overflowY:"auto" }}>
        <div style={{ width:40, height:4, background:"var(--line-strong)", borderRadius:2, margin:"0 auto 18px" }} />
        <div className="serif" style={{ fontSize:20, color:"var(--gold-pale)" }}>{t("list_title")}</div>
        <div className="muted" style={{ fontSize:12.5, margin:"4px 0 16px" }}>{item ? D.tt(item.title, lang) : ""} · {t("list_sub")}</div>

        <Label>{t("list_choose_type")}</Label>
        <div style={{ display:"flex", flexDirection:"column", gap:8, marginBottom:18 }}>
          {types.map(ty => {
            const on = atype === ty.k;
            return (
              <button key={ty.k} onClick={()=>setAtype(ty.k)} style={{ textAlign:"start", cursor:"pointer",
                border:"1px solid", borderColor:on?"var(--gold)":"var(--line-strong)", borderRadius:"var(--r-2)",
                background:on?"linear-gradient(110deg,var(--burg-deep),var(--bg-1))":"var(--bg-0)", padding:"13px 15px",
                display:"flex", alignItems:"center", gap:12, color:"var(--fg)" }}>
                <span style={{ color:on?"var(--gold)":"var(--fg-muted)" }}><Icon name={ty.icon} size={18}/></span>
                <div style={{ flex:1 }}>
                  <div style={{ fontWeight:600, fontSize:14 }}>{t("mode_"+ty.k)}</div>
                  <div className="muted" style={{ fontSize:11, marginTop:2 }}>{D.ATYPES[ty.k][lang]}</div>
                </div>
                <span style={{ width:18, height:18, borderRadius:"50%", border:"2px solid", borderColor:on?"var(--gold)":"var(--line-strong)",
                  display:"flex", alignItems:"center", justifyContent:"center" }}>
                  {on && <span style={{ width:8, height:8, borderRadius:"50%", background:"var(--gold)" }} />}</span>
              </button>
            );
          })}
        </div>

        {/* duration — only for timed (passive) types, set by owner */}
        <div style={{ opacity: timed?1:0.4, pointerEvents: timed?"auto":"none", transition:"opacity .2s" }}>
          <Label>{t("list_duration")} · {t("set_by_owner")}</Label>
          <div style={{ display:"flex", gap:8, marginBottom:18 }}>
            {durs.map(d => (
              <button key={d} onClick={()=>setDur(d)} className="mono" style={{ flex:1, padding:"13px", borderRadius:"var(--r-1)", cursor:"pointer",
                border:"1px solid", borderColor: dur===d&&timed?"var(--gold)":"var(--line-strong)",
                background: dur===d&&timed?"var(--gold)":"transparent", color: dur===d&&timed?"#1B1207":"var(--fg-muted)", fontWeight:600, fontSize:14 }}>
                {t("dur_"+d+"d")}</button>
            ))}
          </div>
        </div>

        <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>onSubmit({ atype, dur: timed?dur:null })}>{t("list_confirm")}</button>
      </div>
    </div>
  );
}

Object.assign(window, { PassiveAuctionScreen, BidStoreScreen, ListToAuctionSheet, useCountdown, CountdownPill });
