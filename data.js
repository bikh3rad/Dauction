/* ============================================================
   DAUCTION — sample data (window.DATA)
   Numbers follow the house style: thousands separators, USDC suffix,
   TRADE-token never prefixed. Maison names are invented (no real marks).
   ============================================================ */
(function () {
  const usd = (n) => "$" + n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) + " USDC";
  const usd0 = (n) => "$" + n.toLocaleString("en-US") + " USDC";
  // trilingual field resolver: tt({en,fa,ar}, lang) → string (falls back to en, or passthrough if plain string)
  const tt = (v, lang) => (v && typeof v === "object") ? (v[lang] || v.en) : v;

  // category → {en,fa,ar} label + art key
  const CATS = {
    horology:  { en:"Horology", fa:"ساعت", ar:"الساعات", tr:"Saat", g:"watch" },
    bag:       { en:"Haute Maroquinerie", fa:"کیف لوکس", ar:"حقائب فاخرة", tr:"Lüks Çanta", g:"bag" },
    sneaker:   { en:"Grail Sneaker", fa:"اسنیکر نایاب", ar:"حذاء نادر", tr:"Nadir Sneaker", g:"sneaker" },
    perfume:   { en:"Rare Perfume", fa:"عطر نایاب", ar:"عطر نادر", tr:"Nadir Parfüm", g:"bottle" },
    art:       { en:"Blue-Chip Art", fa:"اثر هنری", ar:"فن أيقوني", tr:"Seçkin Sanat", g:"frame" },
    painting:  { en:"Fine Painting", fa:"نقاشی نفیس", ar:"لوحة فنية", tr:"Güzel Tablo", g:"painting" },
    jewel:     { en:"Haute Joaillerie", fa:"جواهر", ar:"مجوهرات", tr:"Yüksek Mücevher", g:"ring" },
  };

  // auction modes
  // dutch   = live descending price (active)
  // vickrey = sealed second-price, timed (passive)
  // uniqbid = lowest unique bid, timed (passive)
  const ATYPES = {
    dutch:   { en:"Dutch · live",   fa:"هلندی · زنده",  ar:"هولندي · مباشر", tr:"Hollanda · canlı" },
    vickrey: { en:"Vickrey · sealed", fa:"ویکری · دربسته", ar:"فيكري · مغلق", tr:"Vickrey · kapalı" },
    uniqbid: { en:"UniqBid · timed", fa:"یکتاقیمت · زمان‌دار", ar:"العرض الفريد · مؤقّت", tr:"UniqBid · süreli" },
  };

  const LOTS = [
    /* ---- THE single live auction ---- */
    {
      id:"lot-07", no:7, ref:"DXB·26·W23·007",
      maison:"Patek Philippe",
      title:{ en:"Nautilus 5711/1A — Tiffany Blue Dial", fa:"ناتیلوس ۵۷۱۱/۱A — صفحهٔ آبی تیفانی", ar:"نوتيلوس 5711/1A — مينا تيفاني الأزرق", tr:"Nautilus 5711/1A — Tiffany Mavi Kadran" },
      desc:{
        en:"The discontinued steel Nautilus with the Tiffany & Co. co-signed dial — arguably the most coveted reference of the decade. Delivered full set, unworn, with a single recorded owner.",
        fa:"ناتیلوس استیلِ از تولید خارج‌شده با صفحهٔ هم‌امضای تیفانی اند کو — بحث‌برانگیزترین و خواستنی‌ترین رفرنس این دهه. کامل، استفاده‌نشده و تک‌مالک تحویل می‌شود.",
        ar:"نوتيلوس الفولاذي المتوقّف بمينا موقّع مع تيفاني آند كو — أكثر المراجع رغبةً في العقد. يُسلّم بالطقم الكامل، غير ملبوس، بمالك واحد مسجّل.", tr:"Üretimi durdurulmuş çelik Nautilus, Tiffany & Co. ortak imzalı kadranıyla — kuşkusuz on yılın en çok arzulanan referansı. Tam set, kullanılmamış, tek kayıtlı sahip." },
      cat:"horology", year:2022, condition:"Unworn · full set",
      floor:620000, start:1180000, dropStep:2500, dropEvery:22,
      watching:412, participants:14, threshold:10,
      inspector:"0xA1·Horology Lab DXB", premium:0.10, box:true,
      provenance:{ en:"Tiffany & Co. co-signed dial, NY allocation 2022. Single owner.", fa:"صفحهٔ هم‌امضای تیفانی، تخصیص نیویورک ۲۰۲۲. تک‌مالک.", ar:"مينا موقّع مع تيفاني، تخصيص نيويورك 2022. مالك واحد.", tr:"Tiffany & Co. ortak imzalı kadran, NY tahsisi 2022. Tek sahip." },
      status:"live", live:true, accent:"#1d3a4a",
    },
    {
      id:"lot-08", no:8, ref:"DXB·26·W23·008",
      maison:"Hermès",
      title:{ en:"Birkin 25 Himalaya — Niloticus, Diamond Hardware", fa:"برکین ۲۵ هیمالیا — پوست نیلوتیکوس، یراق الماس", ar:"بيركين 25 هيمالايا — نيلوتيكوس، تجهيزات ماسية", tr:"Birkin 25 Himalaya — Niloticus, Pırlanta Aksam" },
      desc:{
        en:"The grail of collectible handbags: matte Niloticus crocodile graduated to evoke a snow-capped peak, with 18k white-gold and diamond hardware. Store fresh, with CITES papers.",
        fa:"جام مقدسِ کیف‌های کلکسیونی: پوست کروکودیل نیلوتیکوسِ مات با سایه‌روشنی که قلهٔ برف‌گرفته را تداعی می‌کند، با یراق طلای سفید ۱۸ عیار و الماس. نو، همراه مدارک CITES.",
        ar:"كأس حقائب الجمع: جلد تمساح نيلوتيكوس مطفأ متدرّج يحاكي قمة مكسوّة بالثلج، بتجهيزات من الذهب الأبيض عيار 18 والماس. جديدة بأوراق CITES.", tr:"Koleksiyon çantalarının kutsal kâsesi: kar örtülü zirveyi andıran mat Niloticus timsahı, 18 ayar beyaz altın ve pırlanta aksamla. Mağaza tazeliğinde, CITES belgeli." },
      cat:"bag", year:2021, condition:"Pristine · store fresh", floor:285000, start:520000,
      dropStep:1500, dropEvery:24, watching:366, participants:9, threshold:8,
      inspector:"0xC4·Leather Atelier", premium:0.12, box:true,
      provenance:{ en:"Faubourg boutique acquisition, Paris (2021). With CITES.", fa:"خریداری از بوتیک فوبور، پاریس (۲۰۲۱). همراه CITES.", ar:"شراء من بوتيك فوبور، باريس (2021). مع CITES.", tr:"Faubourg butik alımı, Paris (2021). CITES belgeli." },
      status:"upcoming", opensIn:1820, accent:"#5E1226",
    },
    {
      id:"lot-09", no:9, ref:"DXB·26·W23·009",
      maison:"Richard Mille",
      title:{ en:"RM 11-03 Flyback Chronograph — Titanium", fa:"RM 11-03 کرونوگراف فلای‌بک — تیتانیوم", ar:"RM 11-03 كرونوغراف فلاي باك — تيتانيوم", tr:"RM 11-03 Flyback Kronograf — Titanyum" },
      desc:{
        en:"A skeletonised automatic flyback chronograph in grade-5 titanium — featherlight on the wrist, engineered like a racing chassis. Recently serviced by an authorised watchmaker.",
        fa:"کرونوگراف فلای‌بکِ اتوماتیکِ اسکلتی در تیتانیوم گرید ۵ — روی مچ بسیار سبک، با مهندسی‌ای مانند شاسی مسابقه. به‌تازگی توسط ساعت‌سازِ مجاز سرویس شده.",
        ar:"كرونوغراف فلاي باك أوتوماتيكي مفرّغ من التيتانيوم عيار 5 — خفيف للغاية على المعصم، مهندَس كهيكل سباق. خضع لصيانة حديثة لدى صانع ساعات معتمد.", tr:"5. sınıf titanyumdan iskelet otomatik flyback kronograf — bilekte tüy gibi, yarış şasisi gibi mühendislik. Yetkili saat ustasınca yakında servis edildi." },
      cat:"horology", year:2023, condition:"Mint · serviced", floor:198000, start:395000,
      dropStep:1000, dropEvery:20, watching:241, participants:7, threshold:8,
      inspector:"0xA1·Horology Lab DXB", premium:0.10, box:true,
      provenance:{ en:"Authorised dealer, Dubai Mall (2023).", fa:"نمایندگی مجاز، دبی مال (۲۰۲۳).", ar:"وكيل معتمد، دبي مول (2023).", tr:"Yetkili bayi, Dubai Mall (2023)." },
      status:"upcoming", opensIn:5400, accent:"#3a2f1a",
    },
    {
      id:"lot-10", no:10, ref:"DXB·26·W23·010",
      maison:"Yayoi Kusama",
      title:{ en:"Pumpkin (Yellow) — Screenprint, ed. 47/120", fa:"کدوتنبل (زرد) — سیلک‌اسکرین، نسخهٔ ۴۷/۱۲۰", ar:"اليقطين (الأصفر) — طباعة شاشية، نسخة 47/120", tr:"Balkabağı (Sarı) — Serigrafi, baskı 47/120" },
      desc:{
        en:"Kusama's signature motif in her instantly recognisable polka-dot language — a hand-pulled screenprint, numbered 47 of 120, archival-framed with gallery certificate.",
        fa:"نقش‌مایهٔ شاخص کوساما در زبان خال‌خالیِ بی‌درنگ‌شناخته‌شده‌اش — سیلک‌اسکرینِ دست‌چاپ، با شمارهٔ ۴۷ از ۱۲۰، قاب آرشیوی همراه گواهی گالری.",
        ar:"الزخرفة المميّزة لكوساما بلغتها النقطية المعروفة فوراً — طباعة شاشية مسحوبة يدوياً، مرقّمة 47 من 120، بإطار أرشيفي وشهادة معرض.", tr:"Kusama’nın anında tanınan puantiye diliyle imza motifi — elle çekilmiş serigrafi, 120’den 47 numara, arşiv çerçeveli ve galeri sertifikalı." },
      cat:"art", year:2004, condition:"Framed · archival", floor:96000, start:185000,
      dropStep:500, dropEvery:26, watching:158, participants:5, threshold:6,
      inspector:"0x3D·Fine Art Survey", premium:0.12, box:false,
      provenance:{ en:"Two private collections; gallery COA included.", fa:"دو کلکسیون خصوصی؛ همراه گواهی اصالت گالری.", ar:"مجموعتان خاصتان؛ مع شهادة أصالة المعرض.", tr:"İki özel koleksiyon; galeri orijinallik belgesi dahil." },
      status:"upcoming", opensIn:9000, accent:"#4A0F1E",
    },
    {
      id:"lot-11", no:11, ref:"DXB·26·W23·011",
      maison:"Nike × Dior",
      title:{ en:"Air Jordan 1 High — 1 of 8,500, EU exclusive", fa:"ایر جردن ۱ های — ۱ از ۸٬۵۰۰، انحصاری اروپا", ar:"إير جوردن 1 هاي — 1 من 8,500، حصري لأوروبا", tr:"Air Jordan 1 High — 8.500’den 1’i, AB özel" },
      desc:{
        en:"The Dior-collaboration Air Jordan 1, numbered 1 of 8,500 from the European allocation. Deadstock, unworn, with the original box, carrier bag and receipt.",
        fa:"ایر جردن ۱ از همکاری با دیور، با شمارهٔ ۱ از ۸٬۵۰۰ از تخصیص اروپا. آک‌بند، استفاده‌نشده، با جعبه، ساک و رسید اصلی.",
        ar:"إير جوردن 1 بالتعاون مع ديور، مرقّمة 1 من 8,500 من التخصيص الأوروبي. جديدة غير ملبوسة، مع العلبة والحقيبة والإيصال الأصلي.", tr:"Dior iş birliğiyle Air Jordan 1, Avrupa tahsisinden 8.500’den 1 numara. Kullanılmamış, orijinal kutu, taşıma çantası ve fişiyle." },
      cat:"sneaker", year:2020, condition:"Deadstock · receipt", floor:11000, start:27000,
      dropStep:150, dropEvery:18, watching:489, participants:11, threshold:8,
      inspector:"0xB2·Sneaker Auth Co.", premium:0.13, box:true,
      provenance:{ en:"Unworn, original box & carrier bag.", fa:"استفاده‌نشده، جعبه و ساک اصلی.", ar:"غير ملبوسة، العلبة والحقيبة الأصلية.", tr:"Kullanılmamış, orijinal kutu ve taşıma çantası." },
      status:"upcoming", opensIn:14400, accent:"#2a2a2a",
    },
    {
      id:"lot-12", no:12, ref:"DXB·26·W23·012",
      maison:"Clive Christian",
      title:{ en:"No. 1 Imperial Majesty — Baccarat Flacon", fa:"نامبر ۱ ایمپریال مجستی — فلاکن باکارا", ar:"رقم 1 إمبريال ماجستي — قارورة باكارات", tr:"No. 1 Imperial Majesty — Baccarat Flakon" },
      desc:{
        en:"One of only ten flacons produced: a hand-blown Baccarat crystal bottle with a five-carat diamond collar and 24k gold neck. Sealed, in its presentation case.",
        fa:"یکی از تنها ده فلاکن تولیدشده: شیشهٔ کریستال باکاراتِ دست‌دمیده با حلقهٔ الماسِ پنج‌قیراطی و گردنِ طلای ۲۴ عیار. پلمب، در جعبهٔ نمایش.",
        ar:"واحدة من عشر قوارير فقط: زجاجة كريستال باكارات منفوخة يدوياً بطوق ماسي من خمسة قراريط وعنق من ذهب 24 قيراطاً. مختومة في علبة العرض.", tr:"Üretilen yalnızca on flakondan biri: beş karat pırlanta tasmalı ve 24 ayar altın boyunlu, elle üflenmiş Baccarat kristal şişe. Mühürlü, sunum kutusunda." },
      cat:"perfume", year:2019, condition:"Sealed · 1 of 10", floor:42000, start:96000,
      dropStep:400, dropEvery:22, watching:127, participants:4, threshold:6,
      inspector:"0x9F·Maison Verification", premium:0.15, box:true,
      provenance:{ en:"Five-carat diamond collar, 24k gold neck. Sealed.", fa:"حلقهٔ الماس پنج‌قیراطی، گردن طلای ۲۴ عیار. پلمب.", ar:"طوق ماسي خمسة قراريط، عنق ذهب 24 قيراطاً. مختومة.", tr:"Beş karatlık pırlanta tasma, 24 ayar altın boyun. Mühürlü." },
      status:"upcoming", opensIn:21600, accent:"#3A0A16",
    },

    /* ---- PASSIVE · VICKREY (sealed second-price), owner-set 5-day window ---- */
    {
      id:"lot-13", no:13, ref:"DXB·26·W23·013",
      maison:"Estate Commission",
      title:{ en:"Crimson Atrium — Oil on Linen, signed", fa:"دالان قرمز — رنگ‌روغن روی کتان، امضاشده", ar:"الردهة القرمزية — زيت على الكتان، موقّعة", tr:"Kızıl Atriyum — Keten üzerine yağlıboya, imzalı" },
      desc:{
        en:"A large signed oil from a private estate — deep crimson and gilt, gallery-framed. Offered as a sealed Vickrey auction: submit one hidden bid; the second-highest price wins, paid at that price.",
        fa:"یک رنگ‌روغن بزرگ و امضاشده از یک مجموعهٔ خصوصی — قرمز عمیق و طلایی، با قاب گالری. به‌صورت حراج دربستهٔ ویکری عرضه می‌شود: یک پیشنهاد پنهان ثبت کنید؛ دومین قیمت بالا برنده است و به همان قیمت پرداخت می‌شود.",
        ar:"لوحة زيتية كبيرة موقّعة من مقتنيات خاصة — قرمزي عميق وذهبي، بإطار معرض. تُعرض كمزاد فيكري مغلق: قدّم عرضاً واحداً مخفياً؛ يفوز ثاني أعلى سعر ويُدفع بذلك السعر.", tr:"Özel bir mülkten büyük, imzalı bir yağlıboya — derin kızıl ve yaldız, galeri çerçeveli. Kapalı Vickrey açık artırması olarak sunulur: tek gizli teklif verin; ikinci en yüksek fiyat kazanır ve o fiyattan ödenir." },
      cat:"painting", year:2014, condition:"Gallery framed",
      atype:"vickrey", floor:54000, reserve:54000, closesIn:5*86400, durationDays:5,
      watching:96, bidsPlaced:41, threshold:0, premium:0.12, box:false,
      inspector:"0x3D·Fine Art Survey",
      provenance:{ en:"Single estate, never publicly exhibited.", fa:"یک مجموعهٔ خصوصی، هرگز به‌طور عمومی به نمایش درنیامده.", ar:"مقتنى واحد، لم يُعرض علناً قط.", tr:"Tek mülk, daha önce hiç sergilenmedi." },
      status:"passive", accent:"#4A0F1E",
    },
    /* ---- PASSIVE · UNIQBID (lowest unique price), owner-set 7-day window ---- */
    {
      id:"lot-14", no:14, ref:"DXB·26·W23·014",
      maison:"Hermès",
      title:{ en:"Kelly Sellier 25 — Rouge Casaque", fa:"کلی سلیه ۲۵ — رژ کازاک", ar:"كيلي سيلييه 25 — أحمر كازاك", tr:"Kelly Sellier 25 — Rouge Casaque" },
      desc:{
        en:"A coveted Kelly in Rouge Casaque epsom with gold hardware. Offered as UniqBid: place as many unique prices as you like — the lowest price that no one else has chosen wins.",
        fa:"یک کلی خواستنی در چرم اپسوم رژ کازاک با یراق طلایی. به‌صورت یکتاقیمت عرضه می‌شود: هر تعداد قیمت یکتا که می‌خواهید ثبت کنید — کمترین قیمتی که هیچ‌کس دیگری انتخاب نکرده برنده است.",
        ar:"كيلي مرغوبة بلون أحمر كازاك إبسوم وتجهيزات ذهبية. تُعرض بنظام العرض الفريد: قدّم ما تشاء من الأسعار الفريدة — يفوز أقل سعر لم يختره أحد غيرك.", tr:"Altın aksamlı Rouge Casaque epsom deride çok arzulanan bir Kelly. UniqBid olarak sunulur: istediğiniz kadar benzersiz fiyat verin — başkasının seçmediği en düşük fiyat kazanır." },
      cat:"bag", year:2022, condition:"Unworn · full set",
      atype:"uniqbid", floor:38000, closesIn:7*86400, durationDays:7,
      watching:312, bidsPlaced:184, threshold:0, premium:0.13, box:true,
      inspector:"0xC4·Leather Atelier",
      provenance:{ en:"Boutique acquisition, 2022. Full set.", fa:"خرید از بوتیک، ۲۰۲۲. کامل.", ar:"شراء من بوتيك، 2022. طقم كامل.", tr:"Butik alımı, 2022. Tam set." },
      status:"passive", accent:"#6B1228",
    },
  ];

  // The signed-in member's own vault
  const VAULT = [
    { id:"v1", maison:"Rolex", title:{ en:"Daytona 116500LN — Panda Dial", fa:"دیتونا ۱۱۶۵۰۰LN — صفحهٔ پاندا", ar:"دايتونا 116500LN — مينا الباندا", tr:"Daytona 116500LN — Panda Kadran" }, cat:"horology",
      year:2021, value:38500, state:"in_closet", img:"vault-watch" },
    { id:"v2", maison:"Chanel", title:{ en:"Classic Flap Medium — Black Caviar", fa:"کلاسیک فلپ مدیوم — کاویار مشکی", ar:"كلاسيك فلاب متوسط — كافيار أسود", tr:"Classic Flap Medium — Siyah Caviar" }, cat:"bag",
      year:2022, value:12400, state:"appraising", img:"vault-bag" },
    { id:"v3", maison:"Andy Warhol", title:{ en:"Flowers (1970) — Screenprint", fa:"گل‌ها (۱۹۷۰) — سیلک‌اسکرین", ar:"زهور (1970) — طباعة شاشية", tr:"Çiçekler (1970) — Serigrafi" }, cat:"art",
      year:1970, value:96000, state:"in_auction", img:"vault-art" },
    { id:"v4", maison:"Cartier", title:{ en:"Love Bracelet — Pavé Diamond, Gold", fa:"دستبند لاو — پاوهٔ الماس، طلا", ar:"سوار لوف — ماس مرصوف، ذهب", tr:"Love Bilezik — Pavé Pırlanta, Altın" }, cat:"jewel",
      year:2020, value:28900, state:"sold", img:"vault-jewel" },
    /* ---- the member's own luxury painting at home, ready to list ---- */
    { id:"v5", maison:"Private Collection", title:{ en:"Gilded Horizon — Oil, home collection", fa:"افق زراندود — رنگ‌روغن، مجموعهٔ خانگی", ar:"الأفق المذهّب — زيت، مقتنى منزلي", tr:"Yaldızlı Ufuk — Yağlıboya, ev koleksiyonu" }, cat:"painting",
      year:2009, value:46000, state:"in_closet", img:"vault-painting", mine:true },
  ];

  // ---- bid-credit economy (passive auctions) ----
  // each bid credit = $1; spent to submit a bid in Vickrey / UniqBid auctions
  const BID_PACKAGES = [
    { id:"pkg-100", bids:100, price:80, perBid:0.80, save:"20%", best:true },
    { id:"pkg-50",  bids:50,  price:45, perBid:0.90, save:"10%", best:false },
    { id:"pkg-20",  bids:20,  price:20, perBid:1.00, save:"—",   best:false },
  ];

  const MEMBER = {
    name:"Member 0x7A4E", handle:"@aurelia.dxb", tier:"standard",
    walletUSDC:212400, vaultCredit:34850, bids:18, invitedBy:"MAISON · 0x11", joined:"2025·11",
    vaultAddress:"vault.dauction.xyz/0x7a4e",
  };

  const TIERS = [
    { key:"guest",    feeBuyer:"15%", access:"guest",    cadence:"—" },
    { key:"standard", feeBuyer:"12%", access:"standard", cadence:"Early access" },
    { key:"vip",      feeBuyer:"10%", access:"vip",      cadence:"VIP-first · gallery waivers" },
  ];

  // ---- Admin data ----
  const INVITES = [
    { code:"LUX-7F2A-9KQ", issuedBy:"0x11 · Maison", uses:"0 / 1", status:"active",  chain:"Maison → ?" },
    { code:"VELT-3C8-XQ2", issuedBy:"0x7A4E",        uses:"1 / 1", status:"redeemed", chain:"0x7A4E → 0x91" },
    { code:"NOIR-55K-A0",  issuedBy:"0x2D · VIP",    uses:"0 / 1", status:"active",  chain:"VIP → ?" },
    { code:"GHOST-99-ZZ",  issuedBy:"unknown",        uses:"3 / 1", status:"flagged",  chain:"⚠ over-use" },
    { code:"MAISON-04",    issuedBy:"House",          uses:"0 / 1", status:"active",  chain:"House → ?" },
  ];

  const KYC_QUEUE = [
    { id:"0x91", member:"Member 0x91", doc:"Emirates ID", issuedBy:"0x7A4E", status:"pending" },
    { id:"0x4C", member:"Member 0x4C", doc:"Passport · UK", issuedBy:"0x2D", status:"pending" },
    { id:"0xF7", member:"Member 0xF7", doc:"Emirates ID", issuedBy:"House", status:"approved" },
  ];

  const CERT_QUEUE = [
    { id:"lot-08", object:"Birkin 25 Himalaya", maison:"Hermès", value:285000, status:"appraising" },
    { id:"lot-09", object:"RM 11-03 Flyback", maison:"Richard Mille", value:198000, status:"appraising" },
    { id:"v2",     object:"Classic Flap Medium", maison:"Chanel", value:12400, status:"appraising" },
    { id:"lot-07", object:"Nautilus 5711/1A", maison:"Patek Philippe", value:620000, status:"certified" },
  ];

  const ESCROW = [
    { id:"esc-07", lot:"Nautilus 5711/1A", member:"0x7A4E", amount:962000, state:"funded", premium:96200 },
    { id:"esc-05", lot:"Flowers (1970)", member:"0x91", amount:96000, state:"in_transit", premium:11520 },
    { id:"esc-03", lot:"Love Bracelet", member:"0x4C", amount:28900, state:"completed", premium:3757 },
    { id:"esc-02", lot:"Air Jordan 1 × Dior", member:"0xF7", amount:11000, state:"disputed", premium:1430 },
  ];

  window.DATA = {
    usd, usd0, tt, CATS, ATYPES, LOTS, VAULT, MEMBER, TIERS, BID_PACKAGES,
    INVITES, KYC_QUEUE, CERT_QUEUE, ESCROW,
    lot: (id) => LOTS.find(l => l.id === id),
  };
})();
