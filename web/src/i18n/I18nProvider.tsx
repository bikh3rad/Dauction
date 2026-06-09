import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  type Dir,
  type Lang,
  dirOf,
  loadLang,
  saveLang,
  translate,
} from "./locales";

interface I18nValue {
  lang: Lang;
  dir: Dir;
  t: (key: string) => string;
  setLang: (lang: Lang) => void;
}

const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => loadLang());
  const dir = dirOf(lang);

  const setLang = useCallback((next: Lang) => {
    setLangState(next);
    saveLang(next);
  }, []);

  // Cascade dir + lang to <html> so CSS vars (RTL font, etc.) apply everywhere.
  useEffect(() => {
    document.documentElement.setAttribute("dir", dir);
    document.documentElement.lang = lang;
  }, [dir, lang]);

  const value = useMemo<I18nValue>(
    () => ({ lang, dir, t: (key: string) => translate(lang, key), setLang }),
    [lang, dir, setLang],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within <I18nProvider>");
  return ctx;
}
