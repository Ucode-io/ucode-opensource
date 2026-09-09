import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import en from "@/locales/en.json";
import ru from "@/locales/ru.json";
import uz from "@/locales/uz.json";

/** Ключ перевода. Строка, которой нет в en.json, не скомпилируется. */
export type TranslationKey = keyof typeof en;

/**
 * Перевод как обычная функция: ключ и подстановки → строка.
 *
 * Нужен там, где `t` уезжает ПАРАМЕТРОМ в чистую функцию. Родной тип
 * `TFunction` — генерик с выводом по опциям (`returnDetails`,
 * `returnObjects`, запасное значение), и на каждый вызов с подстановкой
 * компилятор разворачивает его поверх union'а из восьми сотен ключей.
 * На таком размере он упирается в предел глубины и отвечает «type
 * instantiation is excessively deep» — причём в том файле, который
 * просто позвал `t`, а не в i18next.
 *
 * Здесь же вывода нет вовсе: два параметра, возврат — строка. Проверка
 * ключа остаётся (TranslationKey), а разворачивать нечего.
 */
export interface Translate {
  (key: TranslationKey): string;
  /*
   * Две формы вызова, а не один необязательный параметр: с включённым
   * exactOptionalPropertyTypes необязательный параметр значит «может
   * приехать undefined», а опции i18next такого не принимают — и
   * настоящий `t` перестаёт подходить под этот тип.
   */
  (key: TranslationKey, values: Record<string, string | number>): string;
}

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
/**
 * Ключи ПЛОСКИЕ, и точка в них — часть имени, а не вложенность.
 *
 * Это надо сказать i18next дважды — типам и рантайму, — иначе он
 * считает `settings.profile` путём и ищет вложенный объект, а плоский
 * ключ находит только запасным разбором.
 *
 * Типам это важнее: с разделителем «.» их машинерия перебирает все
 * возможные разбиения каждого ключа, и на восьмой сотне ключей вывод
 * упирается в предел глубины — компилятор отвечает «type instantiation
 * is excessively deep» в случайном файле, который просто позвал `t`
 * с параметром. Проверка ключей при этом не слабеет: `TranslationKey`
 * по-прежнему выводится из en.json.
 */
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "translation";
    resources: { translation: typeof en };
    keySeparator: false;
    nsSeparator: false;
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
  // Те же две настройки, что и у типов выше: ключ плоский целиком.
  keySeparator: false,
  nsSeparator: false,
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
