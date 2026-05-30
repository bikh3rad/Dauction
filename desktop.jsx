/* ============================================================
   DAUCTION — DESKTOP buyer app
   Same data + auction engine as mobile, re-laid-out for a wide screen.
   Core surfaces are desktop-native (gallery, lot, auction, vault);
   forms/flows reuse the mobile screens inside a centered column.
   ============================================================ */

/* ---------- desktop chrome ---------- */
function DeskNav({ app }) {
  const { t, go, nav, tier } = app;
  const D = window.DATA; const M = D.MEMBER;
  const items = [
    { k:"gallery", label:t("nav_gallery") },
    { k:"vault", label:t("nav_closet") },
    { k:"membership", label:t("nav_membership") },
  ];
  const active = nav.screen;
  const inGallery = ["gallery","lot","auction","escrow"].includes(active);
  return (
    <div style={{ height:64, flexShrink:0, display:"flex", alignItems:"center", gap:28, padding:"0 30px",
      borderBottom:"1px solid var(--gold-line)", background:"rgba(20,12,16,0.7)", backdropFilter:"blur(12px)" }}>
      <button onClick={()=>go("gallery")} style={{ display:"flex", alignItems:"center", gap:10, background:"none", border:"none", cursor:"pointer", color:"var(--gold-pale)" }}>
        <Icon name="crown" size={22} style={{ color:"var(--gold)" }} />
        <span className="serif" style={{ fontSize:21, letterSpacing:"0.04em" }}>{t("brand")}</span>
      </button>
      <div style={{ display:"flex", gap:4 }}>
        {items.map(it=>{
          const on = it.k===active || (it.k==="gallery" && inGallery);
          return (
            <button key={it.k} onClick={()=>go(it.k)} style={{ background:"none", border:"none", cursor:"pointer",
              padding:"8px 14px", borderRadius:"var(--r-1)", fontSize:14.5, fontFamily:"var(--sans)",
              fontWeight:on?600:500, color:on?"var(--gold-pale)":"var(--fg-muted)",
              borderBottom:"2px solid", borderBottomColor:on?"var(--gold)":"transparent" }}>{it.label}</button>
          );
        })}
      </div>
      <div style={{ flex:1 }} />
      <div style={{ display:"flex", alignItems:"center", gap:18 }}>
        <div style={{ textAlign:"end" }}>
          <div className="mono up" style={{ fontSize:8.5, color:"var(--fg-faint)" }}>{t("desk_wallet")}</div>
          <div className="mono" style={{ fontSize:13, color:"var(--gold-pale)" }}>{D.usd0(M.walletUSDC)}</div>
        </div>
        <Chip state={tier==="guest"?"neut":"active"} label={t(tier==="vip"?"mem_vip":tier==="member"?"member_tag":"guest_tag").toUpperCase()} />
        <button onClick={()=>go("bidstore")} className="mono" style={{ display:"flex", alignItems:"center", gap:6, padding:"7px 12px", borderRadius:"var(--r-pill)", border:"1px solid var(--gold-line)", background:"var(--bg-1)", color:"var(--gold-pale)", cursor:"pointer", fontSize:12.5 }}>
          <Icon name="coins" size={15} style={{ color:"var(--gold)" }}/> {app.bidWallet}</button>
        <LangPill app={app} />
        <button onClick={()=>go("account")} aria-label={t("desk_nav_account")} style={{ width:40, height:40, borderRadius:"50%",
          border:"1px solid var(--gold-line)", background:"var(--burg)", color:"var(--gold-pale)", cursor:"pointer",
          display:"flex", alignItems:"center", justifyContent:"center" }}><Icon name="user" size={20} /></button>
      </div>
    </div>
  );
}

/* a centered, phone-width column for forms/flows reused from mobile */
function CenterCol({ children, w = 460 }) {
  return (
    <div style={{ height:"100%", overflow:"auto", display:"flex", justifyContent:"center", padding:"30px 20px" }}>
      <div style={{ width:w, maxWidth:"100%", minHeight:"min-content", border:"1px solid var(--line)", borderRadius:"var(--r-3)",
        overflow:"hidden", background:"var(--bg-void)", boxShadow:"var(--shadow-modal)", alignSelf:"flex-start", height:700 }}>
        {children}
      </div>
    </div>
  );
}

/* ============================================================
   DESKTOP GALLERY — hero live lot + wide grid
   ============================================================ */
function DeskGallery({ app }) {
  const { t, go, lang } = app; const D = window.DATA;
  const [filter, setFilter] = useState("all");
  const live = D.LOTS.find(l => l.live);
  const list = filter==="upcoming" ? D.LOTS.filter(l=>l.status==="upcoming")
    : filter==="live" ? D.LOTS.filter(l=>l.live) : D.LOTS;
  return (
    <div style={{ height:"100%", overflow:"auto" }}>
      <div style={{ maxWidth:1160, margin:"0 auto", padding:"30px 30px 50px" }}>
        {/* guest banner */}
        {app.tier==="guest" && (
          <button onClick={()=>go("invite")} style={{ width:"100%", cursor:"pointer", textAlign:"start", marginBottom:24,
            border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", background:"linear-gradient(110deg,var(--burg),var(--bg-1) 70%)",
            padding:"16px 22px", display:"flex", alignItems:"center", gap:16, color:"var(--fg)" }}>
            <Icon name="crown" size={24} style={{ color:"var(--gold)" }} />
            <span style={{ flex:1, fontSize:14.5 }}>{t("guest_banner")}</span>
            <span className="btn btn-burg" style={{ pointerEvents:"none", padding:"9px 16px" }}>{t("enter_invite")}</span>
          </button>
        )}

        {/* header */}
        <div style={{ display:"flex", alignItems:"flex-end", justifyContent:"space-between", marginBottom:8 }}>
          <div>
            <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:8 }}>{t("gal_kicker")}</div>
            <h1 className="serif" style={{ fontSize:40, margin:0, color:"var(--gold-pale)", lineHeight:1 }}>{t("gal_title")}</h1>
          </div>
          <div style={{ display:"flex", alignItems:"center", gap:14 }}>
            <span className="mono" style={{ fontSize:12, color:"var(--fg-muted)" }}>06 / 32 · {t("gal_supply")}</span>
            <div style={{ display:"flex", gap:8 }}>
              {[["all",t("gal_all")],["live",t("gal_live")],["upcoming",t("gal_upcoming")]].map(([k,lbl])=>(
                <button key={k} onClick={()=>setFilter(k)} className="mono" style={{ cursor:"pointer", padding:"8px 15px",
                  borderRadius:"var(--r-pill)", fontSize:12, letterSpacing:"0.05em", fontWeight:600,
                  border:"1px solid", borderColor:filter===k?"var(--gold)":"var(--line-strong)",
                  background:filter===k?"var(--gold)":"transparent", color:filter===k?"#1B1207":"var(--fg-muted)" }}>{lbl}</button>
              ))}
            </div>
          </div>
        </div>
        <div className="hr-gold" style={{ margin:"18px 0 26px" }} />

        {/* hero live lot */}
        {live && (filter==="all"||filter==="live") && (
          <button onClick={()=>go("auction",{lotId:live.id})} className="fade-up" style={{ width:"100%", textAlign:"start", cursor:"pointer",
            display:"grid", gridTemplateColumns:"1.1fr 1fr", gap:0, marginBottom:30, color:"var(--fg)",
            border:"1px solid var(--gold-line)", borderRadius:"var(--r-3)", overflow:"hidden", background:"var(--bg-1)" }}>
            <div style={{ position:"relative", minHeight:340 }}>
              <Ph art={D.CATS[live.cat].g} ratio={null} style={{ position:"absolute", inset:0, borderRadius:0, border:0 }} />
              <div style={{ position:"absolute", top:18, insetInlineStart:18 }}><Chip state="live" label={t("auc_live")} pulse /></div>
            </div>
            <div style={{ padding:"34px 36px", display:"flex", flexDirection:"column", justifyContent:"center",
              background:"linear-gradient(135deg,var(--burg-deep),var(--bg-1) 80%)" }}>
              <div className="mono up" style={{ fontSize:10, color:"var(--gold)", letterSpacing:"0.14em" }}>{t("gal_lot")} {String(live.no).padStart(2,"0")} · {D.CATS[live.cat][lang]}</div>
              <div className="serif" style={{ fontSize:30, color:"var(--gold-pale)", lineHeight:1.15, margin:"12px 0 6px" }}>{D.tt(live.title, lang)}</div>
              <div className="muted" style={{ fontSize:14 }}>{live.maison} · {live.condition}</div>
              <div style={{ display:"flex", alignItems:"flex-end", gap:28, marginTop:26 }}>
                <div><div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:5 }}>{t("auc_current")}</div>
                  <Money n={live.start-3000} big gold cents={false} /></div>
                <div><div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:5 }}>{t("gal_floor")}</div>
                  <Money n={live.floor} cents={false} /></div>
                <div style={{ display:"flex", alignItems:"center", gap:6, color:"var(--fg-muted)", fontSize:13, marginInlineStart:"auto" }}>
                  <Icon name="eye" size={15}/><span className="mono">{live.watching}</span></div>
              </div>
              <span className="btn btn-gold" style={{ marginTop:24, pointerEvents:"none", alignSelf:"flex-start" }}>{t("lot_enter")} <Icon name="arrow-right" size={17}/></span>
            </div>
          </button>
        )}

        {/* grid */}
        <div style={{ display:"grid", gridTemplateColumns:"repeat(auto-fill,minmax(250px,1fr))", gap:18 }}>
          {list.filter(l=>!(l.live && (filter==="all"))).map(l=>(
            <button key={l.id} onClick={()=>go(l.status==="passive"?"passive":l.live?"auction":"lot",{lotId:l.id})} className="fade-up" style={{ textAlign:"start", cursor:"pointer",
              background:"var(--bg-1)", border:"1px solid var(--line)", borderRadius:"var(--r-2)", overflow:"hidden", color:"var(--fg)", padding:0 }}>
              <div style={{ position:"relative" }}>
                <Ph art={D.CATS[l.cat].g} ratio="4 / 3" />
                {l.live
                  ? <div style={{ position:"absolute", top:10, insetInlineStart:10 }}><Chip state="live" label={t("auc_live")} pulse /></div>
                  : l.status==="passive"
                  ? <div style={{ position:"absolute", top:10, insetInlineStart:10 }}><span className="chip" data-st="warn"><Icon name="clock" size={11}/> {D.ATYPES[l.atype][lang]}</span></div>
                  : <div style={{ position:"absolute", top:10, insetInlineStart:10 }}><Chip state="proposed" label={t("gal_upcoming").toUpperCase()} /></div>}
              </div>
              <div style={{ padding:"14px 16px" }}>
                <div className="mono up" style={{ fontSize:9, color:"var(--gold)" }}>{t("gal_lot")} {String(l.no).padStart(2,"0")} · {D.CATS[l.cat][lang]}</div>
                <div className="serif" style={{ fontSize:16, lineHeight:1.2, margin:"6px 0 12px", color:"var(--fg)",
                  display:"-webkit-box", WebkitLineClamp:2, WebkitBoxOrient:"vertical", overflow:"hidden", minHeight:38 }}>{D.tt(l.title, lang)}</div>
                <div style={{ display:"flex", justifyContent:"space-between", alignItems:"baseline" }}>
                  <span className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{l.live?t("auc_current"):t("gal_floor")}</span>
                  <Money n={l.live?l.start-3000:l.floor} gold cents={false} />
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

/* ============================================================
   DESKTOP LOT — two columns
   ============================================================ */
function DeskLot({ app }) {
  const { t, go, back, lang } = app; const D = window.DATA;
  const lot = D.lot(app.nav.params.lotId) || D.LOTS[0];
  const cat = D.CATS[lot.cat][lang];
  const [certOpen, setCertOpen] = useState(false);
  const [reserveOpen, setReserveOpen] = useState(false);
  const canParticipate = app.tier !== "guest";
  const registered = !!app.parts[lot.id];
  return (
    <div style={{ height:"100%", overflow:"auto", position:"relative" }}>
      <div style={{ maxWidth:1100, margin:"0 auto", padding:"24px 30px 50px" }}>
        <button onClick={back} className="mono" style={{ display:"flex", alignItems:"center", gap:7, background:"none", border:"none",
          color:"var(--fg-muted)", cursor:"pointer", fontSize:12.5, marginBottom:20 }}>
          <Icon name={app.dir==="rtl"?"arrow-right":"arrow-left"} size={16}/> {t("common_back")}</button>
        <div style={{ display:"grid", gridTemplateColumns:"1fr 1fr", gap:40, alignItems:"start" }}>
          {/* left: image */}
          <div style={{ position:"sticky", top:0 }}>
            <div style={{ position:"relative" }}>
              <Ph art={D.CATS[lot.cat].g} ratio="1 / 1" style={{ borderRadius:"var(--r-3)" }} />
              <div style={{ position:"absolute", top:16, insetInlineStart:16 }}>
                <span className="chip" data-st="good"><Icon name="check" size={12}/> {t("lot_authentic")}</span></div>
              <button style={{ position:"absolute", bottom:16, insetInlineEnd:16, ...iconBtnStyle, width:"auto", padding:"9px 15px", gap:8, fontSize:12.5, color:"var(--gold-pale)" }}>
                <Icon name="refresh" size={16}/> {t("lot_view360")}</button>
            </div>
          </div>
          {/* right: details */}
          <div>
            <div className="mono up" style={{ fontSize:10, color:"var(--gold)" }}>{t("gal_lot")} {String(lot.no).padStart(2,"0")} · {cat}</div>
            <h1 className="serif" style={{ fontSize:32, lineHeight:1.15, margin:"10px 0 6px", color:"var(--gold-pale)" }}>{D.tt(lot.title, lang)}</h1>
            <div className="muted" style={{ fontSize:15, marginBottom:22 }}>{lot.maison} · {lot.condition} · {lot.year}</div>

            <div style={{ display:"flex", gap:12, marginBottom:22 }}>
              <Stat label={t("gal_start")} value={<Money n={lot.start} cents={false} />} />
              <Stat label={t("gal_floor")} value={<Money n={lot.floor} cents={false} />} gold />
              <Stat label={t("auc_premium")} value={Math.round(lot.premium*100)+"%"} />
            </div>

            <button onClick={()=>setCertOpen(true)} style={{ width:"100%", cursor:"pointer", border:"1px solid var(--gold-line)",
              borderRadius:"var(--r-2)", background:"var(--bg-1)", padding:"16px 18px", display:"flex", alignItems:"center", gap:16, color:"var(--fg)", textAlign:"start", marginBottom:22 }}>
              <Seal size={52} label="CERT" sub="" id={lot.inspector.split("·")[0].trim()} date={String(lot.year)} />
              <div style={{ flex:1 }}><div style={{ fontWeight:600, fontSize:15 }}>{t("lot_cert")}</div>
                <div className="muted mono" style={{ fontSize:11.5, marginTop:3 }}>{lot.ref}</div></div>
              <Icon name={app.dir==="rtl"?"chevron-left":"chevron-right"} size={18} style={{ color:"var(--gold)" }} />
            </button>

            <div className="kv">
              <div className="k">{t("lot_brand")}</div><div className="v">{lot.maison}</div>
              <div className="k">{t("lot_year")}</div><div className="v">{lot.year}</div>
              <div className="k">{t("lot_box")}</div><div className="v">{lot.box?"✓":"—"}</div>
              <div className="k">{t("lot_inspected")}</div><div className="v" style={{ fontSize:12 }}>{lot.inspector}</div>
              <div className="k">{t("lot_provenance")}</div><div className="v" style={{ fontSize:12, textAlign:"end", maxWidth:260 }}>{D.tt(lot.provenance, lang)}</div>
            </div>
            <div style={{ marginTop:20 }}>
              <Label>{t("lot_about")}</Label>
              <p className="muted" style={{ fontSize:14, lineHeight:1.7, margin:0 }}>{D.tt(lot.desc, lang)}</p>
            </div>

            <div style={{ marginTop:24, paddingTop:22, borderTop:"1px solid var(--line)" }}>
              <div style={{ display:"flex", alignItems:"center", justifyContent:"space-between", marginBottom:12 }}>
                <span className="mono up" style={{ fontSize:10, color:"var(--fg-muted)" }}>{lot.live?t("auc_live"):t("await_live")}</span>
                <span className="mono" style={{ fontSize:13, color:"var(--gold-pale)" }}>{lot.live?"—":fmtClock(lot.opensIn||0)}</span>
              </div>
              {lot.live ? (
                <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>go("auction",{lotId:lot.id})}>{t("lot_enter")} <Icon name="arrow-right" size={18}/></button>
              ) : !canParticipate ? (
                <button className="btn btn-burg" style={{ width:"100%" }} onClick={()=>go("invite")}><Icon name="crown" size={17}/> {t("need_member")}</button>
              ) : registered ? (
                <div style={{ display:"flex", alignItems:"center", justifyContent:"center", gap:10, padding:"14px",
                  border:"1px solid var(--st-good)", borderRadius:"var(--r-1)", background:"var(--st-good-bg)", color:"var(--st-good)", fontWeight:600 }}>
                  <Icon name="check" size={18}/> {t("registered")} · <span className="mono">{D.usd0(Math.round(lot.floor*0.1))}</span></div>
              ) : (
                <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>setReserveOpen(true)}><Icon name="lock" size={17}/> {t("req_participate")} · {D.usd0(Math.round(lot.floor*0.1))}</button>
              )}
            </div>
          </div>
        </div>
      </div>
      {certOpen && <CertModal app={app} lot={lot} onClose={()=>setCertOpen(false)} />}
      {reserveOpen && <ReserveSheet app={app} lot={lot} onClose={()=>setReserveOpen(false)} onReserve={()=>{ app.setPart(lot.id,"requested"); setReserveOpen(false); }} />}
    </div>
  );
}

/* ============================================================
   DESKTOP AUCTION — big stage + price rail (reuses the engine)
   ============================================================ */
function DeskAuction({ app }) {
  const { t, back, go, tw, lang } = app; const D = window.DATA;
  const lot = D.lot(app.nav.params.lotId) || D.LOTS.find(l=>l.live) || D.LOTS[0];
  const fomo = tw.fomo;
  const [locked, setLocked] = useState(false);
  const [lockSheet, setLockSheet] = useState(false);
  const [won, setWon] = useState(false);
  const [flash, setFlash] = useState(false);
  const auc = useDutchAuction(lot, fomo, !won);
  const room = useRoom(lot, fomo, !won);
  const prev = useRef(auc.drops);
  const deposit = lot.floor;
  useEffect(()=>{ if(auc.drops!==prev.current){ prev.current=auc.drops; setFlash(true); const id=setTimeout(()=>setFlash(false),360); return ()=>clearTimeout(id); } }, [auc.drops]);
  const heat = fomo>=6 && auc.progress>0.55;

  if (won) return (
    <CenterCol>
      <WonView app={app} lot={lot} hammer={auc.price} onContinue={()=>go("escrow",{lotId:lot.id,hammer:auc.price})} onBack={()=>{ setWon(false); auc.reset(); }} />
    </CenterCol>
  );

  return (
    <div style={{ height:"100%", overflow:"auto", position:"relative",
      background: heat?"radial-gradient(90% 60% at 50% 20%, rgba(201,162,75,0.10), var(--bg-void) 70%)":"var(--bg-void)" }}>
      <ActivityToast toast={room.toast} app={app} />
      <div style={{ maxWidth:1100, margin:"0 auto", padding:"22px 30px 40px" }}>
        <div style={{ display:"flex", alignItems:"center", justifyContent:"space-between", marginBottom:18 }}>
          <button onClick={back} className="mono" style={{ display:"flex", alignItems:"center", gap:7, background:"none", border:"none", color:"var(--fg-muted)", cursor:"pointer", fontSize:12.5 }}>
            <Icon name={app.dir==="rtl"?"arrow-right":"arrow-left"} size={16}/> {t("common_back")}</button>
          <span className="chip" data-st="live"><span className="dot"/>{t("auc_live")} · {lot.ref}</span>
        </div>

        {fomo>=4 && (
          <div style={{ display:"flex", alignItems:"center", justifyContent:"center", gap:10, padding:"11px 18px", marginBottom:20,
            background:heat?"var(--burg)":"var(--bg-1)", border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", color:"var(--gold-pale)", fontSize:13.5 }}>
            <Icon name="flame" size={16} style={{ color:"var(--gold)" }}/><span style={{ fontWeight:600 }}>{t("auc_dropping")}</span>
            <span className="mono" style={{ opacity:0.8 }}>· −{D.usd0(lot.dropStep)} / {Math.round(auc.every)}{t("auc_secs")}</span>
          </div>
        )}

        <div style={{ display:"grid", gridTemplateColumns:"1.05fr 0.95fr", gap:36, alignItems:"start" }}>
          {/* stage */}
          <div style={{ position:"relative" }}>
            <Ph art={D.CATS[lot.cat].g} ratio="1 / 1" style={{ borderRadius:"var(--r-3)" }} />
            <div style={{ position:"absolute", inset:-12, display:"flex", alignItems:"center", justifyContent:"center", pointerEvents:"none" }}>
              <HeatRing progress={auc.progress} nextIn={auc.nextIn} every={auc.every} size={"calc(100% + 24px)"} atFloor={auc.atFloor} />
            </div>
          </div>
          {/* rail */}
          <div>
            <div className="mono up" style={{ fontSize:10, color:"var(--gold)" }}>{t("gal_lot")} {String(lot.no).padStart(2,"0")} · {D.CATS[lot.cat][lang]}</div>
            <h1 className="serif" style={{ fontSize:28, lineHeight:1.15, margin:"10px 0 4px", color:"var(--gold-pale)" }}>{D.tt(lot.title, lang)}</h1>
            <div className="muted" style={{ fontSize:14, marginBottom:24 }}>{lot.maison} · {lot.condition}</div>

            <div style={{ border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", overflow:"hidden", background:"linear-gradient(135deg,var(--burg-deep),var(--bg-1) 80%)" }}>
              <div style={{ padding:"22px 24px", borderBottom:"1px solid var(--line)" }}>
                <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:8 }}>{t("auc_current")}</div>
                <PriceBig price={auc.price} flash={flash} size={54} />
              </div>
              <div style={{ display:"flex" }}>
                <div style={{ flex:1, padding:"16px 24px", borderInlineEnd:"1px solid var(--line)" }}>
                  <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:5 }}>{t("auc_floor")}</div>
                  <div className="mono" style={{ fontSize:16, color:"var(--fg)" }}>${lot.floor.toLocaleString("en-US")}</div>
                </div>
                <div style={{ flex:1, padding:"16px 24px" }}>
                  <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:5 }}>{t("auc_next")}</div>
                  <div className="mono" style={{ fontSize:16, color:"var(--gold-pale)" }}>{auc.atFloor?"—":`${Math.ceil(auc.nextIn)}${t("auc_secs")}`}</div>
                </div>
              </div>
            </div>

            <div style={{ display:"flex", alignItems:"center", gap:18, margin:"18px 2px", fontSize:13, color:"var(--fg-muted)" }}>
              <span style={{ display:"flex", alignItems:"center", gap:6 }}><Icon name="users" size={15}/><span className="mono">{room.room}</span> {t("auc_participants")}</span>
              <span style={{ display:"flex", alignItems:"center", gap:6 }}><Icon name="eye" size={15}/><span className="mono">{room.watch}</span></span>
            </div>

            {!locked ? (
              <>
                <div className="muted" style={{ fontSize:12.5, marginBottom:10, display:"flex", gap:7, alignItems:"center" }}>
                  <Icon name="lock" size={14}/>{t("lockfull_body")}</div>
                <button className="btn btn-gold" style={{ width:"100%", fontSize:16, padding:"15px" }} onClick={()=>setLockSheet(true)}>
                  <Icon name="lock" size={18}/> {t("lockfull_cta")} · {D.usd0(deposit)}</button>
              </>
            ) : (
              <>
                <div style={{ display:"flex", alignItems:"center", gap:8, marginBottom:10 }}>
                  <Chip state="active" label={t("full_locked")} /><span className="mono" style={{ fontSize:12, color:"var(--fg-muted)" }}>{D.usd0(deposit)}</span></div>
                <button className={"btn btn-gold"+(fomo>=7?" buypulse":"")} style={{ width:"100%", fontSize:18, padding:"17px" }} onClick={()=>setWon(true)}>
                  {t("auc_buy")} ${auc.price.toLocaleString("en-US")}</button>
              </>
            )}
          </div>
        </div>
      </div>
      {lockSheet && <LockSheet app={app} lot={lot} deposit={deposit} onClose={()=>setLockSheet(false)} onLock={()=>{ setLocked(true); setLockSheet(false); }} />}
    </div>
  );
}

/* ============================================================
   DESKTOP VAULT — wide
   ============================================================ */
function DeskVault({ app }) {
  const { t, lang } = app; const D = window.DATA; const M = D.MEMBER;
  const [bb, setBb] = useState(null);
  const [listItem, setListItem] = useState(null);
  const [listed, setListed] = useState(false);
  return (
    <div style={{ height:"100%", overflow:"auto", position:"relative" }}>
      <div style={{ maxWidth:1100, margin:"0 auto", padding:"30px 30px 50px" }}>
        <div style={{ display:"flex", alignItems:"flex-end", justifyContent:"space-between", marginBottom:8 }}>
          <div>
            <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:8 }}>{M.handle}</div>
            <h1 className="serif" style={{ fontSize:38, margin:0, color:"var(--gold-pale)", lineHeight:1 }}>{t("clo_title")}</h1>
          </div>
          <button className="btn btn-ghost" style={{ borderStyle:"dashed", borderColor:"var(--gold-line)", color:"var(--gold-pale)" }}><Icon name="plus" size={18}/> {t("clo_add")}</button>
        </div>
        <div className="hr-gold" style={{ margin:"18px 0 24px" }} />

        <div style={{ display:"flex", gap:14, marginBottom:26 }}>
          <div style={{ flex:1, padding:"18px 22px", border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", background:"linear-gradient(120deg,var(--burg-deep),var(--bg-1))" }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--gold)" }}>{t("clo_credit")}</div>
            <div style={{ marginTop:8 }}><Money n={M.vaultCredit} big gold cents={false} /></div>
          </div>
          <div style={{ flex:1, padding:"18px 22px", border:"1px solid var(--line)", borderRadius:"var(--r-2)", background:"var(--bg-1)" }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{t("clo_value")}</div>
            <div style={{ marginTop:8 }}><Money n={D.VAULT.reduce((a,v)=>a+v.value,0)} big cents={false} /></div>
          </div>
          <div style={{ flex:2, padding:"18px 22px", border:"1px solid var(--line)", borderRadius:"var(--r-2)", background:"var(--bg-1)", display:"flex", alignItems:"center" }}>
            <div className="mono" style={{ fontSize:12, color:"var(--fg-muted)" }}><Icon name="hash" size={13} style={{ display:"inline", verticalAlign:"-2px" }}/> {M.vaultAddress}</div>
          </div>
        </div>

        <div style={{ display:"grid", gridTemplateColumns:"repeat(auto-fill,minmax(240px,1fr))", gap:18 }}>
          {D.VAULT.map(v=>(
            <div key={v.id} style={{ background:"var(--bg-1)", border:"1px solid var(--line)", borderRadius:"var(--r-2)", overflow:"hidden" }}>
              <div style={{ position:"relative" }}>
                <Ph art={D.CATS[v.cat].g} ratio="4 / 3" />
                <div style={{ position:"absolute", top:10, insetInlineStart:10 }}><Chip state={v.state} label={t("st_"+(v.state==="in_closet"?"in_closet":v.state==="in_auction"?"live":v.state==="appraising"?"appraising":"completed"))} /></div>
              </div>
              <div style={{ padding:"13px 15px" }}>
                <div className="muted mono up" style={{ fontSize:9 }}>{v.maison}</div>
                <div className="serif" style={{ fontSize:15, color:"var(--fg)", margin:"5px 0 10px", lineHeight:1.2 }}>{D.tt(v.title, lang)}</div>
                <div style={{ display:"flex", justifyContent:"space-between", alignItems:"center" }}>
                  <Money n={v.value} cents={false} gold />
                  {v.state==="in_closet" && <div style={{ display:"flex", gap:12 }}>
                    <button onClick={()=>setListItem(v)} className="mono" style={{ fontSize:11, color:"var(--gold-pale)", background:"none", border:"none", cursor:"pointer", fontWeight:600 }}>{t("list_title")}</button>
                    <button onClick={()=>setBb(v)} className="mono" style={{ fontSize:11, color:"var(--gold)", background:"none", border:"none", cursor:"pointer", fontWeight:600 }}>{t("clo_buyback")}</button>
                  </div>}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
      {bb && <BuybackSheet app={app} item={bb} onClose={()=>setBb(null)} />}
      {listItem && <ListToAuctionSheet app={app} item={listItem} onClose={()=>setListItem(null)} onSubmit={()=>{ setListItem(null); setListed(true); setTimeout(()=>setListed(false),2200); }} />}
      {listed && <div className="fade-up" style={{ position:"absolute", bottom:20, insetInlineStart:"50%", transform:"translateX(-50%)", zIndex:80, padding:"12px 18px", borderRadius:"var(--r-2)", background:"var(--st-good-bg)", border:"1px solid var(--st-good)", color:"var(--st-good)", fontWeight:600, fontSize:13.5, display:"flex", gap:8, alignItems:"center" }}><Icon name="check" size={16}/> {t("list_submitted")}</div>}
    </div>
  );
}

/* ---------- desktop root router ---------- */
function DesktopBuyer({ app }) {
  const screen = app.nav.screen;
  const DESK = { gallery:DeskGallery, lot:DeskLot, auction:DeskAuction, vault:DeskVault };
  const CENTER = { membership:window.MembershipScreen, account:window.AccountScreen, invite:window.InviteScreen, kyc:window.KycScreen, escrow:window.EscrowScreen, passive:window.PassiveAuctionScreen, bidstore:window.BidStoreScreen };
  let body;
  if (DESK[screen]) { const C = DESK[screen]; body = <C app={app} />; }
  else if (CENTER[screen]) { const C = CENTER[screen]; body = <CenterCol><C app={app} /></CenterCol>; }
  else { body = <DeskGallery app={app} />; }
  return (
    <div dir={app.dir} style={{ height:"100%", display:"flex", flexDirection:"column", background:"var(--bg-void)", color:"var(--fg)",
      fontFamily: app.dir==="rtl"?"var(--sans-rtl)":"var(--sans)" }}>
      <DeskNav app={app} />
      <div style={{ flex:1, minHeight:0, position:"relative" }}>
        <div key={screen + (app.nav.params&&app.nav.params.lotId||"")} style={{ height:"100%" }}>{body}</div>
      </div>
    </div>
  );
}

Object.assign(window, { DesktopBuyer });
