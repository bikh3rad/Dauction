/* ============================================================
   DAUCTION — LIVE DUTCH AUCTION (hero) + ESCROW FLOW
   Layout variants via tw.auctionLayout: 'stage' | 'ledger' | 'split'
   FOMO scaled 0–10 via tw.fomo: drop speed, room ticker, toasts, heat.
   ============================================================ */

/* descending-price engine */
function useDutchAuction(lot, fomo, running) {
  const startRef = useRef(Date.now());
  const [now, setNow] = useState(Date.now());
  useInterval(() => setNow(Date.now()), 100, running);
  const every = Math.max(8, lot.dropEvery * (1.2 - fomo * 0.04)); // gentler; fomo nudges speed only slightly
  const elapsed = (now - startRef.current) / 1000;
  const drops = Math.floor(elapsed / every);
  const price = Math.max(lot.floor, lot.start - lot.dropStep * drops);
  const atFloor = price <= lot.floor;
  const nextIn = atFloor ? 0 : every - (elapsed % every);
  const span = lot.start - lot.floor;
  const progress = span > 0 ? 1 - (price - lot.floor) / span : 1; // 0→1 as it falls
  return { price, nextIn, atFloor, progress, drops, every, reset: () => (startRef.current = Date.now()) };
}

/* live "room" ticker + activity toasts, scaled by FOMO */
function useRoom(lot, fomo, active) {
  const [room, setRoom] = useState(lot.participants);
  const [watch, setWatch] = useState(lot.watching);
  const [toast, setToast] = useState(null);
  const actors = ["0x9F", "0x4C", "0x11", "0xB2", "0x7E", "0xA1", "0x2D", "0xF7"];
  useInterval(() => {
    const r = Math.random();
    setWatch(w => Math.max(lot.watching - 20, w + (r < 0.5 ? 1 : -1) + Math.round(fomo / 4)));
    if (r < 0.18 + fomo * 0.05) {
      const a = actors[Math.floor(Math.random() * actors.length)];
      const kinds = ["locked a deposit", "entered the room", "is watching"];
      const k = kinds[Math.floor(Math.random() * kinds.length)];
      if (k === "locked a deposit") setRoom(n => n + 1);
      setToast({ id: Date.now(), txt: `${a} ${k}` });
    }
  }, Math.max(1400, 4200 - fomo * 320), active);
  useEffect(() => { if (!toast) return; const id = setTimeout(() => setToast(null), 2600); return () => clearTimeout(id); }, [toast]);
  return { room, watch, toast };
}

function HeatRing({ progress, nextIn, every, size = 132, atFloor }) {
  const r = size / 2 - 8, c = 2 * Math.PI * r;
  const frac = every > 0 ? 1 - nextIn / every : 1; // fill toward next drop
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} style={{ transform:"rotate(-90deg)" }}>
      <circle cx={size/2} cy={size/2} r={r} fill="none" stroke="var(--line)" strokeWidth="3" />
      <circle cx={size/2} cy={size/2} r={r} fill="none" stroke="var(--gold)" strokeWidth="3"
        strokeDasharray={c} strokeDashoffset={c * (1 - frac)} strokeLinecap="round"
        style={{ transition: "stroke-dashoffset 100ms linear", filter: atFloor?"none":"drop-shadow(0 0 4px var(--gold-glow))" }} />
    </svg>
  );
}

function ActivityToast({ toast, app }) {
  if (!toast) return null;
  return (
    <div key={toast.id} className="fade-up" style={{ position:"absolute", top:14, insetInlineStart:16, insetInlineEnd:16, zIndex:20,
      display:"flex", alignItems:"center", gap:8, padding:"8px 12px", borderRadius:"var(--r-pill)",
      background:"rgba(20,12,16,0.92)", border:"1px solid var(--gold-line)", boxShadow:"var(--shadow-card)", width:"fit-content" }}>
      <span className="dot" style={{ width:6, height:6, borderRadius:"50%", background:"var(--gold)" }} />
      <span className="mono" style={{ fontSize:11.5, color:"var(--fg)" }}>{toast.txt}</span>
    </div>
  );
}

/* PRICE numerals — big, flashes on drop */
function PriceBig({ price, flash, size = 46 }) {
  return (
    <div className="mono tnum" style={{ fontSize:size, fontWeight:600, lineHeight:1, letterSpacing:"-0.02em",
      color: flash ? "var(--gold-bright)" : "var(--gold-pale)", transition:"color 320ms var(--ease-doc)",
      textShadow: flash ? "0 0 18px var(--gold-glow)" : "none" }}>
      ${price.toLocaleString("en-US")}
    </div>
  );
}

function AuctionScreen({ app }) {
  const { t, back, go, tw, lang } = app;
  const D = window.DATA;
  const lot = D.lot(app.nav.params.lotId) || D.LOTS.find(l=>l.live) || D.LOTS[0];
  const fomo = tw.fomo;
  const [locked, setLocked] = useState(false);
  const [lockSheet, setLockSheet] = useState(false);
  const [won, setWon] = useState(false);
  const [flash, setFlash] = useState(false);
  const auc = useDutchAuction(lot, fomo, !won);
  const room = useRoom(lot, fomo, !won);
  const prevDrops = useRef(auc.drops);
  const deposit = Math.round(lot.floor * 0.10);

  useEffect(() => {
    if (auc.drops !== prevDrops.current) { prevDrops.current = auc.drops; setFlash(true); const id = setTimeout(()=>setFlash(false), 360); return ()=>clearTimeout(id); }
  }, [auc.drops]);

  const buy = () => { setWon(true); };
  const toEscrow = () => go("escrow", { lotId: lot.id, hammer: auc.price });
  const fullAmount = lot.floor; // 100% locks at open (10% reserve already held)

  const layout = tw.auctionLayout; // stage | ledger | split
  const heatStyle = fomo >= 6 && auc.progress > 0.55;

  /* ---- WON overlay ---- */
  if (won) return <WonView app={app} lot={lot} hammer={auc.price} onContinue={toEscrow} onBack={()=>{setWon(false); auc.reset();}} />;

  const RoomBar = (
    <div style={{ display:"flex", alignItems:"center", gap:14, fontSize:12, color:"var(--fg-muted)" }}>
      <span style={{ display:"flex", alignItems:"center", gap:5 }}><Icon name="users" size={14}/><span className="mono">{room.room}</span> {t("auc_participants")}</span>
      <span style={{ display:"flex", alignItems:"center", gap:5 }}><Icon name="eye" size={14}/><span className="mono">{room.watch}</span></span>
    </div>
  );

  const BuyBar = (
    <div style={{ zIndex:25, padding:"14px 16px 24px", background:"var(--bg-1)", borderTop:"1px solid var(--gold-line)" }}>
      {!locked ? (
        <>
          <div className="muted" style={{ fontSize:11.5, textAlign:"center", marginBottom:9, display:"flex", gap:6, justifyContent:"center", alignItems:"center" }}>
            <Icon name="lock" size={13}/>{t("lockfull_body")}</div>
          <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>setLockSheet(true)}>
            <Icon name="lock" size={17}/> {t("lockfull_cta")} · {D.usd0(fullAmount)}
          </button>
        </>
      ) : (
        <>
          <div style={{ display:"flex", alignItems:"center", justifyContent:"center", gap:8, marginBottom:9 }}>
            <Chip state="active" label={t("full_locked")} /><span className="mono" style={{ fontSize:11, color:"var(--fg-muted)" }}>{D.usd0(fullAmount)}</span>
          </div>
          <button className={"btn btn-gold"+(fomo>=7?" buypulse":"")} style={{ width:"100%", fontSize:17, padding:"16px" }} onClick={buy}>
            {t("auc_buy")} ${auc.price.toLocaleString("en-US")}
          </button>
        </>
      )}
    </div>
  );

  return (
    <>
    <ScreenShell
      bg={heatStyle ? "radial-gradient(120% 60% at 50% 30%, rgba(201,162,75,0.10), var(--bg-void) 70%)" : "var(--bg-void)"}
      top={<>
        <TopBar app={app} kicker={`${t("gal_lot")} ${String(lot.no).padStart(2,"0")} · ${lot.ref}`} title={lot.maison} onBack={back}
          right={<span className="chip" data-st="live" style={{ alignSelf:"center" }}><span className="dot"/>{t("auc_live")}</span>} />
        {fomo >= 4 && (
          <div style={{ display:"flex", alignItems:"center", justifyContent:"center", gap:8, padding:"9px 16px",
            background: heatStyle?"var(--burg)":"var(--bg-1)", borderBottom:"1px solid var(--gold-line)", color:"var(--gold-pale)", fontSize:12.5 }}>
            <Icon name="flame" size={15} style={{ color:"var(--gold)" }} />
            <span style={{ fontWeight:600 }}>{t("auc_dropping")}</span>
            <span className="mono" style={{ opacity:0.8 }}>· −{D.usd0(lot.dropStep)} / {Math.round(auc.every)}{t("auc_secs")}</span>
          </div>
        )}
      </>}
      footer={BuyBar}>
      <div style={{ position:"relative", minHeight:"100%" }}>
        <ActivityToast toast={room.toast} app={app} />
        {layout === "ledger" && <LedgerLayout {...{app,lot,auc,flash,RoomBar,t}} />}
        {layout === "split"  && <SplitLayout  {...{app,lot,auc,flash,RoomBar,t}} />}
        {(layout === "stage" || !layout) && <StageLayout {...{app,lot,auc,flash,RoomBar,t}} />}
      </div>
    </ScreenShell>
    {lockSheet && <LockSheet app={app} lot={lot} deposit={deposit} onClose={()=>setLockSheet(false)} onLock={()=>{ setLocked(true); setLockSheet(false); }} />}
    </>
  );
}

/* ---- LAYOUT A: STAGE (dramatic, centered) ---- */
function StageLayout({ app, lot, auc, flash, RoomBar, t }) {
  return (
    <div style={{ padding:"22px 20px 0", display:"flex", flexDirection:"column", alignItems:"center", textAlign:"center" }}>
      <div style={{ position:"relative", width:200, height:200, marginBottom:18 }}>
        <Ph art={window.DATA.CATS[lot.cat].g} ratio="1 / 1" label={lot.maison} style={{ borderRadius:"var(--r-3)", width:"100%", height:"100%" }} />
        <div style={{ position:"absolute", inset:-10, display:"flex", alignItems:"center", justifyContent:"center", pointerEvents:"none" }}>
          <HeatRing progress={auc.progress} nextIn={auc.nextIn} every={auc.every} size={224} atFloor={auc.atFloor} />
        </div>
      </div>
      <div className="serif" style={{ fontSize:19, color:"var(--gold-pale)", lineHeight:1.2, marginBottom:18, maxWidth:280 }}>{window.DATA.tt(lot.title, app.lang)}</div>

      <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:6 }}>{t("auc_current")}</div>
      <PriceBig price={auc.price} flash={flash} size={52} />
      <div style={{ display:"flex", gap:18, marginTop:14, marginBottom:16 }}>
        <MiniStat label={t("auc_floor")} value={`$${lot.floor.toLocaleString("en-US")}`} />
        <MiniStat label={t("auc_next")} value={auc.atFloor?"—":`${Math.ceil(auc.nextIn)}${t("auc_secs")}`} gold />
      </div>
      {RoomBar}
    </div>
  );
}

/* ---- LAYOUT B: LEDGER (manifest / protocol) ---- */
function LedgerLayout({ app, lot, auc, flash, RoomBar, t }) {
  const D = window.DATA;
  return (
    <div style={{ padding:"4px 0 0" }}>
      <Ph art={D.CATS[lot.cat].g} ratio="16 / 9" label={`${lot.maison} · ${window.DATA.tt(lot.title, app.lang)}`} style={{ borderRadius:0, borderInline:0 }} />
      <div style={{ padding:"18px 20px 0" }}>
        <div className="serif" style={{ fontSize:18, color:"var(--gold-pale)", marginBottom:14 }}>{window.DATA.tt(lot.title, app.lang)}</div>
        <div style={{ border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", overflow:"hidden", background:"var(--bg-1)" }}>
          <div style={{ padding:"16px 16px", background:"linear-gradient(90deg,var(--burg-deep),transparent)", borderBottom:"1px solid var(--line)" }}>
            <div className="mono up" style={{ fontSize:9.5, color:"var(--gold)", marginBottom:6 }}>{t("auc_current")}</div>
            <PriceBig price={auc.price} flash={flash} size={40} />
          </div>
          <LedRow k={t("auc_floor")} v={`$${lot.floor.toLocaleString("en-US")} USDC`} />
          <LedRow k={t("auc_step")} v={`−$${lot.dropStep.toLocaleString("en-US")} / ${Math.round(auc.every)}${t("auc_secs")}`} />
          <LedRow k={t("auc_next")} v={auc.atFloor?"AT_FLOOR":`${Math.ceil(auc.nextIn)} ${t("auc_secs")}`} gold last />
        </div>
        <div style={{ marginTop:14 }}>{RoomBar}</div>
      </div>
    </div>
  );
}
function LedRow({ k, v, gold, last }) {
  return (
    <div style={{ display:"flex", justifyContent:"space-between", alignItems:"center", padding:"12px 16px", borderBottom: last?"none":"1px solid var(--line)" }}>
      <span className="mono up" style={{ fontSize:10, color:"var(--fg-muted)" }}>{k}</span>
      <span className="mono" style={{ fontSize:13, color: gold?"var(--gold-pale)":"var(--fg)" }}>{v}</span>
    </div>
  );
}

/* ---- LAYOUT C: SPLIT (compact) ---- */
function SplitLayout({ app, lot, auc, flash, RoomBar, t }) {
  const D = window.DATA;
  return (
    <div style={{ padding:"18px 18px 0" }}>
      <div style={{ display:"flex", gap:14, marginBottom:16 }}>
        <Ph art={D.CATS[lot.cat].g} label={lot.maison} style={{ width:104, height:104, borderRadius:"var(--r-2)", flexShrink:0 }} ratio={null} />
        <div style={{ minWidth:0 }}>
          <div className="serif" style={{ fontSize:16, color:"var(--gold-pale)", lineHeight:1.2 }}>{window.DATA.tt(lot.title, app.lang)}</div>
          <div className="muted" style={{ fontSize:12, marginTop:4 }}>{lot.condition} · {lot.year}</div>
          <div style={{ marginTop:10 }}>{RoomBar}</div>
        </div>
      </div>
      <div style={{ border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", padding:"16px 18px", background:"linear-gradient(120deg,var(--burg-deep),var(--bg-1))" }}>
        <div style={{ display:"flex", justifyContent:"space-between", alignItems:"flex-end" }}>
          <div>
            <div className="mono up" style={{ fontSize:9.5, color:"var(--gold)", marginBottom:6 }}>{t("auc_current")}</div>
            <PriceBig price={auc.price} flash={flash} size={38} />
          </div>
          <div style={{ textAlign:"end" }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:4 }}>{t("auc_next")}</div>
            <div className="mono" style={{ fontSize:20, color:"var(--gold-pale)" }}>{auc.atFloor?"—":`${Math.ceil(auc.nextIn)}${t("auc_secs")}`}</div>
          </div>
        </div>
        <div style={{ height:5, background:"var(--bg-0)", borderRadius:3, marginTop:14, overflow:"hidden" }}>
          <div style={{ width:`${Math.round(auc.progress*100)}%`, height:"100%", background:"var(--gold)", transition:"width .3s var(--ease-mech)" }} />
        </div>
        <div style={{ display:"flex", justifyContent:"space-between", marginTop:6 }}>
          <span className="mono" style={{ fontSize:10, color:"var(--fg-faint)" }}>${lot.floor.toLocaleString("en-US")}</span>
          <span className="mono" style={{ fontSize:10, color:"var(--fg-faint)" }}>${lot.start.toLocaleString("en-US")}</span>
        </div>
      </div>
    </div>
  );
}
function MiniStat({ label, value, gold }) {
  return (
    <div style={{ textAlign:"center" }}>
      <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:4 }}>{label}</div>
      <div className="mono" style={{ fontSize:16, color: gold?"var(--gold-pale)":"var(--fg)" }}>{value}</div>
    </div>
  );
}

/* ---- deposit lock bottom sheet ---- */
function LockSheet({ app, lot, deposit, onClose, onLock }) {
  const { t } = app; const D = window.DATA; const M = D.MEMBER;
  return (
    <div onClick={onClose} style={{ position:"absolute", inset:0, zIndex:70, background:"rgba(8,5,6,0.65)",
      backdropFilter:"blur(6px)", WebkitBackdropFilter:"blur(6px)", display:"flex", alignItems:"flex-end" }}>
      <div onClick={e=>e.stopPropagation()} className="fade-up" style={{ width:"100%", background:"var(--bg-1)",
        borderTop:"1px solid var(--gold-line)", borderRadius:"16px 16px 0 0", padding:"22px 20px 34px" }}>
        <div style={{ width:40, height:4, background:"var(--line-strong)", borderRadius:2, margin:"0 auto 18px" }} />
        <div style={{ display:"flex", alignItems:"center", gap:10, marginBottom:6 }}>
          <span style={{ color:"var(--gold)" }}><Icon name="lock" size={20}/></span>
          <div className="serif" style={{ fontSize:20, color:"var(--gold-pale)" }}>{t("lockfull_title")}</div>
        </div>
        <p className="muted" style={{ fontSize:13, lineHeight:1.6, margin:"0 0 18px" }}>{t("lockfull_body")}</p>
        <div className="kv" style={{ marginBottom:20 }}>
          <div className="k">{t("req_deposit")} · 10%</div><div className="v" style={{ color:"var(--st-good)" }}>✓ {D.usd(deposit)}</div>
          <div className="k">{t("lockfull_title")} · 100%</div><div className="v" style={{ color:"var(--gold-pale)" }}>{D.usd(lot.floor)}</div>
          <div className="k">{t("esc_balance")}</div><div className="v">{D.usd(M.walletUSDC)}</div>
        </div>
        <button className="btn btn-gold" style={{ width:"100%" }} onClick={onLock}>
          <Icon name="lock" size={17}/> {t("lockfull_cta")} · {D.usd0(lot.floor)}
        </button>
      </div>
    </div>
  );
}

/* ---- WON view — stamp the win ---- */
function WonView({ app, lot, hammer, onContinue, onBack }) {
  const { t } = app;
  return (
    <div style={{ height:"100%", display:"flex", flexDirection:"column", padding:"0 24px",
      background:"radial-gradient(120% 70% at 50% 0%, var(--burg), var(--bg-void) 65%)" }}>
      <TopBar app={app} kicker={lot.ref} title={lot.maison} onBack={onBack} />
      <div style={{ flex:1, display:"flex", flexDirection:"column", alignItems:"center", justifyContent:"center", textAlign:"center", gap:6 }}>
        <div className="stampin"><Seal size={120} label="HAMMER" sub="DAUCTION" id="0x7A4E" date="LOT 07" live /></div>
        <div className="mono up" style={{ fontSize:11, color:"var(--gold)", letterSpacing:"0.2em", marginTop:14 }}>{t("auc_won")}</div>
        <div className="serif" style={{ fontSize:22, color:"var(--gold-pale)", lineHeight:1.2, maxWidth:280 }}>{window.DATA.tt(lot.title, app.lang)}</div>
        <div style={{ marginTop:14 }}><div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:6 }}>{t("esc_hammer")}</div>
          <PriceBig price={hammer} size={42} /></div>
      </div>
      <div style={{ paddingBottom:40 }}>
        <button className="btn btn-gold" style={{ width:"100%" }} onClick={onContinue}>{t("esc_complete_title")} <Icon name="arrow-right" size={18}/></button>
      </div>
    </div>
  );
}

/* ============================================================
   ESCROW FLOW  — complete → funded → release → done
   ============================================================ */
function EscrowScreen({ app }) {
  const { t, go, back } = app;
  const D = window.DATA;
  const lot = D.lot(app.nav.params.lotId) || D.LOTS[0];
  const hammer = app.nav.params.hammer || lot.floor;
  const premium = Math.round(hammer * lot.premium);
  const total = hammer + premium;
  const deposit = Math.round(lot.floor * 0.10);
  const [phase, setPhase] = useState("complete"); // complete | funded | release | done
  const [secs, setSecs] = useState(24 * 3600 - 12);
  useInterval(() => setSecs(s => s - 37), 1000, phase === "complete"); // accelerated demo clock

  const flow = ["complete","funded","release","done"];
  const idx = flow.indexOf(phase);

  return (
    <ScreenShell top={<TopBar app={app} kicker={lot.ref} title={t("esc_title")} onBack={back} />}>
      <div style={{ padding:"16px 20px 40px" }}>
        {/* escrow state diagram */}
        <div style={{ display:"flex", gap:6, marginBottom:22, overflowX:"auto" }} className="noscroll">
          {[["FUNDED","funded"],["IN_TRANSIT","release"],["DELIVERED","release"],["COMPLETED","done"]].map(([s,ph],i)=>{
            const reached = flow.indexOf(ph) <= idx || (i===0 && idx>=0);
            return <React.Fragment key={i}>
              <span className="chip" data-st={reached?(s==="COMPLETED"?"good":"active"):"neut"} style={{ opacity:reached?1:0.45 }}>{s}</span>
              {i<3 && <span style={{ color:"var(--fg-faint)", alignSelf:"center" }}>›</span>}
            </React.Fragment>;
          })}
        </div>

        {phase === "complete" && (
          <div className="fade-up">
            <h2 className="serif" style={{ fontSize:22, color:"var(--gold-pale)", margin:"0 0 8px" }}>{t("esc_complete_title")}</h2>
            <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:"0 0 18px" }}>{t("esc_complete_body")}</p>
            <div style={{ border:"1px solid var(--st-warn)", borderRadius:"var(--r-2)", padding:"14px 16px", marginBottom:18,
              background:"var(--st-warn-bg)", display:"flex", alignItems:"center", gap:12 }}>
              <Icon name="clock" size={20} style={{ color:"var(--st-warn)" }}/>
              <div style={{ flex:1 }}><div className="mono up" style={{ fontSize:9, color:"var(--st-warn)" }}>{t("esc_remaining")}</div>
                <div className="mono" style={{ fontSize:20, color:"var(--fg)" }}>{fmtClock(secs)}</div></div>
            </div>
            <div className="kv" style={{ marginBottom:20 }}>
              <div className="k">{t("esc_hammer")}</div><div className="v">{D.usd(hammer)}</div>
              <div className="k">{t("esc_locked")} · 10%</div><div className="v" style={{ color:"var(--st-good)" }}>− {D.usd(deposit)}</div>
              <div className="k">{t("auc_premium")} · {Math.round(lot.premium*100)}%</div><div className="v">{D.usd(premium)}</div>
              <div className="k" style={{ fontWeight:700, color:"var(--fg)" }}>{t("esc_total")}</div><div className="v" style={{ color:"var(--gold-pale)", fontSize:15 }}>{D.usd(total - deposit)}</div>
            </div>
            <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>setPhase("funded")}>{t("esc_fund")} · {D.usd0(total-deposit)}</button>
          </div>
        )}

        {phase === "funded" && (
          <div className="fade-up" style={{ textAlign:"center", paddingTop:10 }}>
            <Seal size={104} label="FUNDED" sub="ESCROW" id="0x7A4E" date="HELD" live />
            <div style={{ marginTop:14 }}><Chip state="funded" label="FUNDED" /></div>
            <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:"14px auto 0", maxWidth:280 }}>{t("esc_funded")}.</p>
            <div className="kv" style={{ marginTop:20, textAlign:"start" }}>
              <div className="k">{t("esc_total")}</div><div className="v" style={{ color:"var(--gold-pale)" }}>{D.usd(total)}</div>
              <div className="k">{t("esc_release_title")}</div><div className="v"><span className="chip" data-st="active">IN_TRANSIT</span></div>
            </div>
            <button className="btn btn-gold" style={{ width:"100%", marginTop:22 }} onClick={()=>setPhase("release")}>{t("common_continue")} <Icon name="arrow-right" size={18}/></button>
          </div>
        )}

        {phase === "release" && (
          <div className="fade-up">
            <h2 className="serif" style={{ fontSize:22, color:"var(--gold-pale)", margin:"0 0 8px" }}>{t("esc_release_title")}</h2>
            <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:"0 0 18px" }}>{t("esc_release_body")}</p>
            <div style={{ position:"relative", marginBottom:18 }}>
              <Ph art={D.CATS[lot.cat].g} ratio="4 / 3" label={`${lot.maison} · in hand`} style={{ borderRadius:"var(--r-2)" }} />
              <div style={{ position:"absolute", top:12, insetInlineStart:12 }}><span className="chip" data-st="active"><span className="dot"/>IN_TRANSIT</span></div>
            </div>
            <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>setPhase("done")}>
              <Icon name="check" size={18}/> {t("esc_release")}
            </button>
          </div>
        )}

        {phase === "done" && (
          <div className="fade-up" style={{ textAlign:"center", paddingTop:10 }}>
            <div className="stampin"><Seal size={112} label="RELEASED" sub="DAUCTION" id="0x7A4E" date="COMPLETE" live /></div>
            <div style={{ marginTop:14 }}><Chip state="completed" label="COMPLETED" /></div>
            <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:"14px auto 0", maxWidth:290 }}>{t("esc_released")}. {t("esc_release_body")}</p>
            <div style={{ marginTop:18, padding:"14px 16px", border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", background:"var(--bg-1)" }}>
              <div className="mono up" style={{ fontSize:9, color:"var(--gold)", marginBottom:6 }}>seller payout · 110% {t("clo_creditopt")}</div>
              <Money n={Math.round(hammer*1.10)} big gold cents={false} />
            </div>
            <button className="btn btn-gold" style={{ width:"100%", marginTop:22 }} onClick={()=>go("gallery")}>{t("nav_gallery")}</button>
          </div>
        )}
      </div>
    </ScreenShell>
  );
}

Object.assign(window, { AuctionScreen, EscrowScreen, useDutchAuction, useRoom, HeatRing, PriceBig, ActivityToast, LockSheet, WonView, StageLayout, MiniStat });
