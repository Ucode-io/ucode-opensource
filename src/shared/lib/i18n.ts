import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import en from "@/locales/en.json";
import ru from "@/locales/ru.json";
import uz from "@/locales/uz.json";

/** Ключ перевода. Строка, которой нет в en.json, не скомпилируется. */
export type TranslationKey = keyof typeof en;

export const LOCALES = ["ru", "en", "uz"] as const;
export type Locale = (typeof LOCALES)[number];

/**
 * Язык интерфейса. Не путать с мультиязычными полями (enable_multilanguage) —
 * это язык данных пользователя, см. CONTEXT.md.
 *
 * Локали лежат рядом с кодом и типизированы по en: ключ, которого нет
 * в en.json, не скомпилируется. В старом ucode файлы разъезжались
 * (ru 299 ключей, en 254, ar 176) и никто этого не замечал.
 */
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "translation";
    resources: { translation: typeof en };
  }
}

/*
 * Хранилище читается с оглядкой: этот модуль тянут за собой мутации
 * (им нужен текст ошибки), а их проверяют в node, где localStorage нет.
 * Падать на импорте из-за настройки языка — плохой обмен.
 */
const stored = typeof localStorage === "undefined" ? null : localStorage.getItem("ucode.locale");
const locale = (LOCALES as readonly string[]).includes(stored ?? "")
  ? (stored as Locale)
  : "ru";

void i18next.use(initReactI18next).init({
  lng: locale,
  fallbackLng: "en",
  resources: {
    en: { translation: en },
    ru: { translation: ru },
    uz: { translation: uz },
  },
  interpolation: { escapeValue: false },
});

export function setLocale(next: Locale) {
  localStorage.setItem("ucode.locale", next);
  void i18next.changeLanguage(next);
}

export default i18next;
