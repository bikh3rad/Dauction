/* ============================================================
   DAUCTION — ADMIN (desktop) — invitations, authentication, escrow & disputes
   Manifest dark shell · 56px status bar · 240px sidebar (TRADE layout rules)
   ============================================================ */
function AdminApp({ app }) {
  const { t, lang } = app;
  const D = window.DATA;
  const [sec, setSec] = useState("overview");
  const nav = [
    { k:"overview", icon:"grid", label:t("adm_overview") },
    { k:"invites", icon:"hash", label:t("adm_invites") },
    { k:"auth", icon:"stamp", label:t("adm_closet") },
    { k:"escrow", icon:"scale", label:t("adm_escrow") },
  ];
  return (
    <div style={{ height:"100%", display:"flex", flexDirection:"column", background:"var(--bg-void)", color:"var(--fg)" }}>
      {/* top status bar */}
      <div style={{ height:56, flexShrink:0, borderBottom:"1px solid var(--gold-line)", display:"flex", alignItems:"center",
        padding:"0 20px", gap:14, background:"rgba(20,12,16,0.7)", backdropFilter:"blur(12px)" }}>
        <span style={{ color:"var(--gold)" }}><Icon name="crown" size={20}/></span>
        <span className="serif" style={{ fontSize:18, color:"var(--gold-pale)", letterSpacing:"0.04em" }}>{t("brand")}</span>
        <span className="mono up" style={{ fontSize:9.5, color:"var(--fg-faint)", border:"1px solid var(--line)", padding:"3px 8px", borderRadius:"var(--r-1)" }}>HOUSE OPS</span>
        <div style={{ flex:1 }} />
        <span className="chip" data-st="live"><span className="dot"/>WEEK 23 · LIVE</span>
        <span className="mono" style={{ fontSize:12, color:"var(--fg-muted)" }}>registrar · 0x11</span>
      </div>

      <div style={{ flex:1, display:"flex", minHeight:0 }}>
        {/* sidebar */}
        <div style={{ width:240, flexShrink:0, borderInlineEnd:"1px solid var(--line)", padding:"16px 12px", background:"var(--bg-0)" }}>
          {nav.map(n=>(
            <button key={n.k} onClick={()=>setSec(n.k)} style={{ width:"100%", textAlign:"start", display:"flex", alignItems:"center", gap:11,
              padding:"11px 12px", borderRadius:"var(--r-2)", marginBottom:4, cursor:"pointer", border:"none",
              background: sec===n.k?"var(--bg-2)":"transparent", color: sec===n.k?"var(--gold-pale)":"var(--fg-muted)",
              borderInlineStart:"2px solid", borderInlineStartColor: sec===n.k?"var(--gold)":"transparent", fontSize:14, fontFamily:"var(--sans)", fontWeight:sec===n.k?600:500 }}>
              {n.k==="auth" ? <Seal size={18} label="" sub="" id="" date="" /> : n.k==="escrow" ? <Icon name="scale" size={18}/> : <Icon name={n.icon} size={18}/>}
              {n.label}
            </button>
          ))}
          <div style={{ marginTop:24, padding:"0 12px" }}>
            <div className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginBottom:10 }}>{t("switch_view")}</div>
            <button className="btn btn-ghost" style={{ width:"100%", fontSize:13, padding:"9px" }} onClick={()=>app.setView("buyer")}>
              <Icon name="arrow-left" size={15}/> {t("view_buyer")}
            </button>
          </div>
        </div>

        {/* content */}
        <div style={{ flex:1, overflow:"auto", padding:"28px 32px" }}>
          {sec==="overview" && <AdmOverview app={app} setSec={setSec} />}
          {sec==="invites" && <AdmInvites app={app} />}
          {sec==="auth" && <AdmAuth app={app} />}
          {sec==="escrow" && <AdmEscrow app={app} />}
        </div>
      </div>
    </div>
  );
}

function SecHead({ kicker, title }) {
  return (
    <div style={{ marginBottom:22 }}>
      {kicker && <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:6 }}>{kicker}</div>}
      <h1 className="serif" style={{ fontSize:30, margin:0, color:"var(--gold-pale)" }}>{title}</h1>
    </div>
  );
}
function Tile({ label, value, sub, accent }) {
  return (
    <div style={{ flex:1, minWidth:170, padding:"18px 20px", border:"1px solid var(--line)", borderRadius:"var(--r-2)",
      background: accent?"linear-gradient(135deg,var(--burg-deep),var(--bg-1))":"var(--bg-1)" }}>
      <div className="mono up" style={{ fontSize:9.5, color:"var(--fg-faint)", marginBottom:10 }}>{label}</div>
      <div className="serif" style={{ fontSize:30, color:"var(--gold-pale)", lineHeight:1 }}>{value}</div>
      {sub && <div className="mono" style={{ fontSize:11, color:"var(--fg-muted)", marginTop:8 }}>{sub}</div>}
    </div>
  );
}
function DTable({ cols, children }) {
  return (
    <div style={{ border:"1px solid var(--line)", borderRadius:"var(--r-2)", overflow:"hidden", background:"var(--bg-1)" }}>
      <table style={{ width:"100%", borderCollapse:"collapse", fontSize:13.5 }}>
        <thead><tr style={{ background:"var(--bg-0)" }}>
          {cols.map((c,i)=><th key={i} style={{ textAlign: c.end?"end":"start", padding:"12px 16px", fontFamily:"var(--mono)",
            fontSize:9.5, letterSpacing:"0.12em", textTransform:"uppercase", color:"var(--fg-faint)", fontWeight:600,
            borderBottom:"1px solid var(--line)" }}>{c.label}</th>)}
        </tr></thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}
const tdS = { padding:"13px 16px", borderBottom:"1px solid var(--line)", verticalAlign:"middle" };
function GBtn({ children, onClick, kind="ghost", small }) {
  const bg = kind==="gold"?"var(--gold)":kind==="bad"?"transparent":"transparent";
  const col = kind==="gold"?"#1B1207":kind==="bad"?"var(--st-bad)":"var(--fg)";
  const bd = kind==="gold"?"var(--gold-bright)":kind==="bad"?"var(--st-bad)":"var(--line-strong)";
  return <button onClick={onClick} className="mono" style={{ fontSize:11, fontWeight:600, padding: small?"6px 11px":"8px 14px",
    borderRadius:"var(--r-1)", border:"1px solid "+bd, background:bg, color:col, cursor:"pointer", letterSpacing:"0.04em" }}>{children}</button>;
}

function AdmOverview({ app, setSec }) {
  const { t } = app; const D = window.DATA;
  const locked = D.ESCROW.reduce((a,e)=> a + (["funded","in_transit"].includes(e.state)? e.amount : 0), 0);
  return (
    <div className="fade-up">
      <SecHead kicker="WEEK 23 · 2026" title={t("adm_overview")} />
      <div style={{ display:"flex", gap:14, flexWrap:"wrap", marginBottom:26 }}>
        <Tile label={t("adm_active_invites")} value="3" sub="1 flagged" accent />
        <Tile label={t("adm_pending_kyc")} value="2" sub="UAE 2023 KYC" />
        <Tile label={t("adm_lots_week")} value="06 / 32" sub="supply cap 32" />
        <Tile label={t("adm_escrow_locked")} value={D.usd0(locked)} sub="across 2 trades" accent />
      </div>
      <div style={{ display:"grid", gridTemplateColumns:"1.3fr 1fr", gap:20 }}>
        <div>
          <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:10 }}>{t("adm_chain")}</div>
          <DTable cols={[{label:"CODE"},{label:t("adm_issued_by")},{label:t("adm_status"),end:true}]}>
            {D.INVITES.slice(0,4).map(iv=>(
              <tr key={iv.code}><td style={tdS}><span className="mono" style={{ color:"var(--gold-pale)" }}>{iv.code}</span></td>
                <td style={{...tdS, color:"var(--fg-muted)"}} className="mono">{iv.issuedBy}</td>
                <td style={{...tdS, textAlign:"end"}}><Chip state={iv.status} label={iv.status.toUpperCase()} /></td></tr>
            ))}
          </DTable>
        </div>
        <div>
          <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:10 }}>{t("adm_escrow")}</div>
          <DTable cols={[{label:t("adm_object")},{label:t("adm_status"),end:true}]}>
            {D.ESCROW.map(e=>(
              <tr key={e.id}><td style={{...tdS}}>{e.lot}<div className="mono" style={{ fontSize:10.5, color:"var(--fg-faint)", marginTop:2 }}>{D.usd0(e.amount)}</div></td>
                <td style={{...tdS, textAlign:"end"}}><Chip state={e.state} label={e.state.toUpperCase().replace(" ","_")} /></td></tr>
            ))}
          </DTable>
        </div>
      </div>
    </div>
  );
}

function AdmInvites({ app }) {
  const { t } = app; const D = window.DATA;
  const [rows, setRows] = useState(D.INVITES);
  const revoke = (code)=> setRows(rs=>rs.map(r=> r.code===code?{...r,status:"revoked"}:r));
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_invites")} />
      <DTable cols={[{label:"CODE"},{label:t("adm_issued_by")},{label:"USES"},{label:t("adm_chain")},{label:t("adm_status")},{label:t("adm_action"),end:true}]}>
        {rows.map(iv=>(
          <tr key={iv.code}>
            <td style={tdS}><span className="mono" style={{ color:"var(--gold-pale)" }}>{iv.code}</span></td>
            <td style={{...tdS, color:"var(--fg-muted)"}} className="mono">{iv.issuedBy}</td>
            <td style={tdS} className="mono">{iv.uses}</td>
            <td style={{...tdS, color:"var(--fg-muted)"}} className="mono">{iv.chain}</td>
            <td style={tdS}><Chip state={iv.status==="revoked"?"flagged":iv.status} label={iv.status.toUpperCase()} /></td>
            <td style={{...tdS, textAlign:"end"}}>{iv.status==="active"||iv.status==="flagged" ? <GBtn kind={iv.status==="flagged"?"bad":"ghost"} small onClick={()=>revoke(iv.code)}>{t("adm_revoke")}</GBtn> : <span className="faint mono" style={{ fontSize:11 }}>—</span>}</td>
          </tr>
        ))}
      </DTable>
    </div>
  );
}

function AdmAuth({ app }) {
  const { t } = app; const D = window.DATA;
  const [kyc, setKyc] = useState(D.KYC_QUEUE);
  const [cert, setCert] = useState(D.CERT_QUEUE);
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_closet")} />
      <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:10 }}>{t("adm_pending_kyc")}</div>
      <DTable cols={[{label:t("adm_member")},{label:"DOCUMENT"},{label:t("adm_issued_by")},{label:t("adm_status")},{label:t("adm_action"),end:true}]}>
        {kyc.map(k=>(
          <tr key={k.id}>
            <td style={tdS}><span className="mono" style={{ color:"var(--gold-pale)" }}>{k.member}</span></td>
            <td style={tdS}>{k.doc}</td>
            <td style={{...tdS, color:"var(--fg-muted)"}} className="mono">{k.issuedBy}</td>
            <td style={tdS}><Chip state={k.status} label={k.status.toUpperCase()} /></td>
            <td style={{...tdS, textAlign:"end"}}>{k.status==="pending"
              ? <div style={{ display:"flex", gap:8, justifyContent:"flex-end" }}>
                  <GBtn kind="gold" small onClick={()=>setKyc(q=>q.map(x=>x.id===k.id?{...x,status:"approved"}:x))}>{t("adm_approve")}</GBtn>
                  <GBtn kind="bad" small onClick={()=>setKyc(q=>q.map(x=>x.id===k.id?{...x,status:"rejected"}:x))}>{t("adm_reject")}</GBtn>
                </div>
              : <span className="faint mono" style={{ fontSize:11 }}>✓</span>}</td>
          </tr>
        ))}
      </DTable>

      <div className="mono up" style={{ fontSize:10, color:"var(--gold)", margin:"26px 0 10px" }}>{t("adm_closet")} · CERTIFIED PRE-OWNED</div>
      <DTable cols={[{label:t("adm_object")},{label:"MAISON"},{label:t("adm_value")},{label:t("adm_status")},{label:t("adm_action"),end:true}]}>
        {cert.map(c=>(
          <tr key={c.id}>
            <td style={tdS}>{c.object}</td>
            <td style={{...tdS, color:"var(--fg-muted)"}}>{c.maison}</td>
            <td style={tdS} className="mono">{D.usd0(c.value)}</td>
            <td style={tdS}><Chip state={c.status} label={c.status.toUpperCase()} /></td>
            <td style={{...tdS, textAlign:"end"}}>{c.status==="appraising"
              ? <GBtn kind="gold" small onClick={()=>setCert(q=>q.map(x=>x.id===c.id?{...x,status:"certified"}:x))}>{t("adm_certify")}</GBtn>
              : <span style={{ display:"inline-flex", color:"var(--gold)" }}><Seal size={26} label="" sub="" id="" date="" /></span>}</td>
          </tr>
        ))}
      </DTable>
    </div>
  );
}

function AdmEscrow({ app }) {
  const { t } = app; const D = window.DATA;
  const [rows, setRows] = useState(D.ESCROW);
  const disputed = rows.find(r=>r.state==="disputed");
  const [ruling, setRuling] = useState(null);
  return (
    <div className="fade-up">
      <SecHead kicker={t("adm_title")} title={t("adm_escrow")} />
      <DTable cols={[{label:"ESCROW"},{label:t("adm_object")},{label:t("adm_member")},{label:t("adm_value")},{label:"PREMIUM"},{label:t("adm_status")},{label:t("adm_action"),end:true}]}>
        {rows.map(e=>(
          <tr key={e.id}>
            <td style={tdS}><span className="mono" style={{ color:"var(--gold-pale)" }}>{e.id}</span></td>
            <td style={tdS}>{e.lot}</td>
            <td style={{...tdS, color:"var(--fg-muted)"}} className="mono">{e.member}</td>
            <td style={tdS} className="mono">{D.usd0(e.amount)}</td>
            <td style={{...tdS, color:"var(--fg-muted)"}} className="mono">{D.usd0(e.premium)}</td>
            <td style={tdS}><Chip state={e.state} label={e.state.toUpperCase().replace(" ","_")} /></td>
            <td style={{...tdS, textAlign:"end"}}>
              {e.state==="disputed" ? <GBtn kind="bad" small onClick={()=>setRuling(e)}>{t("adm_resolve")}</GBtn>
                : e.state==="in_transit" ? <GBtn small>{t("adm_hold")}</GBtn>
                : <span className="faint mono" style={{ fontSize:11 }}>—</span>}
            </td>
          </tr>
        ))}
      </DTable>

      {disputed && (
        <div style={{ marginTop:22, border:"1px solid var(--st-bad)", borderRadius:"var(--r-2)", overflow:"hidden" }}>
          <div style={{ padding:"14px 18px", background:"var(--st-bad-bg)", display:"flex", alignItems:"center", gap:12, borderBottom:"1px solid var(--st-bad)" }}>
            <span style={{ color:"var(--st-bad)" }}><Icon name="scale" size={22}/></span>
            <div><div className="serif" style={{ fontSize:17, color:"var(--fg)" }}>Dispute Court · {disputed.id}</div>
              <div className="mono" style={{ fontSize:11.5, color:"var(--fg-muted)", marginTop:2 }}>{disputed.lot} · buyer claims non-authentic on delivery</div></div>
          </div>
          <div style={{ padding:"18px", display:"flex", gap:12, alignItems:"center" }}>
            <span className="muted" style={{ fontSize:13 }}>{t("adm_resolve")}:</span>
            <GBtn kind="ghost" small onClick={()=>setRuling({...disputed, r:"refund"})}>REFUND_BUYER</GBtn>
            <GBtn kind="ghost" small onClick={()=>setRuling({...disputed, r:"release"})}>RELEASE_SELLER</GBtn>
            <GBtn kind="ghost" small onClick={()=>setRuling({...disputed, r:"split"})}>SPLIT</GBtn>
            {ruling?.r && <span className="chip" data-st={ruling.r==="split"?"warn":ruling.r==="refund"?"bad":"good"} style={{ marginInlineStart:"auto" }}>RULED · {ruling.r.toUpperCase()}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

Object.assign(window, { AdminApp });
