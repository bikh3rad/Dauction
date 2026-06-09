// i18n catalogs + helpers. Reuses the prototype's 290-key catalogs verbatim
// (en/fa/ar/tr) and the locales metadata. The API is language-neutral; only
// UI chrome is translated here (root CLAUDE.md §0.7).

import en from "./catalogs/en.json";
import fa from "./catalogs/fa.json";
import ar from "./catalogs/ar.json";
import tr from "./catalogs/tr.json";
import meta from "./catalogs/locales.json";

export type Lang = "en" | "fa" | "ar" | "tr";
export type Dir = "ltr" | "rtl";

// Catalog values are mostly strings, but a nested `err` object maps error codes
// to messages — so values are `unknown` and resolved with dotted-path support.
type Catalog = Record<string, unknown>;
const CATALOGS: Record<Lang, Catalog> = { en, fa, ar, tr };

function resolve(cat: Catalog, key: string): string | undefined {
  if (typeof cat[key] === "string") return cat[key] as string;
  // dotted path, e.g. "err.ACCESS_DENIED"
  if (key.includes(".")) {
    let cur: unknown = cat;
    for (const part of key.split(".")) {
      if (cur && typeof cur === "object") cur = (cur as Record<string, unknown>)[part];
      else return undefined;
    }
    return typeof cur === "string" ? cur : undefined;
  }
  return undefined;
}

export const LANGS = meta.supported as Lang[];
export const DEFAULT_LANG = meta.default as Lang;
export const LABELS = meta.label as Record<Lang, string>;
const DIRS = meta.dir as Record<Lang, Dir>;

export function dirOf(lang: Lang): Dir {
  return DIRS[lang] ?? "ltr";
}

export function translate(lang: Lang, key: string): string {
  const cat = CATALOGS[lang] ?? CATALOGS.en;
  return resolve(cat, key) ?? resolve(CATALOGS.en, key) ?? key;
}

/** Map a backend error code to a localized message (catalog `err.<CODE>`). */
export function translateError(lang: Lang, code?: string): string | undefined {
  if (!code) return undefined;
  return resolve(CATALOGS[lang] ?? CATALOGS.en, `err.${code}`) ?? resolve(CATALOGS.en, `err.${code}`);
}

const STORAGE_KEY = "dauction.lang";
export function loadLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY) as Lang | null;
    if (saved && LANGS.includes(saved)) return saved;
  } catch {
    /* ignore */
  }
  return DEFAULT_LANG;
}
export function saveLang(lang: Lang): void {
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    /* ignore */
  }
}
