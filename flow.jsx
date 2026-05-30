/* ============================================================
   DAUCTION — FLOW VIEW (desktop)
   The complete user-flow across every role, end to end.
   Swimlanes (roles) × phases, plus the escrow balance strip
   showing 0 → 10% reserve → 100% lock → settle → release.
   ============================================================ */
function FlowView({ app }) {
  const { t, lang, dir } = app;
  const D = window.DATA;
  const liveLot = D.LOTS.find(l => l.live) || D.LOTS[0];
  const floor = liveLot.floor;
  const tri = (o) => o[lang] || o.en;
  const [open, setOpen] = useState(null);

  const ROLES = [
    { key:"buyer",     icon:"user",   color:"var(--gold)" },
    { key:"house",     icon:"crown",  color:"var(--gold)" },
    { key:"inspector", icon:"shield", color:"var(--gold)" },
    { key:"seller",    icon:"store",  color:"var(--gold)" },
    { key:"escrow",    icon:"lock",   color:"var(--gold)" },
  ];
  const PHASES = ["discover","access","verify","reserve","live","settle","release"];

  // step = { role, phase, st, happy, text:{en,fa,ar}, detail:{en,fa,ar} }
  const S = [
    { role:"buyer", phase:"discover", st:"neut", happy:true,
      text:{en:"Browse the weekly gallery — open to everyone, no account.",fa:"تماشای گالری هفتگی — برای همه باز است، بدون حساب.",ar:"تصفّح المعرض الأسبوعي — مفتوح للجميع بدون حساب."} },
    { role:"buyer", phase:"access", st:"active", happy:true,
      text:{en:"Enter an invitation → tier rises to Member (one level up).",fa:"وارد کردن دعوت‌نامه → ارتقا به «عضو» (یک سطح بالاتر).",ar:"إدخال دعوة → الترقية إلى «عضو» (مستوى أعلى)."} },
    { role:"buyer", phase:"verify", st:"warn", happy:true,
      text:{en:"KYC: Emirates ID / passport + GCC OTP. Guest until approved.",fa:"احراز هویت: امارات آی‌دی/پاسپورت + OTP. تا تأیید، مهمان.",ar:"التحقّق: هوية إماراتية/جواز + OTP. ضيف حتى الموافقة."} },
    { role:"buyer", phase:"reserve", st:"active", happy:true,
      text:{en:"Request to participate → lock 10% reservation deposit.",fa:"درخواست شرکت → قفل ۱۰٪ ودیعهٔ رزرو.",ar:"طلب المشاركة → تجميد عربون حجز 10٪."} },
    { role:"buyer", phase:"live", st:"live", happy:true,
      text:{en:"At open, full amount locks. Buy as the price falls.",fa:"هنگام شروع، کل مبلغ قفل می‌شود. با کاهش قیمت بخر.",ar:"عند الفتح يُجمّد المبلغ الكامل. اشترِ مع هبوط السعر."} },
    { role:"buyer", phase:"settle", st:"active", happy:true,
      text:{en:"Win → 24 h to fund hammer + buyer's premium, or forfeit deposit.",fa:"برد → ۲۴ ساعت برای پرداخت چکش + کارمزد، وگرنه ضبط ودیعه.",ar:"فوز → 24 ساعة لدفع المطرقة + العمولة، وإلا يُصادر العربون."} },
    { role:"buyer", phase:"release", st:"good", happy:true,
      text:{en:"Confirm authenticity & delivery → escrow releases.",fa:"تأیید اصالت و تحویل → آزادسازی اسکرو.",ar:"تأكيد الأصالة والتسليم → الإفراج عن الضمان."} },

    { role:"house", phase:"access", st:"neut",
      text:{en:"Issue & track invite codes; monitor the invite chain.",fa:"صدور و ردیابی کدهای دعوت؛ پایش زنجیرهٔ دعوت.",ar:"إصدار وتتبّع رموز الدعوة؛ مراقبة سلسلة الدعوة."} },
    { role:"house", phase:"verify", st:"good",
      text:{en:"Approve KYC; certify object (Certified Pre-Owned).",fa:"تأیید KYC؛ صدور گواهی اصالت کالا.",ar:"الموافقة على التحقّق؛ توثيق القطعة."} },
    { role:"house", phase:"live", st:"live",
      text:{en:"Activate auction once participant threshold is funded.",fa:"فعال‌سازی حراج پس از رسیدن به حد نصاب تأمین‌شده.",ar:"تفعيل المزاد بعد اكتمال النصاب المموّل."} },
    { role:"house", phase:"settle", st:"bad",
      text:{en:"Dispute Court if a buyer claims non-authentic on delivery.",fa:"داوری در صورت ادعای عدم اصالت پس از تحویل.",ar:"محكمة النزاع عند ادعاء عدم الأصالة بعد التسليم."} },

    { role:"inspector", phase:"verify", st:"warn",
      text:{en:"Physical appraisal of the object before it enters auction.",fa:"کارشناسی فیزیکی کالا پیش از ورود به حراج.",ar:"فحص القطعة فعلياً قبل دخول المزاد."} },
    { role:"inspector", phase:"release", st:"good",
      text:{en:"Attestation seal recorded on-chain (CERTIFIED).",fa:"ثبت مهر اصالت روی زنجیره (CERTIFIED).",ar:"تسجيل ختم التوثيق على السلسلة (CERTIFIED)."} },

    { role:"seller", phase:"discover", st:"neut",
      text:{en:"List the object in the private Vault.",fa:"عرضهٔ کالا در کمد خصوصی.",ar:"إدراج القطعة في الخزنة الخاصة."} },
    { role:"seller", phase:"reserve", st:"neut",
      text:{en:"Approve moving the object to the weekly auction.",fa:"تأیید انتقال کالا به حراج هفتگی.",ar:"الموافقة على نقل القطعة إلى المزاد الأسبوعي."} },
    { role:"seller", phase:"release", st:"good",
      text:{en:"Receive 100% cash — or 110% as Vault Credit.",fa:"دریافت ۱۰۰٪ نقد — یا ۱۱۰٪ اعتبار کمد.",ar:"استلام 100٪ نقداً — أو 110٪ كرصيد خزنة."} },

    { role:"escrow", phase:"reserve", st:"active",
      text:{en:"Freeze 10% reservation deposit on request.",fa:"قفل ۱۰٪ ودیعهٔ رزرو هنگام درخواست.",ar:"تجميد عربون الحجز 10٪ عند الطلب."} },
    { role:"escrow", phase:"live", st:"active",
      text:{en:"At open, lock 100% of every participant's amount.",fa:"هنگام شروع، قفل ۱۰۰٪ مبلغ همهٔ شرکت‌کنندگان.",ar:"عند الفتح، تجميد 100٪ من مبلغ كل مشارك."} },
    { role:"escrow", phase:"settle", st:"warn",
      text:{en:"Refund losers within 5 min; hold the winner's funds.",fa:"بازگردانی بازندگان ظرف ۵ دقیقه؛ نگه‌داشتن وجه برنده.",ar:"إعادة الخاسرين خلال 5 دقائق؛ الاحتفاظ بأموال الفائز."} },
    { role:"escrow", phase:"release", st:"good",
      text:{en:"Release to seller only on buyer confirmation.",fa:"آزادسازی به فروشنده تنها با تأیید خریدار.",ar:"الإفراج للبائع فقط عند تأكيد المشتري."} },
  ];

  const cell = (role, phase) => S.find(s => s.role === role && s.phase === phase);

  // escrow balance strip stops
  const STOPS = [
    { phase:"reserve", label:"10%", amt:Math.round(floor*0.10), sub:t("req_deposit") },
    { phase:"live",    label:"100%", amt:floor, sub:t("lockfull_title") },
    { phase:"settle",  label:"hammer", amt:Math.round(floor*0.92), sub:t("esc_hammer") },
    { phase:"release", label:"110%", amt:Math.round(floor*0.92*1.10), sub:t("clo_creditopt") },
  ];

  return (
    <div style={{ height:"100%", display:"flex", flexDirection:"column", background:"var(--bg-void)", color:"var(--fg)" }}>
      {/* header */}
      <div style={{ flexShrink:0, padding:"22px 28px 14px", borderBottom:"1px solid var(--line)",
        display:"flex", alignItems:"flex-end", justifyContent:"space-between", gap:20 }}>
        <div>
          <div className="mono up" style={{ fontSize:10, color:"var(--gold)", marginBottom:6 }}>{t("brand")} · {t("flow_sub")}</div>
          <h1 className="serif" style={{ fontSize:28, margin:0, color:"var(--gold-pale)" }}>{t("flow_title")}</h1>
        </div>
        <div style={{ display:"flex", alignItems:"center", gap:14 }}>
          <span style={{ display:"flex", alignItems:"center", gap:7, fontSize:12, color:"var(--fg-muted)" }}>
            <span style={{ width:14, height:14, borderRadius:3, background:"var(--gold)", display:"inline-block" }} /> {t("flow_legend")}
          </span>
          <LangPill app={app} />
        </div>
      </div>

      {/* escrow balance strip */}
      <div style={{ flexShrink:0, padding:"16px 28px", borderBottom:"1px solid var(--line)", background:"linear-gradient(90deg,var(--burg-deep),transparent)" }}>
        <div className="mono up" style={{ fontSize:9.5, color:"var(--gold)", marginBottom:10 }}>{t("flow_escrow_state")} · {liveLot.maison} {liveLot.title}</div>
        <div style={{ display:"flex", alignItems:"center", gap:0 }}>
          {STOPS.map((s,i)=>(
            <React.Fragment key={s.phase}>
              <div style={{ flex:"0 0 auto", textAlign:"center" }}>
                <div style={{ width:13, height:13, borderRadius:"50%", background:"var(--gold)", margin:"0 auto 7px",
                  boxShadow:"0 0 0 4px var(--st-live-bg)" }} />
                <div className="mono" style={{ fontSize:13, color:"var(--gold-pale)", fontWeight:600 }}>{D.usd0(s.amt)}</div>
                <div className="mono up" style={{ fontSize:8.5, color:"var(--fg-faint)", marginTop:2 }}>{s.label} · {s.sub}</div>
              </div>
              {i<STOPS.length-1 && <div style={{ flex:1, height:2, background:"linear-gradient(90deg,var(--gold),var(--gold-line))", margin:"0 8px", marginBottom:24 }} />}
            </React.Fragment>
          ))}
        </div>
      </div>

      {/* swimlane grid */}
      <div style={{ flex:1, minHeight:0, overflow:"auto", padding:"0 28px 24px" }}>
        <div style={{ display:"grid", gridTemplateColumns:`140px repeat(${PHASES.length}, 1fr)`, gap:0, minWidth:980 }}>
          {/* phase header row */}
          <div style={{ position:"sticky", top:0, zIndex:3, background:"var(--bg-void)" }} />
          {PHASES.map((p,i)=>(
            <div key={p} style={{ position:"sticky", top:0, zIndex:3, background:"var(--bg-void)", padding:"14px 8px 10px", textAlign:"center", borderBottom:"1px solid var(--line)" }}>
              <div className="mono" style={{ width:22, height:22, borderRadius:"50%", border:"1px solid var(--gold-line)", color:"var(--gold)",
                display:"flex", alignItems:"center", justifyContent:"center", fontSize:11, margin:"0 auto 7px" }}>{i+1}</div>
              <div className="up" style={{ fontSize:10, letterSpacing:"0.1em", color:"var(--fg-muted)", fontWeight:600 }}>{t("flow_phase_"+p)}</div>
            </div>
          ))}

          {/* role rows */}
          {ROLES.map(role=>(
            <React.Fragment key={role.key}>
              <div style={{ display:"flex", alignItems:"center", gap:9, padding:"12px 10px", borderBottom:"1px solid var(--line)",
                borderInlineEnd:"1px solid var(--line)", position:"sticky", insetInlineStart:0, background:"var(--bg-void)", zIndex:2 }}>
                <span style={{ color:role.color }}><Icon name={role.icon} size={18} /></span>
                <span className="serif" style={{ fontSize:14.5, color:"var(--fg)" }}>{t("role_"+role.key)}</span>
              </div>
              {PHASES.map(phase=>{
                const c = cell(role.key, phase);
                return (
                  <div key={phase} style={{ padding:6, borderBottom:"1px solid var(--line)", borderInlineEnd:"1px solid var(--line)", minHeight:84 }}>
                    {c && (
                      <button onClick={()=>setOpen(c)} className="flow-cell" style={{ width:"100%", height:"100%", textAlign:"start", cursor:"pointer",
                        borderRadius:"var(--r-2)", padding:"9px 10px", border:"1px solid",
                        borderColor: c.happy?"var(--gold-line)":"var(--line)",
                        background: c.happy?"linear-gradient(135deg,rgba(201,162,75,0.10),var(--bg-1))":"var(--bg-1)",
                        color:"var(--fg)", display:"flex", flexDirection:"column", gap:7 }}>
                        <Chip state={c.st} label={(c.st==="live"?"LIVE":c.st==="active"?"ESCROW":c.st==="good"?"DONE":c.st==="bad"?"DISPUTE":c.st==="warn"?"REVIEW":"OPEN")} pulse={c.st==="live"} />
                        <span style={{ fontSize:11.5, lineHeight:1.4, color:"var(--fg)" }}>{tri(c.text)}</span>
                      </button>
                    )}
                  </div>
                );
              })}
            </React.Fragment>
          ))}
        </div>
      </div>

      {open && (
        <div onClick={()=>setOpen(null)} style={{ position:"absolute", inset:0, zIndex:80, background:"rgba(8,5,6,0.72)",
          backdropFilter:"blur(6px)", display:"flex", alignItems:"center", justifyContent:"center", padding:30 }}>
          <div onClick={e=>e.stopPropagation()} className="fade-up" dir={dir} style={{ width:"100%", maxWidth:440, background:"var(--bg-1)",
            border:"1px solid var(--gold-line)", borderRadius:"var(--r-3)", padding:"26px 28px" }}>
            <div style={{ display:"flex", alignItems:"center", gap:10, marginBottom:14 }}>
              <span style={{ color:"var(--gold)" }}><Icon name={ROLES.find(r=>r.key===open.role).icon} size={20} /></span>
              <span className="serif" style={{ fontSize:18, color:"var(--gold-pale)" }}>{t("role_"+open.role)}</span>
              <span className="mono up" style={{ fontSize:9, color:"var(--fg-faint)", marginInlineStart:"auto" }}>{t("flow_phase_"+open.phase)}</span>
            </div>
            <p style={{ fontSize:15, lineHeight:1.65, color:"var(--fg)", margin:0 }}>{tri(open.text)}</p>
            <button className="btn btn-gold" style={{ width:"100%", marginTop:22 }} onClick={()=>setOpen(null)}>{t("common_close")}</button>
          </div>
        </div>
      )}
    </div>
  );
}

Object.assign(window, { FlowView });
