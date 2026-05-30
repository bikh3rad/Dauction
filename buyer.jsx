/* ============================================================
   DAUCTION — buyer mobile flow (chrome + invite/kyc/gallery/lot/vault/membership)
   Renders inside <IOSDevice dark>. Live auction + escrow live in auction.jsx.
   All screens receive `app`: { t, lang, dir, tw, nav, go, back, state, set }.
   ============================================================ */

/* ---------- screen shell: fixed header · scrolling body · fixed footer ---------- */
function ScreenShell({ top, footer, children, bg }) {
  return (
    <div style={{ height:"100%", display:"flex", flexDirection:"column", background: bg || "transparent" }}>
      {top && <div style={{ flexShrink:0 }}>{top}</div>}
      <div className="noscroll" style={{ flex:1, minHeight:0, overflowY:"auto", position:"relative" }}>{children}</div>
      {footer && <div style={{ flexShrink:0 }}>{footer}</div>}
    </div>
  );
}

/* ---------- shared chrome ---------- */
function TopBar({ app, title, kicker, onBack, right }) {
  const { t } = app;
  const deskTop = app && app.platform === "desktop";
  return (
    <div style={{ position:"relative", zIndex:8, background:"rgba(20,12,16,0.86)",
      backdropFilter:"blur(14px)", WebkitBackdropFilter:"blur(14px)",
      borderBottom:"1px solid var(--line)", padding:(deskTop?"16px":"54px")+" 16px 12px" }}>
      <div style={{ display:"flex", alignItems:"center", gap:12 }}>
        {onBack ? (
          <button className="iconbtn" onClick={onBack} aria-label={t("common_back")}
            style={iconBtnStyle}><Icon name={app.dir==="rtl"?"chevron-right":"chevron-left"} size={20} /></button>
        ) : (
          <div style={{ width:38, height:38, display:"flex", alignItems:"center", justifyContent:"center",
            color:"var(--gold)" }}><Icon name="crown" size={20} /></div>
        )}
        <div style={{ flex:1, minWidth:0 }}>
          {kicker && <div className="mono up" style={{ fontSize:9, color:"var(--gold)", marginBottom:2 }}>{kicker}</div>}
          <div className="serif" style={{ fontSize:19, lineHeight:1.1, color:"var(--fg)", whiteSpace:"nowrap", overflow:"hidden", textOverflow:"ellipsis" }}>{title}</div>
        </div>
        {right}
      </div>
    </div>
  );
}
const iconBtnStyle = { width:38, height:38, borderRadius:"var(--r-1)", border:"1px solid var(--line)",
  background:"var(--bg-1)", color:"var(--fg)", display:"flex", alignItems:"center", justifyContent:"center", cursor:"pointer" };

function BottomNav({ app }) {
  const { t, nav, go } = app;
  const items = [
    { k:"gallery", icon:"layers", label:t("nav_gallery") },
    { k:"vault", icon:"package", label:t("nav_closet") },
    { k:"membership", icon:"crown", label:t("nav_membership") },
    { k:"account", icon:"user", label:t("nav_account") },
  ];
  const active = nav.screen;
  return (
    <div style={{ flexShrink:0, zIndex:30,
      background:"rgba(12,8,9,0.92)", backdropFilter:"blur(16px)", WebkitBackdropFilter:"blur(16px)",
      borderTop:"1px solid var(--gold-line)", padding:"10px 8px 26px",
      display:"grid", gridTemplateColumns:"repeat(4,1fr)" }}>
      {items.map(it => {
        const on = active === it.k || (it.k==="gallery" && (active==="lot"||active==="auction"||active==="escrow"));
        return (
          <button key={it.k} onClick={() => go(it.k)} style={{ background:"none", border:"none", cursor:"pointer",
            display:"flex", flexDirection:"column", alignItems:"center", gap:4,
            color: on ? "var(--gold)" : "var(--fg-faint)" }}>
            <Icon name={it.icon} size={21} stroke={on?2.2:1.8} />
            <span style={{ fontSize:9.5, fontWeight:on?600:500, fontFamily:"var(--sans)" }}>{it.label}</span>
          </button>
        );
      })}
    </div>
  );
}

/* ============================================================
   1 · INVITE GATE
   ============================================================ */
function InviteScreen({ app }) {
  const { t, go } = app;
  const VALID = ["LUX-7F2A-9KQ", "MAISON-04"];
  const [code, setCode] = useState("");
  const [err, setErr] = useState(false);
  const [ok, setOk] = useState(false);
  const submit = () => {
    const c = code.trim().toUpperCase();
    if (VALID.includes(c)) { setErr(false); setOk(true); app.setTier("member"); setTimeout(() => go("kyc"), 1300); }
    else { setErr(true); setOk(false); }
  };
  return (
    <ScreenShell>
    <div style={{ minHeight:"100%", display:"flex", flexDirection:"column", padding:"54px 24px 24px",
      background:"radial-gradient(130% 80% at 50% -10%, var(--burg-deep), var(--bg-void) 60%)" }}>
      <button onClick={()=>go("gallery")} aria-label={t("common_close")} style={{ ...iconBtnStyle, position:"absolute", top:54, insetInlineStart:16 }}>
        <Icon name="x" size={18} /></button>
      <div style={{ height:70 }} />
      <div style={{ textAlign:"center" }}>
        <div style={{ color:"var(--gold)", display:"inline-flex", marginBottom:18 }}><Icon name="crown" size={34} /></div>
        <div className="serif" style={{ fontSize:40, letterSpacing:"0.04em", color:"var(--gold-pale)", lineHeight:1 }}>{t("brand")}</div>
        <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginTop:12, letterSpacing:"0.22em" }}>{t("inv_kicker")}</div>
        <p className="muted" style={{ fontSize:12.5, lineHeight:1.6, margin:"12px auto 0", maxWidth:280 }}>{t("invite_elevate")}</p>
      </div>

      <div style={{ flex:1 }} />

      <div className="fade-up" style={{ paddingBottom:14 }}>
        <h1 className="serif" style={{ fontSize:26, margin:"0 0 10px", color:"var(--fg)", textAlign:"center" }}>{t("inv_title")}</h1>
        <p className="muted" style={{ fontSize:14, lineHeight:1.6, textAlign:"center", margin:"0 0 22px" }}>{t("inv_body")}</p>

        {ok ? (
          <div className="fade-up" style={{ display:"flex", flexDirection:"column", alignItems:"center", gap:12, padding:"10px 0 18px" }}>
            <Seal size={92} label="MEMBER" sub="DAUCTION" id="0x11" date="2026" live />
            <div style={{ color:"var(--st-good)", display:"flex", alignItems:"center", gap:8, fontWeight:600 }}>
              <Icon name="check" size={18} /> {t("tier_up")}
            </div>
          </div>
        ) : (
          <>
            <input className={"field field-mono"} value={code} dir="ltr"
              onChange={e => { setCode(e.target.value); setErr(false); }}
              onKeyDown={e => e.key === "Enter" && submit()}
              placeholder={t("inv_ph")} style={{ textAlign:"center", fontSize:18,
                borderColor: err ? "var(--st-bad)" : undefined }} />
            {err && <div style={{ color:"var(--st-bad)", fontSize:12.5, marginTop:8, display:"flex", gap:6, alignItems:"center" }}>
              <Icon name="alert" size={14} /> {t("inv_err")}</div>}
            <button className="btn btn-gold" style={{ width:"100%", marginTop:14 }} onClick={submit} disabled={!code.trim()}>
              {t("inv_cta")} <Icon name="arrow-right" size={18} />
            </button>
            <button className="btn btn-ghost" style={{ width:"100%", marginTop:10, borderColor:"transparent", color:"var(--fg-muted)" }} onClick={()=>{}}>
              {t("inv_req")}
            </button>
            <div className="mono" style={{ fontSize:10.5, color:"var(--fg-faint)", textAlign:"center", marginTop:18, letterSpacing:"0.04em" }}>{t("inv_demo")}</div>
          </>
        )}
      </div>
      <div style={{ height:20 }} />
    </div>
    </ScreenShell>
  );
}

/* ============================================================
   2 · KYC
   ============================================================ */
function KycScreen({ app }) {
  const { t, go } = app;
  const [step, setStep] = useState(0); // 0 phone, 1 otp, 2 doc, 3 pending
  const [phone, setPhone] = useState("");
  const [otp, setOtp] = useState("");
  const [doc, setDoc] = useState(false);
  return (
    <ScreenShell top={<TopBar app={app} kicker={t("kyc_kicker")} title={t("kyc_title")} />}>
      <div style={{ padding:"18px 22px 40px" }}>
        <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:"0 0 22px" }}>{t("kyc_body")}</p>

        <Stepper steps={["1","2","3"]} active={Math.min(step,2)} />

        {step <= 1 && (
          <div className="fade-up" style={{ marginTop:24 }}>
            <Label>{t("kyc_phone")}</Label>
            <div style={{ display:"flex", gap:8 }}>
              <input className="field" dir="ltr" value={phone} onChange={e=>setPhone(e.target.value)} placeholder="+971 5X XXX XXXX" style={{ flex:1 }} />
              <button className="btn btn-ghost" onClick={()=>setStep(1)} disabled={!phone}>{t("kyc_send_otp")}</button>
            </div>
            {step === 1 && (
              <div className="fade-up" style={{ marginTop:18 }}>
                <Label>{t("kyc_otp")}</Label>
                <OtpInput value={otp} onChange={setOtp} />
                <button className="btn btn-gold" style={{ width:"100%", marginTop:16 }} disabled={otp.length<4} onClick={()=>setStep(2)}>
                  {t("common_continue")} <Icon name="arrow-right" size={18}/>
                </button>
              </div>
            )}
          </div>
        )}

        {step === 2 && (
          <div className="fade-up" style={{ marginTop:24 }}>
            <Label>{t("kyc_doc")}</Label>
            <button onClick={()=>setDoc(true)} style={{ width:"100%", cursor:"pointer", textAlign:"start",
              border:"1px dashed var(--gold-line)", background: doc?"var(--bg-2)":"var(--bg-1)", borderRadius:"var(--r-2)",
              padding:"22px 18px", color:"var(--fg)", display:"flex", gap:14, alignItems:"center" }}>
              <div style={{ color: doc?"var(--st-good)":"var(--gold)" }}><Icon name={doc?"check":"upload"} size={26} /></div>
              <div>
                <div style={{ fontWeight:600, fontSize:14.5 }}>{doc ? "emirates-id.scan.pdf" : t("kyc_doc")}</div>
                <div className="muted" style={{ fontSize:12, marginTop:2 }}>{doc ? "1.2 MB · encrypted" : t("kyc_doc_hint")}</div>
              </div>
            </button>
            <button className="btn btn-gold" style={{ width:"100%", marginTop:18 }} disabled={!doc} onClick={()=>setStep(3)}>
              {t("kyc_submit")}
            </button>
          </div>
        )}

        {step === 3 && (
          <div className="fade-up" style={{ marginTop:30, textAlign:"center" }}>
            <Seal size={96} label="PENDING" sub="KYC" id="UAE·23" date="review" />
            <div style={{ marginTop:16 }}><Chip state="pending" label="UNDER_REVIEW" /></div>
            <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:"16px auto 0", maxWidth:280 }}>{t("kyc_pending")}</p>
            <button className="btn btn-gold" style={{ width:"100%", marginTop:24 }} onClick={()=>go("gallery")}>
              {t("nav_gallery")} <Icon name="arrow-right" size={18}/>
            </button>
          </div>
        )}
      </div>
    </ScreenShell>
  );
}
function Label({ children }) { return <div className="mono up" style={{ fontSize:10, color:"var(--fg-muted)", marginBottom:8 }}>{children}</div>; }
function Stepper({ steps, active }) {
  return (
    <div style={{ display:"flex", gap:8, alignItems:"center" }}>
      {steps.map((s,i)=>(
        <React.Fragment key={i}>
          <div className="mono" style={{ width:26, height:26, borderRadius:"50%", display:"flex", alignItems:"center", justifyContent:"center",
            fontSize:12, fontWeight:600, border:"1px solid", borderColor: i<=active?"var(--gold)":"var(--line-strong)",
            background: i<=active?"var(--gold)":"transparent", color: i<=active?"#1B1207":"var(--fg-faint)" }}>{i<active?"✓":s}</div>
          {i<steps.length-1 && <div style={{ flex:1, height:1, background: i<active?"var(--gold)":"var(--line)" }} />}
        </React.Fragment>
      ))}
    </div>
  );
}
function OtpInput({ value, onChange }) {
  const ref = useRef(null);
  return (
    <div style={{ position:"relative" }} onClick={()=>ref.current && ref.current.focus()}>
      <div style={{ display:"flex", gap:8 }}>
        {[0,1,2,3].map(i=>(
          <div key={i} style={{ flex:1, height:54, borderRadius:"var(--r-1)", border:"1px solid",
            borderColor: (value.length===i || (value.length===4 && i===3))?"var(--gold)":"var(--line-strong)", background:"var(--bg-0)",
            display:"flex", alignItems:"center", justifyContent:"center", fontFamily:"var(--mono)", fontSize:22, color:"var(--fg)" }}>
            {value[i] || ""}
          </div>
        ))}
      </div>
      <input ref={ref} value={value} dir="ltr" inputMode="numeric" autoFocus aria-label="enter code"
        onChange={e=>onChange(e.target.value.replace(/\D/g,"").slice(0,4))}
        style={{ position:"absolute", inset:0, width:"100%", height:"100%", opacity:0, border:"none",
          background:"transparent", color:"transparent", caretColor:"transparent", cursor:"pointer",
          fontSize:16, textAlign:"center" }} />
    </div>
  );
}

/* ============================================================
   3 · GALLERY  (grid ↔ magazine via tweak)
   ============================================================ */
function GalleryScreen({ app }) {
  const { t, go, tw, lang } = app;
  const D = window.DATA;
  const [filter, setFilter] = useState("all");
  const live = D.LOTS.filter(l => l.status === "live");
  const up = D.LOTS.filter(l => l.status === "upcoming");
  const shown = filter==="live" ? live : filter==="upcoming" ? up : D.LOTS;
  const mag = tw.galleryLayout === "magazine";

  return (
    <ScreenShell top={<TopBar app={app} kicker={t("gal_kicker")} title={t("gal_title")} right={<LangPill app={app} />} />}>
      {/* guest → invite tier-up banner */}
      {app.tier === "guest" && (
        <button onClick={()=>go("invite")} style={{ width:"calc(100% - 32px)", margin:"14px 16px 0", cursor:"pointer", textAlign:"start",
          border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)", background:"linear-gradient(110deg,var(--burg),var(--bg-1) 80%)",
          padding:"12px 14px", display:"flex", alignItems:"center", gap:12, color:"var(--fg)" }}>
          <span style={{ color:"var(--gold)" }}><Icon name="crown" size={20}/></span>
          <span style={{ flex:1, fontSize:12.5, lineHeight:1.45 }}>{t("guest_banner")}</span>
          <span className="mono up" style={{ fontSize:10, color:"var(--gold-pale)", whiteSpace:"nowrap" }}>{t("enter_invite")}
            <Icon name={app.dir==="rtl"?"arrow-left":"arrow-right"} size={13} style={{ display:"inline", verticalAlign:"-2px", marginInlineStart:4 }}/></span>
        </button>
      )}
      {/* supply scarcity banner */}
      <div style={{ margin:"14px 16px 4px", padding:"12px 14px", border:"1px solid var(--gold-line)",
        borderRadius:"var(--r-2)", background:"linear-gradient(90deg,var(--burg-deep),transparent)",
        display:"flex", alignItems:"center", gap:12 }}>
        <div className="serif" style={{ fontSize:30, color:"var(--gold-pale)", lineHeight:1 }}>32</div>
        <div style={{ flex:1 }}>
          <div style={{ fontSize:12.5, color:"var(--fg)" }}>{t("gal_supply")}</div>
          <div style={{ height:5, background:"var(--bg-0)", borderRadius:3, marginTop:7, overflow:"hidden" }}>
            <div style={{ width:"19%", height:"100%", background:"var(--gold)" }} />
          </div>
        </div>
        <div className="mono" style={{ fontSize:11, color:"var(--gold)" }}>06 / 32</div>
      </div>

      <div style={{ display:"flex", gap:8, padding:"12px 16px 4px", overflowX:"auto" }} className="noscroll">
        {[["all",t("gal_all")],["live",t("gal_live")],["upcoming",t("gal_upcoming")]].map(([k,lbl])=>(
          <button key={k} onClick={()=>setFilter(k)} className="mono" style={{ cursor:"pointer",
            padding:"7px 14px", borderRadius:"var(--r-pill)", fontSize:11.5, letterSpacing:"0.06em", whiteSpace:"nowrap",
            border:"1px solid", borderColor: filter===k?"var(--gold)":"var(--line-strong)",
            background: filter===k?"var(--gold)":"transparent", color: filter===k?"#1B1207":"var(--fg-muted)", fontWeight:600 }}>
            {k==="live" && <span style={{ display:"inline-block", width:6, height:6, borderRadius:"50%", background:"currentColor", marginInlineEnd:6 }} />}
            {lbl}
          </button>
        ))}
      </div>

      <div style={{ padding: mag ? "8px 16px" : "8px 16px",
        display:"grid", gridTemplateColumns: mag ? "1fr" : "1fr 1fr", gap: mag ? 18 : 12 }}>
        {shown.map(l => mag
          ? <LotCardMag key={l.id} lot={l} app={app} />
          : <LotCardGrid key={l.id} lot={l} app={app} />)}
      </div>
    </ScreenShell>
  );
}

function LotMeta({ lot, app }) {
  const { t } = app; const cat = window.DATA.CATS[lot.cat][app.lang];
  return <div className="mono up" style={{ fontSize:9, color:"var(--gold)", letterSpacing:"0.14em" }}>{t("gal_lot")} {String(lot.no).padStart(2,"0")} · {cat}</div>;
}
function lotRoute(lot){ return lot.status==="passive" ? "passive" : lot.live ? "auction" : "lot"; }
function LotCardGrid({ lot, app }) {
  const { t, go } = app;
  return (
    <button onClick={()=>go(lotRoute(lot),{lotId:lot.id})} className="fade-up" style={{ textAlign:"start",
      background:"var(--bg-1)", border:"1px solid var(--line)", borderRadius:"var(--r-2)", overflow:"hidden", cursor:"pointer", color:"var(--fg)", padding:0 }}>
      <div style={{ position:"relative" }}>
        <Ph label={lot.maison} art={window.DATA.CATS[lot.cat].g} ratio="1 / 1" />
        {lot.live && <div style={{ position:"absolute", top:8, insetInlineStart:8 }}><Chip state="live" label={t("auc_live")} pulse /></div>}
        {lot.status==="passive" && <div style={{ position:"absolute", top:8, insetInlineStart:8 }}><span className="chip" data-st="warn"><Icon name="clock" size={11}/> {window.DATA.ATYPES[lot.atype][app.lang].split(" ")[0]}</span></div>}
        <div style={{ position:"absolute", bottom:8, insetInlineEnd:8, display:"flex", alignItems:"center", gap:4,
          background:"rgba(12,8,9,0.7)", padding:"3px 7px", borderRadius:"var(--r-pill)", fontSize:10, color:"var(--fg-muted)" }}>
          <Icon name="eye" size={12}/><span className="mono">{lot.watching}</span></div>
      </div>
      <div style={{ padding:"10px 11px 12px" }}>
        <LotMeta lot={lot} app={app} />
        <div className="serif" style={{ fontSize:14, lineHeight:1.2, margin:"5px 0 8px", color:"var(--fg)",
          display:"-webkit-box", WebkitLineClamp:2, WebkitBoxOrient:"vertical", overflow:"hidden" }}>{window.DATA.tt(lot.title, app.lang)}</div>
        <div style={{ display:"flex", justifyContent:"space-between", alignItems:"baseline" }}>
          <span className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{lot.live?t("auc_current"):lot.status==="passive"?t("list_floor"):t("gal_floor")}</span>
          <Money n={lot.live?lot.start-3000:lot.floor} gold cents={false} />
        </div>
      </div>
    </button>
  );
}
function LotCardMag({ lot, app }) {
  const { t, go } = app;
  return (
    <button onClick={()=>go(lotRoute(lot),{lotId:lot.id})} className="fade-up" style={{ textAlign:"start",
      background:"var(--bg-1)", border:"1px solid var(--line)", borderRadius:"var(--r-3)", overflow:"hidden", cursor:"pointer", color:"var(--fg)", padding:0 }}>
      <div style={{ position:"relative" }}>
        <Ph art={window.DATA.CATS[lot.cat].g} artW="54%" artTop="38%" ratio="4 / 3" />
        {lot.live
          ? <div style={{ position:"absolute", top:12, insetInlineStart:12 }}><Chip state="live" label={t("auc_live")} pulse /></div>
          : lot.status==="passive"
          ? <div style={{ position:"absolute", top:12, insetInlineStart:12 }}><span className="chip" data-st="warn"><Icon name="clock" size={11}/> {window.DATA.ATYPES[lot.atype][app.lang]}</span></div>
          : <div style={{ position:"absolute", top:12, insetInlineStart:12 }}><Chip state="proposed" label={t("gal_upcoming").toUpperCase()} /></div>}
        <div style={{ position:"absolute", insetInlineStart:0, insetInlineEnd:0, bottom:0, height:"62%", background:"linear-gradient(to top, var(--bg-1) 6%, rgba(12,8,9,0.82) 34%, transparent 100%)" }} />
        <div style={{ position:"absolute", bottom:14, insetInlineStart:14, insetInlineEnd:14 }}>
          <LotMeta lot={lot} app={app} />
          <div className="serif" style={{ fontSize:20, lineHeight:1.2, margin:"5px 0 0", color:"var(--gold-pale)",
            display:"-webkit-box", WebkitLineClamp:2, WebkitBoxOrient:"vertical", overflow:"hidden" }}>{window.DATA.tt(lot.title, app.lang)}</div>
          <div className="mono up" style={{ fontSize:9, color:"var(--fg-muted)", marginTop:5, letterSpacing:"0.1em" }}>{lot.maison}</div>
        </div>
      </div>
      <div style={{ padding:"14px 16px", display:"flex", justifyContent:"space-between", alignItems:"center" }}>
        <div>
          <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{lot.live?t("auc_current"):lot.status==="passive"?t("list_floor"):t("gal_floor")}</div>
          <div style={{ marginTop:3 }}><Money n={lot.live?lot.start-3000:lot.floor} big gold cents={false} /></div>
        </div>
        <div style={{ display:"flex", alignItems:"center", gap:5, color:"var(--fg-muted)", fontSize:12 }}>
          <Icon name="eye" size={14}/><span className="mono">{lot.watching}</span>
          <span style={{ color:"var(--gold)", marginInlineStart:6, display:"inline-flex" }}><Icon name="arrow-right" size={16}/></span>
        </div>
      </div>
    </button>
  );
}

function LangPill({ app }) {
  const { lang, setLang } = app;
  const order = ["en","fa","ar","tr"];
  const next = order[(order.indexOf(lang)+1)%order.length];
  return (
    <button onClick={()=>setLang(next)} className="mono" aria-label="language" style={{ width:38, height:38, borderRadius:"var(--r-1)",
      border:"1px solid var(--line)", background:"var(--bg-1)", color:"var(--gold-pale)", cursor:"pointer", fontSize:12, fontWeight:600 }}>
      {window.I18N.label[lang]}
    </button>
  );
}

/* ============================================================
   4 · LOT DETAIL  (+ Certificate of Authenticity, 360)
   ============================================================ */
function LotScreen({ app }) {
  const { t, go, back, lang } = app;
  const D = window.DATA;
  const lot = D.lot(app.nav.params.lotId) || D.LOTS[0];
  const cat = D.CATS[lot.cat][lang];
  const [spin, setSpin] = useState(0);
  const [certOpen, setCertOpen] = useState(false);
  const [reserveOpen, setReserveOpen] = useState(false);
  const canParticipate = app.tier !== "guest";
  const registered = !!app.parts[lot.id];
  return (
    <>
    <ScreenShell
      top={<TopBar app={app} kicker={`${t("gal_lot")} ${String(lot.no).padStart(2,"0")} · ${cat}`} title={lot.maison} onBack={back}
        right={<button className="iconbtn" style={iconBtnStyle} aria-label="watch"><Icon name="eye" size={18}/></button>} />}
      footer={
        <div style={{ padding:"12px 16px 22px", background:"var(--bg-1)", borderTop:"1px solid var(--line)" }}>
          <div style={{ display:"flex", alignItems:"center", justifyContent:"space-between", marginBottom:10, padding:"0 4px" }}>
            <span className="mono up" style={{ fontSize:10, color:"var(--fg-muted)" }}>{lot.live?t("auc_live"):t("await_live")}</span>
            <span className="mono" style={{ fontSize:12, color:"var(--gold-pale)" }}>{lot.live?"—":fmtClock(lot.opensIn||0)}</span>
          </div>
          {lot.live ? (
            <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>go("auction",{lotId:lot.id})}>
              {t("lot_enter")} <Icon name="arrow-right" size={18}/>
            </button>
          ) : !canParticipate ? (
            <button className="btn btn-burg" style={{ width:"100%" }} onClick={()=>go("invite")}>
              <Icon name="crown" size={17}/> {t("need_member")}
            </button>
          ) : registered ? (
            <div style={{ display:"flex", alignItems:"center", justifyContent:"center", gap:10, padding:"13px",
              border:"1px solid var(--st-good)", borderRadius:"var(--r-1)", background:"var(--st-good-bg)", color:"var(--st-good)", fontWeight:600 }}>
              <Icon name="check" size={18}/> {t("registered")} · <span className="mono">{window.DATA.usd0(Math.round(lot.floor*0.1))} {t("esc_locked").toLowerCase()}</span>
            </div>
          ) : (
            <button className="btn btn-gold" style={{ width:"100%" }} onClick={()=>setReserveOpen(true)}>
              <Icon name="lock" size={17}/> {t("req_participate")} · {window.DATA.usd0(Math.round(lot.floor*0.1))}
            </button>
          )}
        </div>
      }>
      {/* 360 viewer (placeholder) */}
      <div style={{ position:"relative", margin:"4px 0 0" }}>
        <Ph label={`${lot.maison} · ${t("lot_view360")}`} art={D.CATS[lot.cat].g} ratio="1 / 1"
          style={{ borderRadius:0, borderInline:0 }} />
        <button onClick={()=>setSpin(s=>s+1)} style={{ position:"absolute", bottom:14, insetInlineEnd:14,
          ...iconBtnStyle, width:"auto", padding:"8px 14px", gap:8, fontSize:12.5, color:"var(--gold-pale)" }}>
          <Icon name="refresh" size={16} style={{ transform:`rotate(${spin*120}deg)`, transition:"transform .5s var(--ease-doc)" }} /> {t("lot_view360")}
        </button>
        <div style={{ position:"absolute", top:14, insetInlineStart:14 }}>
          <span className="chip" data-st="good"><Icon name="check" size={12}/> {t("lot_authentic")}</span>
        </div>
      </div>

      <div style={{ padding:"18px 20px 0" }}>
        <h1 className="serif" style={{ fontSize:24, lineHeight:1.18, margin:"0 0 4px", color:"var(--gold-pale)" }}>{D.tt(lot.title, lang)}</h1>
        <div className="muted" style={{ fontSize:13, marginBottom:18 }}>{lot.condition} · {lot.year}</div>

        {/* price band */}
        <div style={{ display:"flex", gap:10, marginBottom:20 }}>
          <Stat label={t("gal_start")} value={<Money n={lot.start} cents={false} />} />
          <Stat label={t("gal_floor")} value={<Money n={lot.floor} cents={false} />} gold />
        </div>

        {/* certificate trigger — document chrome teaser */}
        <button onClick={()=>setCertOpen(true)} style={{ width:"100%", cursor:"pointer", border:"1px solid var(--gold-line)",
          borderRadius:"var(--r-2)", background:"var(--bg-1)", padding:"14px 16px", display:"flex", alignItems:"center", gap:14, color:"var(--fg)", textAlign:"start" }}>
          <Seal size={48} label="CERT" sub="" id={lot.inspector.split("·")[0].trim()} date={String(lot.year)} />
          <div style={{ flex:1 }}>
            <div style={{ fontWeight:600, fontSize:14 }}>{t("lot_cert")}</div>
            <div className="muted mono" style={{ fontSize:11, marginTop:3 }}>{lot.ref}</div>
          </div>
          <Icon name={app.dir==="rtl"?"chevron-left":"chevron-right"} size={18} style={{ color:"var(--gold)" }} />
        </button>

        {/* KV facts */}
        <div className="kv" style={{ marginTop:20 }}>
          <div className="k">{t("lot_brand")}</div><div className="v">{lot.maison}</div>
          <div className="k">{t("lot_year")}</div><div className="v">{lot.year}</div>
          <div className="k">{t("lot_box")}</div><div className="v">{lot.box ? "✓" : "—"}</div>
          <div className="k">{t("lot_inspected")}</div><div className="v" style={{ fontSize:11 }}>{lot.inspector}</div>
          <div className="k">{t("lot_ref")}</div><div className="v" style={{ fontSize:11 }}>{lot.ref}</div>
          <div className="k">{t("auc_premium")}</div><div className="v">{Math.round(lot.premium*100)}%</div>
        </div>

        <div style={{ marginTop:18, marginBottom:24 }}>
          <Label>{t("lot_about")}</Label>
          <p className="muted" style={{ fontSize:14, lineHeight:1.7, margin:"0 0 16px" }}>{D.tt(lot.desc, lang)}</p>
          <Label>{t("lot_provenance")}</Label>
          <p className="muted" style={{ fontSize:13.5, lineHeight:1.6, margin:0 }}>{D.tt(lot.provenance, lang)}</p>
        </div>
      </div>
    </ScreenShell>
    {certOpen && <CertModal app={app} lot={lot} onClose={()=>setCertOpen(false)} />}
    {reserveOpen && <ReserveSheet app={app} lot={lot} onClose={()=>setReserveOpen(false)} onReserve={()=>{ app.setPart(lot.id,"requested"); setReserveOpen(false); }} />}
    </>
  );
}

/* Reservation sheet — lock 10% deposit BEFORE the auction opens */
function ReserveSheet({ app, lot, onClose, onReserve }) {
  const { t } = app; const D = window.DATA; const M = D.MEMBER;
  const deposit = Math.round(lot.floor * 0.10);
  return (
    <div onClick={onClose} style={{ position:"absolute", inset:0, zIndex:70, background:"rgba(8,5,6,0.65)",
      backdropFilter:"blur(6px)", WebkitBackdropFilter:"blur(6px)", display:"flex", alignItems:"flex-end" }}>
      <div onClick={e=>e.stopPropagation()} className="fade-up" style={{ width:"100%", background:"var(--bg-1)",
        borderTop:"1px solid var(--gold-line)", borderRadius:"16px 16px 0 0", padding:"22px 20px 34px" }}>
        <div style={{ width:40, height:4, background:"var(--line-strong)", borderRadius:2, margin:"0 auto 18px" }} />
        <div style={{ display:"flex", alignItems:"center", gap:10, marginBottom:6 }}>
          <span style={{ color:"var(--gold)" }}><Icon name="lock" size={20}/></span>
          <div className="serif" style={{ fontSize:20, color:"var(--gold-pale)" }}>{t("req_title")}</div>
        </div>
        <p className="muted" style={{ fontSize:13, lineHeight:1.6, margin:"0 0 18px" }}>{t("req_body")}</p>
        <div className="kv" style={{ marginBottom:20 }}>
          <div className="k">{lot.maison}</div><div className="v" style={{ fontSize:11 }}>{D.tt(lot.title, app.lang)}</div>
          <div className="k">{t("gal_floor")}</div><div className="v">{D.usd(lot.floor)}</div>
          <div className="k">{t("req_deposit")}</div><div className="v" style={{ color:"var(--gold-pale)" }}>{D.usd(deposit)}</div>
          <div className="k">{t("esc_balance")}</div><div className="v">{D.usd(M.walletUSDC)}</div>
        </div>
        <button className="btn btn-gold" style={{ width:"100%" }} onClick={onReserve}>
          <Icon name="lock" size={17}/> {t("req_cta")} · {D.usd0(deposit)}
        </button>
      </div>
    </div>
  );
}
function Stat({ label, value, gold }) {
  return (
    <div style={{ flex:1, border:"1px solid var(--line)", borderRadius:"var(--r-2)", padding:"12px 14px", background:"var(--bg-1)" }}>
      <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:6 }}>{label}</div>
      <div style={{ fontSize:15, color: gold?"var(--gold-pale)":"var(--fg)" }}>{value}</div>
    </div>
  );
}

/* Certificate of Authenticity — full document chrome on cream paper */
function CertModal({ app, lot, onClose }) {
  const { t } = app; const D = window.DATA;
  return (
    <div onClick={onClose} style={{ position:"absolute", inset:0, zIndex:60, background:"rgba(8,5,6,0.7)",
      backdropFilter:"blur(6px)", WebkitBackdropFilter:"blur(6px)", display:"flex", alignItems:"center", justifyContent:"center", padding:20 }}>
      <div onClick={e=>e.stopPropagation()} className="doc grain fade-up" dir="ltr" style={{ width:"100%", maxWidth:340, borderRadius:2 }}>
        <span className="tick-bl" /><span className="tick-br" />
        <div style={{ textAlign:"center" }}>
          <div style={{ display:"inline-flex", color:"var(--burg)" }}><Seal size={64} label="CERTIFIED" sub="DAUCTION" id={lot.inspector.split("·")[0].trim()} date={String(lot.year)} /></div>
          <div className="serif" style={{ fontSize:18, color:"var(--ink)", marginTop:8, fontWeight:700 }}>Certificate of Authenticity</div>
          <div className="mono" style={{ fontSize:10, color:"var(--ink-muted)", letterSpacing:"0.1em", marginTop:4 }}>{lot.ref}</div>
        </div>
        <div style={{ height:1, background:"var(--ink-line)", margin:"16px 0" }} />
        <table style={{ width:"100%", borderCollapse:"collapse", fontSize:12.5, color:"var(--ink)" }}>
          <tbody>
            {[["Object", D.tt(lot.title,"en")],["Maison", lot.maison],["Year", lot.year],["Condition", lot.condition],
              ["Box & papers", lot.box?"Present":"—"],["Inspector", lot.inspector],["Provenance", D.tt(lot.provenance,"en")]].map(([k,v],i)=>(
              <tr key={i}><td style={{ color:"var(--ink-muted)", padding:"7px 0", verticalAlign:"top", width:"40%" }}>{k}</td>
                <td style={{ padding:"7px 0", fontFamily:"var(--mono)", textAlign:"right" }}>{v}</td></tr>
            ))}
          </tbody>
        </table>
        <div style={{ height:1, background:"var(--ink-line)", margin:"14px 0" }} />
        <div style={{ display:"flex", justifyContent:"space-between", alignItems:"flex-end" }}>
          <div>
            <div style={{ fontFamily:"var(--serif)", fontStyle:"italic", fontSize:18, color:"var(--burg)", borderBottom:"1px solid var(--ink-line)", paddingBottom:2 }}>D. Auction</div>
            <div className="mono" style={{ fontSize:8.5, color:"var(--ink-muted)", marginTop:4 }}>HOUSE REGISTRAR</div>
          </div>
          <div className="mono" style={{ fontSize:9, color:"var(--ink-muted)", textAlign:"right" }}>SIGNED ON-CHAIN<br/>0x7a4e…b2f1</div>
        </div>
        <button className="btn" onClick={onClose} style={{ width:"100%", marginTop:18, background:"var(--burg)", color:"var(--paper)" }}>{t("common_close")}</button>
      </div>
    </div>
  );
}

/* ============================================================
   7 · VAULT (My Closet)   — grid ↔ magazine via tweak
   ============================================================ */
function VaultScreen({ app }) {
  const { t, lang, tw, go } = app;
  const D = window.DATA;
  const M = D.MEMBER;
  const [bb, setBb] = useState(null); // buyback target
  const [listItem, setListItem] = useState(null); // list-to-auction target
  const [listed, setListed] = useState(false);
  const counts = { in_closet:0, in_auction:0, sold:0 };
  D.VAULT.forEach(v => { if (v.state==="sold") counts.sold++; else if (v.state==="in_auction") counts.in_auction++; else counts.in_closet++; });
  const mag = tw.galleryLayout === "magazine";
  return (
    <>
    <ScreenShell top={<TopBar app={app} kicker={M.handle} title={t("clo_title")} right={<LangPill app={app} />} />}>
      <div style={{ padding:"16px 16px 24px" }}>
        {/* credit + value band */}
        <div style={{ display:"flex", gap:10, marginBottom:16 }}>
          <div style={{ flex:1, padding:"14px 16px", border:"1px solid var(--gold-line)", borderRadius:"var(--r-2)",
            background:"linear-gradient(120deg,var(--burg-deep),var(--bg-1))" }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--gold)" }}>{t("clo_credit")}</div>
            <div style={{ marginTop:6 }}><Money n={M.vaultCredit} big gold cents={false} /></div>
          </div>
          <div style={{ flex:1, padding:"14px 16px", border:"1px solid var(--line)", borderRadius:"var(--r-2)", background:"var(--bg-1)" }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{t("clo_value")}</div>
            <div style={{ marginTop:6 }}><Money n={D.VAULT.reduce((a,v)=>a+v.value,0)} big cents={false} /></div>
          </div>
        </div>
        <div className="mono" style={{ fontSize:10.5, color:"var(--fg-faint)", marginBottom:14 }}>
          <Icon name="hash" size={12} style={{ display:"inline", verticalAlign:"-2px" }} /> {M.vaultAddress}
        </div>

        <div style={{ display:"flex", gap:14, marginBottom:14, fontSize:11.5 }}>
          {[["clo_incloset",counts.in_closet],["clo_inauction",counts.in_auction],["clo_sold",counts.sold]].map(([k,n])=>(
            <span key={k} className="muted">{t(k)} <b className="mono" style={{ color:"var(--fg)" }}>{n}</b></span>
          ))}
        </div>

        <div style={{ display:"grid", gridTemplateColumns: mag?"1fr":"1fr 1fr", gap:12 }}>
          {D.VAULT.map(v => (
            <div key={v.id} className="fade-up" style={{ background:"var(--bg-1)", border:"1px solid var(--line)", borderRadius:"var(--r-2)", overflow:"hidden" }}>
              <div style={{ position:"relative" }}>
                <Ph art={D.CATS[v.cat].g} ratio={mag?"5 / 3":"1 / 1"} label={v.maison} />
                <div style={{ position:"absolute", top:8, insetInlineStart:8 }}><Chip state={v.state} label={t("st_"+(v.state==="in_closet"?"in_closet":v.state==="in_auction"?"live":v.state==="appraising"?"appraising":"completed"))} /></div>
              </div>
              <div style={{ padding:"10px 12px" }}>
                <div className="serif" style={{ fontSize:13.5, color:"var(--fg)", marginBottom:4, lineHeight:1.2 }}>{D.tt(v.title, lang)}</div>
                <div style={{ display:"flex", justifyContent:"space-between", alignItems:"center", gap:8 }}>
                  <Money n={v.value} cents={false} gold />
                  {v.state==="in_closet" && <div style={{ display:"flex", gap:10 }}>
                    <button onClick={()=>setListItem(v)} className="mono" style={{ fontSize:10, color:"var(--gold-pale)", background:"none", border:"none", cursor:"pointer", fontWeight:600 }}>{t("list_title")}</button>
                    <button onClick={()=>setBb(v)} className="mono" style={{ fontSize:10, color:"var(--gold)", background:"none", border:"none", cursor:"pointer", fontWeight:600 }}>{t("clo_buyback")}</button>
                  </div>}
                </div>
              </div>
            </div>
          ))}
        </div>

        <button className="btn btn-ghost" style={{ width:"100%", margin:"16px 0", borderStyle:"dashed", borderColor:"var(--gold-line)", color:"var(--gold-pale)" }}>
          <Icon name="plus" size={18}/> {t("clo_add")}
        </button>
      </div>
    </ScreenShell>
    {bb && <BuybackSheet app={app} item={bb} onClose={()=>setBb(null)} />}
    {listItem && <ListToAuctionSheet app={app} item={listItem} onClose={()=>setListItem(null)} onSubmit={()=>{ setListItem(null); setListed(true); setTimeout(()=>setListed(false), 2200); }} />}
    {listed && <div className="fade-up" style={{ position:"absolute", bottom:96, insetInlineStart:16, insetInlineEnd:16, zIndex:80, padding:"12px 16px", borderRadius:"var(--r-2)", background:"var(--st-good-bg)", border:"1px solid var(--st-good)", color:"var(--st-good)", fontWeight:600, fontSize:13, display:"flex", gap:8, alignItems:"center" }}><Icon name="check" size={16}/> {t("list_submitted")}</div>}
    </>
  );
}

function BuybackSheet({ app, item, onClose }) {
  const { t } = app;
  return (
    <div onClick={onClose} style={{ position:"absolute", inset:0, zIndex:60, background:"rgba(8,5,6,0.6)", display:"flex", alignItems:"flex-end" }}>
      <div onClick={e=>e.stopPropagation()} className="fade-up" style={{ width:"100%", background:"var(--bg-1)",
        borderTop:"1px solid var(--gold-line)", borderRadius:"16px 16px 0 0", padding:"22px 20px 34px" }}>
        <div style={{ width:40, height:4, background:"var(--line-strong)", borderRadius:2, margin:"0 auto 18px" }} />
        <div className="serif" style={{ fontSize:20, color:"var(--gold-pale)", marginBottom:4 }}>{t("clo_buyback")}</div>
        <p className="muted" style={{ fontSize:13, lineHeight:1.6, margin:"0 0 18px" }}>{t("clo_buyback_body")}</p>
        <div style={{ display:"flex", gap:12 }}>
          <button className="btn btn-ghost" style={{ flex:1, flexDirection:"column", padding:"16px", height:"auto", gap:4 }}>
            <span style={{ fontSize:11, color:"var(--fg-muted)" }} className="mono up">{t("clo_cash")}</span>
            <Money n={Math.round(item.value*0.5)} big cents={false} />
            <span className="mono" style={{ fontSize:10, color:"var(--fg-faint)" }}>50%</span>
          </button>
          <button className="btn btn-gold" style={{ flex:1, flexDirection:"column", padding:"16px", height:"auto", gap:4 }}>
            <span style={{ fontSize:11 }} className="mono up">{t("clo_creditopt")}</span>
            <span className="mono" style={{ fontSize:18, fontWeight:600 }}>{window.DATA.usd0(Math.round(item.value*0.85))}</span>
            <span className="mono" style={{ fontSize:10 }}>85% · Vault Credit</span>
          </button>
        </div>
      </div>
    </div>
  );
}

/* ============================================================
   8 · MEMBERSHIP
   ============================================================ */
function MembershipScreen({ app }) {
  const { t, lang } = app;
  const D = window.DATA;
  const cur = D.MEMBER.tier;
  const desc = { guest:t("mem_guest_a"), standard:t("mem_std_a"), vip:t("mem_vip_a") };
  const names = { guest:t("mem_guest"), standard:t("mem_standard"), vip:t("mem_vip") };
  return (
    <ScreenShell top={<TopBar app={app} kicker={t("mem_sub")} title={t("mem_title")} right={<LangPill app={app} />} />}>
      <div style={{ padding:"18px 16px 24px", display:"flex", flexDirection:"column", gap:14 }}>
        {D.TIERS.map(tier => {
          const on = tier.key === cur;
          const vip = tier.key === "vip";
          return (
            <div key={tier.key} className="fade-up" style={{ position:"relative", borderRadius:"var(--r-3)", overflow:"hidden",
              border:"1px solid", borderColor: on?"var(--gold)":vip?"var(--gold-line)":"var(--line)",
              background: vip?"linear-gradient(135deg,var(--burg),var(--bg-1) 70%)":"var(--bg-1)", padding:"18px 18px" }}>
              {on && <div style={{ position:"absolute", top:0, insetInlineEnd:0 }}><span className="chip" data-st="live" style={{ borderRadius:"0 0 0 var(--r-2)" }}>{t("mem_current")}</span></div>}
              <div style={{ display:"flex", alignItems:"center", gap:10, marginBottom:12 }}>
                {vip && <span style={{ color:"var(--gold)" }}><Icon name="crown" size={22}/></span>}
                <div className="serif" style={{ fontSize:23, color: vip?"var(--gold-pale)":"var(--fg)" }}>{names[tier.key]}</div>
              </div>
              <div style={{ display:"flex", gap:20, marginBottom:12 }}>
                <div><div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)" }}>{t("mem_fee")}</div>
                  <div className="mono" style={{ fontSize:22, color:"var(--gold-pale)", marginTop:3 }}>{tier.feeBuyer}</div></div>
              </div>
              <p className="muted" style={{ fontSize:13, lineHeight:1.55, margin:0 }}>{desc[tier.key]}</p>
              {!on && !["guest"].includes(tier.key) && (
                <button className={"btn "+(vip?"btn-gold":"btn-ghost")} style={{ width:"100%", marginTop:14 }}>{t("mem_upgrade")}</button>
              )}
            </div>
          );
        })}
      </div>
    </ScreenShell>
  );
}

Object.assign(window, { ScreenShell, TopBar, BottomNav, InviteScreen, KycScreen, GalleryScreen, LotScreen, VaultScreen, MembershipScreen, LangPill, Label, Stat, iconBtnStyle, CertModal, ReserveSheet, BuybackSheet, LotCardGrid, LotCardMag, LotMeta });
