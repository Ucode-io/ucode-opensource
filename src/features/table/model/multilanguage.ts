import type { Field } from "./types";

/**
 * Мультиязычное поле: сборка языковых колонок обратно в одно поле.
 *
 * Бэкенд не хранит карту значений по языкам. Мультиязычное поле — это
 * НЕСКОЛЬКО колонок, по одной на язык данных проекта, и код языка
 * дописан к слагу через подчёркивание: `title_en`, `title_cyr`.
 * Признак у каждой из них один и тот же — `enable_multilanguage`.
 *
 * Так это устроено и в бэкенде: layout отдаётся с параметром
 * `language_setting`, и там поля отсеиваются ровно по суффиксу
 * (layout.go:549 — `strings.HasSuffix(field.Slug, "_"+req.LanguageSetting)`).
 * Мы делаем то же самое на клиенте: язык переключается в карточке,
 * без перезапроса раскладки.
 *
 * Без этой сборки карточка показывает три соседние строки «Название»,
 * «Название», «Название», и какая из них узбекская — видно только по
 * значению.
 */

/**
 * База слага без кода языка. `null` — поле не языковое.
 *
 * Код проверяется по списку языков проекта, а не «всё после последнего
 * подчёркивания»: слаг `order_id` не должен превратиться в базу `order`
 * с языком `id`. Старая админка резала именно по последнему `_`
 * и на поле `created_by` спотыкалась.
 */
export function baseSlug(field: Field, languages: string[]): string | null {
  if (!field.multilanguage) return null;

  for (const code of languages) {
    const suffix = `_${code}`;
    if (field.slug.endsWith(suffix)) return field.slug.slice(0, -suffix.length);
  }

  return null;
}

/** Код языка данных в слаге поля. `null` — поле не языковое. */
export function fieldLanguage(field: Field, languages: string[]): string | null {
  const base = baseSlug(field, languages);
  return base === null ? null : field.slug.slice(base.length + 1);
}

/**
 * Есть ли в наборе хоть одно мультиязычное поле. По этому вопросу
 * решается, показывать ли переключатель языка: у таблицы без таких
 * полей он ничего не переключает.
 */
export function hasMultilanguage(fields: Field[], languages: string[]): boolean {
  return fields.some((field) => baseSlug(field, languages) !== null);
}

/**
 * Поля карточки на одном языке: языковые варианты схлопнуты до одного,
 * на месте ПЕРВОГО из них.
 *
 * Место первого, а не место варианта на активном языке: иначе поле
 * прыгало бы вверх-вниз по карточке при переключении языка — порядок
 * колонок в схеме у разных языков разный.
 *
 * Варианта на активном языке может не быть вовсе (язык добавили в проект
 * позже, чем поле). Тогда берётся первый существующий: показать пустоту
 * вместо значения хуже, чем показать значение не на том языке — второе
 * хотя бы видно.
 */
export function fieldsForLanguage(
  fields: Field[],
  languages: string[],
  active: string,
): Field[] {
  if (!languages.length) return fields;

  const chosen: Field[] = [];
  const seen = new Set<string>();

  for (const field of fields) {
    const base = baseSlug(field, languages);

    if (base === null) {
      chosen.push(field);
      continue;
    }

    if (seen.has(base)) continue;
    seen.add(base);

    const variants = fields.filter((item) => baseSlug(item, languages) === base);
    const match = variants.find((item) => item.slug === `${base}_${active}`);

    chosen.push(match ?? variants[0] ?? field);
  }

  return chosen;
}

/**
 * Подпись без кода языка.
 *
 * Подписи заводят по-разному: «Название (en)», «Название en», просто
 * «Название» на всех вариантах. Отрезаем ровно код активного языка
 * в конце и осиротевшие скобки — угадывать дальше нечего, а лишнее
 * обрезание испортило бы честную подпись.
 */
export function stripLanguage(label: string, code: string): string {
  if (!code) return label;

  const trimmed = label.trim();
  const lower = trimmed.toLowerCase();
  const suffix = code.toLowerCase();

  for (const candidate of [`(${suffix})`, `[${suffix}]`, ` ${suffix}`, `_${suffix}`]) {
    if (lower.endsWith(candidate)) return trimmed.slice(0, -candidate.length).trim();
  }

  return trimmed;
}
