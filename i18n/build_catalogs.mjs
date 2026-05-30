#!/usr/bin/env node
// Builds i18n/{en,fa,ar,tr}.json from two sources, so the catalogs never drift:
//   1. UI strings  — ported VERBATIM from the prototype's ../i18n.js (every key).
//   2. err.<CODE>  — the error-code -> message table (authored below), one entry
//      per dauction.common.v1.ErrorCode the API can return.
// The `_dir` meta field is dropped (it lives in locales.json instead).
//
// Run:  node i18n/build_catalogs.mjs   (then commit the JSON; web/ consumes it verbatim)
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');

// --- 1. load the prototype dictionary (IIFE assigns window.I18N) ---
const protoSrc = readFileSync(join(root, 'i18n.js'), 'utf8');
const win = {};
// eslint-disable-next-line no-new-func
new Function('window', protoSrc)(win);
const dict = win.I18N.dict;
const LANGS = win.I18N.langs; // ["en","fa","ar","tr"]

// --- 2. error-code -> message table (client maps ErrorCode -> localized text) ---
const ERR = {
  VALIDATION_FAILED: {
    en: "Please check the highlighted fields and try again.",
    fa: "لطفاً فیلدهای مشخص‌شده را بررسی کنید و دوباره تلاش کنید.",
    ar: "يرجى التحقق من الحقول المحدّدة والمحاولة مرة أخرى.",
    tr: "Lütfen işaretli alanları kontrol edip tekrar deneyin." },
  RESOURCE_NOT_FOUND: {
    en: "Not found.", fa: "یافت نشد.", ar: "غير موجود.", tr: "Bulunamadı." },
  RESOURCE_EXISTS: {
    en: "This already exists.", fa: "این مورد از قبل وجود دارد.",
    ar: "هذا موجود بالفعل.", tr: "Bu zaten mevcut." },
  RESOURCE_INVALID: {
    en: "That action isn’t allowed right now.", fa: "این اقدام در حال حاضر مجاز نیست.",
    ar: "هذا الإجراء غير مسموح به حالياً.", tr: "Bu işlem şu anda yapılamaz." },
  ACCESS_DENIED: {
    en: "You don’t have access to this.", fa: "شما به این بخش دسترسی ندارید.",
    ar: "ليس لديك صلاحية الوصول إلى هذا.", tr: "Buna erişiminiz yok." },
  UNAUTHENTICATED: {
    en: "Please sign in to continue.", fa: "برای ادامه وارد شوید.",
    ar: "يرجى تسجيل الدخول للمتابعة.", tr: "Devam etmek için giriş yapın." },
  RATE_LIMITED: {
    en: "Too many requests. Please slow down.", fa: "درخواست‌های بیش از حد. کمی آهسته‌تر.",
    ar: "طلبات كثيرة جداً. يرجى التمهّل.", tr: "Çok fazla istek. Lütfen yavaşlayın." },
  INTERNAL_ERROR: {
    en: "Something went wrong on our end. Please try again.",
    fa: "خطایی از سمت ما رخ داد. دوباره تلاش کنید.",
    ar: "حدث خطأ لدينا. يرجى المحاولة مرة أخرى.",
    tr: "Tarafımızda bir hata oluştu. Lütfen tekrar deneyin." },

  INVITE_INVALID: {
    en: "Invalid or expired code. Each invitation admits one member.",
    fa: "کد نامعتبر یا منقضی است. هر دعوت‌نامه تنها یک عضو را می‌پذیرد.",
    ar: "رمز غير صالح أو منتهٍ. كل دعوة تقبل عضواً واحداً.",
    tr: "Geçersiz ya da süresi dolmuş kod. Her davet bir üyeyi kabul eder." },
  INVITE_ALREADY_REDEEMED: {
    en: "This invitation has already been redeemed.", fa: "این دعوت‌نامه قبلاً استفاده شده است.",
    ar: "تم استخدام هذه الدعوة بالفعل.", tr: "Bu davet zaten kullanılmış." },
  INVITE_REVOKED: {
    en: "This invitation has been revoked.", fa: "این دعوت‌نامه لغو شده است.",
    ar: "تم إلغاء هذه الدعوة.", tr: "Bu davet iptal edilmiş." },
  KYC_REQUIRED: {
    en: "Verify your identity to participate.", fa: "برای شرکت، هویت خود را احراز کنید.",
    ar: "تحقّق من هويتك للمشاركة.", tr: "Katılmak için kimliğinizi doğrulayın." },
  KYC_PENDING: {
    en: "Your verification is under review. Guest access until approved.",
    fa: "احراز هویت شما در حال بررسی است. تا تأیید، دسترسی مهمان.",
    ar: "تحققك قيد المراجعة. وصول الضيف حتى الموافقة.",
    tr: "Doğrulamanız inceleniyor. Onaya dek Misafir erişimi." },
  KYC_REJECTED: {
    en: "Your verification was not approved. Please resubmit.",
    fa: "احراز هویت شما تأیید نشد. لطفاً دوباره ارسال کنید.",
    ar: "لم تتم الموافقة على تحققك. يرجى إعادة الإرسال.",
    tr: "Doğrulamanız onaylanmadı. Lütfen yeniden gönderin." },
  TIER_TOO_LOW: {
    en: "An invitation is required to participate.", fa: "برای شرکت، دعوت‌نامه لازم است.",
    ar: "الدعوة مطلوبة للمشاركة.", tr: "Katılım için davet gerekli." },
  OTP_INVALID: {
    en: "That code is incorrect. Please try again.", fa: "این کد نادرست است. دوباره تلاش کنید.",
    ar: "الرمز غير صحيح. يرجى المحاولة مرة أخرى.", tr: "Bu kod hatalı. Lütfen tekrar deneyin." },
  OTP_EXPIRED: {
    en: "That code has expired. Request a new one.", fa: "این کد منقضی شده است. کد جدیدی درخواست کنید.",
    ar: "انتهت صلاحية الرمز. اطلب رمزاً جديداً.", tr: "Bu kodun süresi doldu. Yeni bir kod isteyin." },

  OUT_OF_CREDITS: {
    en: "You’re out of bid credits.", fa: "اعتبار پیشنهاد شما تمام شد.",
    ar: "نفدت أرصدة العرض لديك.", tr: "Teklif krediniz bitti." },
  PACKAGE_NOT_FOUND: {
    en: "That bid package is unavailable.", fa: "این بستهٔ پیشنهاد در دسترس نیست.",
    ar: "باقة العروض هذه غير متوفرة.", tr: "Bu teklif paketi mevcut değil." },

  INVALID_TRANSITION: {
    en: "That step isn’t available yet.", fa: "این مرحله هنوز در دسترس نیست.",
    ar: "هذه الخطوة غير متاحة بعد.", tr: "Bu adım henüz kullanılamıyor." },
  AUCTION_NOT_OPEN: {
    en: "This auction isn’t open yet.", fa: "این حراج هنوز باز نشده است.",
    ar: "لم يُفتح هذا المزاد بعد.", tr: "Bu açık artırma henüz açılmadı." },
  AUCTION_CLOSED: {
    en: "This auction has closed.", fa: "این حراج بسته شده است.",
    ar: "أُغلق هذا المزاد.", tr: "Bu açık artırma kapandı." },
  BID_TOO_LOW: {
    en: "Your bid is below the reserve floor.", fa: "پیشنهاد شما کمتر از کف رزرو است.",
    ar: "عرضك أقل من الحد الأدنى المحجوز.", tr: "Teklifiniz rezerv tabanın altında." },
  DUPLICATE_BID: {
    en: "You’ve already placed your sealed bid.", fa: "شما پیشنهاد دربستهٔ خود را قبلاً ثبت کرده‌اید.",
    ar: "لقد قدّمت عرضك المغلق بالفعل.", tr: "Gizli teklifinizi zaten verdiniz." },
  RESERVATION_REQUIRED: {
    en: "Lock your 10% deposit to reserve a place.", fa: "برای رزرو جایگاه، ۱۰٪ ودیعه را قفل کنید.",
    ar: "جمّد عربون 10٪ لحجز مكانك.", tr: "Yer ayırmak için %10 depozitonuzu kilitleyin." },
  DEPOSIT_NOT_LOCKED: {
    en: "Lock your deposit to continue.", fa: "برای ادامه ودیعه را قفل کنید.",
    ar: "جمّد عربونك للمتابعة.", tr: "Devam etmek için depozitonuzu kilitleyin." },
  FULL_LOCK_REQUIRED: {
    en: "Lock the full amount to enter the room.", fa: "برای ورود به اتاق، کل مبلغ را قفل کنید.",
    ar: "جمّد المبلغ الكامل لدخول الغرفة.", tr: "Odaya girmek için tam tutarı kilitleyin." },
  PRICE_CHANGED: {
    en: "The price has moved. Review the new price and try again.",
    fa: "قیمت تغییر کرد. قیمت جدید را ببینید و دوباره تلاش کنید.",
    ar: "تغيّر السعر. راجع السعر الجديد وحاول مرة أخرى.",
    tr: "Fiyat değişti. Yeni fiyatı görüp tekrar deneyin." },
  LOT_NOT_CERTIFIED: {
    en: "This lot is still awaiting certification.", fa: "این کالا هنوز در انتظار صدور گواهی است.",
    ar: "لا تزال هذه القطعة بانتظار التوثيق.", tr: "Bu parça hâlâ sertifika bekliyor." },
  WEEKLY_CAP_REACHED: {
    en: "This week’s catalogue is full. Try again next week.",
    fa: "ظرفیت کاتالوگ این هفته تکمیل است. هفتهٔ بعد تلاش کنید.",
    ar: "اكتمل كتالوج هذا الأسبوع. حاول الأسبوع المقبل.",
    tr: "Bu haftanın kataloğu dolu. Gelecek hafta tekrar deneyin." },

  INSUFFICIENT_FUNDS: {
    en: "Insufficient wallet balance.", fa: "موجودی کیف پول کافی نیست.",
    ar: "رصيد المحفظة غير كافٍ.", tr: "Cüzdan bakiyesi yetersiz." },
  FUNDING_WINDOW_EXPIRED: {
    en: "The 24-hour funding window has passed.", fa: "مهلت ۲۴ ساعتهٔ پرداخت گذشته است.",
    ar: "انقضت مهلة التمويل البالغة 24 ساعة.", tr: "24 saatlik ödeme süresi geçti." },
  ALREADY_FUNDED: {
    en: "This purchase is already funded.", fa: "این خرید قبلاً پرداخت شده است.",
    ar: "تم تمويل هذا الشراء بالفعل.", tr: "Bu alışveriş zaten ödendi." },
  ESCROW_FORFEITED: {
    en: "The deposit was forfeited.", fa: "ودیعه ضبط شد.",
    ar: "تمت مصادرة العربون.", tr: "Depozito yandı." },
  DISPUTE_IN_PROGRESS: {
    en: "A dispute is in progress. Release is on hold.",
    fa: "یک داوری در جریان است. آزادسازی معلق است.",
    ar: "هناك نزاع قيد المعالجة. الإفراج معلّق.",
    tr: "Bir anlaşmazlık sürüyor. Bırakma beklemede." },
  NOT_ESCROW_PARTICIPANT: {
    en: "You’re not a party to this escrow.", fa: "شما طرف این اسکرو نیستید.",
    ar: "أنت لست طرفاً في هذا الضمان.", tr: "Bu emanetin tarafı değilsiniz." },
};

// --- 3. emit one JSON per language: UI keys (sorted) + err block (sorted) ---
const sortObj = (o) => Object.fromEntries(Object.keys(o).sort().map((k) => [k, o[k]]));

for (const lang of LANGS) {
  const ui = { ...dict[lang] };
  delete ui._dir;
  const err = {};
  for (const code of Object.keys(ERR).sort()) err[code] = ERR[code][lang];
  const out = { ...sortObj(ui), err };
  writeFileSync(join(here, `${lang}.json`), JSON.stringify(out, null, 2) + '\n', 'utf8');
  console.log(`wrote i18n/${lang}.json  (${Object.keys(ui).length} ui keys + ${Object.keys(err).length} err codes)`);
}
