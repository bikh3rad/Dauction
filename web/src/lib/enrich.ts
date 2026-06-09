// Client-owned presentation enrichment for lots.
// The backend Lot is intentionally minimal + language-neutral (no category, no
// imagery, no accent). The client derives those presentation hints it owns:
// a category drives the editorial product line-art glyph and an accent tint.
// Inference is best-effort from the maison/title keywords with a deterministic
// hash fallback, so the same lot always renders the same way.

import type { Category, Lot } from "@/types";

const CATEGORIES: Category[] = [
  "horology", "bag", "sneaker", "perfume", "art", "painting", "jewel",
];

// category → the ProductArt key + an accent used for tints/glows.
export const CATEGORY_META: Record<Category, { art: string; accent: string }> = {
  horology: { art: "watch", accent: "#1d3a4a" },
  bag:      { art: "bag", accent: "#5E1226" },
  sneaker:  { art: "sneaker", accent: "#2a2a2a" },
  perfume:  { art: "bottle", accent: "#3A0A16" },
  art:      { art: "frame", accent: "#4A0F1E" },
  painting: { art: "painting", accent: "#4A0F1E" },
  jewel:    { art: "ring", accent: "#3A2F1A" },
};

const KEYWORDS: Array<[Category, RegExp]> = [
  ["horology", /watch|nautilus|daytona|chronograph|rolex|patek|richard mille|tourbillon|flyback|RM ?\d/i],
  ["bag", /birkin|kelly|bag|handbag|hermès|hermes|chanel|flap|maroquinerie|niloticus|epsom/i],
  ["sneaker", /sneaker|jordan|air ?jordan|nike|dunk|yeezy|grail/i],
  ["perfume", /perfume|parfum|flacon|fragrance|baccarat|clive christian|cologne/i],
  ["painting", /oil on|painting|canvas|atrium|linen|estate.*oil/i],
  ["art", /screenprint|warhol|kusama|print|lithograph|art|edition|ed\./i],
  ["jewel", /jewel|bracelet|ring|diamond|cartier|necklace|pavé|pave|joaillerie|gold/i],
];

function hashCategory(seed: string): Category {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  return CATEGORIES[h % CATEGORIES.length];
}

export function categoryOf(lot: Pick<Lot, "id" | "title" | "description">): Category {
  const hay = `${lot.title} ${lot.description ?? ""}`;
  for (const [cat, re] of KEYWORDS) if (re.test(hay)) return cat;
  return hashCategory(lot.id || lot.title);
}

export function accentOf(lot: Pick<Lot, "id" | "title" | "description">): string {
  return CATEGORY_META[categoryOf(lot)].accent;
}

export function artOf(lot: Pick<Lot, "id" | "title" | "description">): string {
  return CATEGORY_META[categoryOf(lot)].art;
}

// Category display labels (the prototype kept these in data.js, not the i18n
// catalog — they are object-class names, client-owned presentation).
const CATEGORY_LABELS: Record<Category, Record<string, string>> = {
  horology: { en: "Horology", fa: "ساعت", ar: "الساعات", tr: "Saat" },
  bag: { en: "Haute Maroquinerie", fa: "کیف لوکس", ar: "حقائب فاخرة", tr: "Lüks Çanta" },
  sneaker: { en: "Grail Sneaker", fa: "اسنیکر نایاب", ar: "حذاء نادر", tr: "Nadir Sneaker" },
  perfume: { en: "Rare Perfume", fa: "عطر نایاب", ar: "عطر نادر", tr: "Nadir Parfüm" },
  art: { en: "Blue-Chip Art", fa: "اثر هنری", ar: "فن أيقوني", tr: "Seçkin Sanat" },
  painting: { en: "Fine Painting", fa: "نقاشی نفیس", ar: "لوحة فنية", tr: "Güzel Tablo" },
  jewel: { en: "Haute Joaillerie", fa: "جواهر", ar: "مجوهرات", tr: "Yüksek Mücevher" },
};

export function categoryLabel(cat: Category, lang: string): string {
  return CATEGORY_LABELS[cat][lang] ?? CATEGORY_LABELS[cat].en;
}

// The maison (brand) is encoded as a prefix in the prototype's titles
// ("Patek Philippe — Nautilus…"). Real lots may or may not follow that; we
// split on an em/en dash to surface a brand line, falling back to the lot title.
export function maisonOf(title: string): string {
  const m = title.split(/\s+[—–-]\s+/)[0];
  return m && m.length <= 40 ? m : "Dauction";
}
