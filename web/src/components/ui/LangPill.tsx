import { useI18n } from "@/i18n/I18nProvider";
import { LABELS, LANGS } from "@/i18n/locales";

// Cycles through the four supported languages (en → fa → ar → tr).
export function LangPill() {
  const { lang, setLang } = useI18n();
  const next = LANGS[(LANGS.indexOf(lang) + 1) % LANGS.length];
  return (
    <button
      onClick={() => setLang(next)}
      className="mono"
      aria-label="language"
      style={{
        width: 38, height: 38, borderRadius: "var(--r-1)", border: "1px solid var(--line)",
        background: "var(--bg-1)", color: "var(--gold-pale)", cursor: "pointer", fontSize: 12, fontWeight: 600,
      }}
    >
      {LABELS[lang]}
    </button>
  );
}
