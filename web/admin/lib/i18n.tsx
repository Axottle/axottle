"use client";

import { createContext, useContext, useState, useEffect, useCallback } from "react";
import en from "@/i18n/en.json";
import ru from "@/i18n/ru.json";

export type Language = "en" | "ru";

const translations: Record<Language, typeof en> = { en, ru };

interface I18nContextValue {
  lang: Language;
  setLang: (lang: Language) => void;
  /**
   * Look up a translation key. If the resolved string contains `{{name}}`
   * placeholders, supply a `vars` map to interpolate them.
   *
   *   t("selfsteal.reason.dns_mismatch", {
   *     domain: "example.com", expected_ip: "1.2.3.4", dns_addrs: "5.6.7.8",
   *   })
   *
   * Unknown placeholders are left as-is so we notice missing data instead
   * of silently rendering an empty string. If the key itself is missing,
   * `defaultValue` is returned (defaults to the key).
   */
  // Two call shapes are supported and BOTH must compile cleanly:
  //   t("nav.users")                        — plain lookup
  //   t("nav.users", "Users")               — legacy default-value form
  //   t("a.b", { count: 3 })                — with placeholders
  //   t("a.b", { count: 3 }, "fallback")    — placeholders + fallback
  t: TFunction;
}

export interface TFunction {
  (key: string): string;
  (key: string, defaultValue: string): string;
  (key: string, vars: Record<string, unknown>, defaultValue?: string): string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: React.ReactNode }) {
  // Русский — язык по умолчанию: панель пишется на нём, inline-фолбэки в
  // вызовах t() тоже русские, а даты везде форматируются через "ru-RU". До
  // этого стартовым языком был "en", и оператор без сохранённого выбора
  // получал английский каркас с русскими надписями внутри. Разметка приходит
  // с lang="ru" (на сервере localStorage нет), поэтому начальное состояние
  // обязано совпадать с ней — сохранённый выбор подхватывает эффект ниже.
  const [lang, setLangState] = useState<Language>("ru");

  useEffect(() => {
    const saved = localStorage.getItem("axottle-lang") as Language | null;
    if (saved) setLangState(saved);
  }, []);

  useEffect(() => {
    localStorage.setItem("axottle-lang", lang);
    // Атрибут держим в актуальном состоянии: по нему работают проверка
    // орфографии в полях ввода, скринридеры и предложение перевести страницу.
    document.documentElement.lang = lang;
  }, [lang]);

  const setLang = useCallback((l: Language) => setLangState(l), []);

  const t = useCallback(
    (key: string, varsOrDefault?: Record<string, unknown> | string, defaultValue?: string) => {
      // Backwards-compat: t(key, "default") still works because some
      // existing callers pass a string as the second arg.
      let vars: Record<string, unknown> | undefined;
      let dflt: string;
      if (typeof varsOrDefault === "string") {
        vars = undefined;
        dflt = varsOrDefault;
      } else {
        vars = varsOrDefault;
        dflt = defaultValue ?? key;
      }

      // Interpolate {{name}} placeholders on whatever string we return —
      // dictionary hit OR fallback. Previously a missing key returned the
      // fallback raw, so an inline fallback like "Показать отклонённые
      // ({{count}})" leaked its placeholder whenever the key had no dictionary
      // entry (the common case here). Unknown placeholders are left intact so
      // missing data is visible rather than silently empty.
      const interpolate = (str: string) => {
        if (!vars) return str;
        return str.replace(/\{\{\s*(\w+)\s*\}\}/g, (m, name) => {
          if (vars && name in vars) {
            const v = vars[name];
            if (v === null || v === undefined) return m;
            if (Array.isArray(v)) return v.join(", ");
            return String(v);
          }
          return m;
        });
      };

      const keys = key.split(".");
      let value: any = translations[lang];
      for (const k of keys) {
        value = value?.[k];
        if (value === undefined || value === null) return interpolate(dflt);
      }
      const str = typeof value === "string" ? value : dflt;
      return interpolate(str);
    },
    [lang]
  );

  return (
    <I18nContext.Provider value={{ lang, setLang, t }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within I18nProvider");
  return ctx;
}
