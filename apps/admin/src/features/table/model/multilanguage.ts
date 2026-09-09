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
 * База слага без кода языка. `null` — в слаге нет кода языка проекта.
 *
 * Код проверяется по списку языков проекта, а не «всё после последнего
 * подчёркивания»: слаг `order_id` не должен превратиться в базу `order`
 * с языком `id`. Старая админка резала именно по последнему `_`
 * и на поле `created_by` спотыкалась.
 *
 * Флаг `enable_multilanguage` здесь НЕ спрашивается, и это не небрежность.
 * Шлюз ставит его на каждое языковое поле, но object_builder не пишет
 * его в базу: в INSERT колонки `enable_multilanguage` нет вовсе
 * (storage/postgres/field.go:116). Поле, созданное мультиязычным,
 * возвращается с флагом false — «единственно истинный» признак
 * у только что созданного поля просто отсутствует.
 *
 * Настоящий признак поэтому — форма набора: см. languageGroups.
 */
export function baseSlug(field: Field, languages: string[]): string | null {
  return slugBase(field.slug, languages);
}

function slugBase(slug: string, languages: string[]): string | null {
  for (const code of languages) {
    const suffix = `_${code}`;
    if (slug.endsWith(suffix)) return slug.slice(0, -suffix.length);
  }

  return null;
}

/**
 * Языковые группы набора: база слага → её варианты.
 *
 * Группой считается база, у которой в наборе есть хотя бы два языковых
 * варианта, либо один — но помеченный флагом. Одиночное поле `title_en`
 * без соседей — это просто поле с таким именем, и сводить его не с чем:
 * иначе колонка молча теряла бы «en» в подписи.
 */
export function languageGroups(fields: Field[], languages: string[]): Map<string, Field[]> {
  const groups = new Map<string, Field[]>();

  for (const field of fields) {
    const base = baseSlug(field, languages);
    if (base === null) continue;

    groups.set(base, [...(groups.get(base) ?? []), field]);
  }

  for (const [base, variants] of groups) {
    if (variants.length < 2 && !variants.some((field) => field.multilanguage)) {
      groups.delete(base);
    }
  }

  return groups;
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
  return languageGroups(fields, languages).size > 0;
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

  const groups = languageGroups(fields, languages);
  const chosen: Field[] = [];
  const seen = new Set<string>();

  for (const field of fields) {
    const base = baseSlug(field, languages);
    const variants = base === null ? undefined : groups.get(base);

    if (base === null || !variants) {
      chosen.push(field);
      continue;
    }

    if (seen.has(base)) continue;
    seen.add(base);

    const match = variants.find((item) => item.slug === `${base}_${active}`);
    chosen.push(match ?? variants[0] ?? field);
  }

  return chosen;
}

/**
 * Языковые колонки, сведённые к одной: вариант на активном языке
 * и подпись без кода языка.
 *
 * Это то, что видит человек и в таблице, и в карточке: одна колонка
 * «Название», а не три подряд — «Название (en)», «Название (cyr)»,
 * «Название (ru)». Свести их обязан тот, кто рисует список полей;
 * бэкенд отдаёт языки отдельными полями и иначе не умеет.
 *
 * Сводится ПОСЛЕ выбора колонок view, а не вместо него: скрытая
 * колонка остаётся скрытой на всех языках.
 */
export function collapseLanguages(fields: Field[], languages: string[], active: string): Field[] {
  if (!languages.length) return fields;

  const groups = languageGroups(fields, languages);

  return fieldsForLanguage(fields, languages, active).map((field) => {
    const base = baseSlug(field, languages);
    const code = base === null || !groups.has(base) ? null : field.slug.slice(base.length + 1);
    if (code === null) return field;

    return {
      ...field,
      label: stripLanguage(field.label, code),
      labels: Object.fromEntries(
        Object.entries(field.labels).map(([key, value]) => [key, stripLanguage(value, code)]),
      ),
    };
  });
}

/**
 * Слаг условия, переехавший на вариант активного языка данных.
 *
 * Фильтр и сортировка ключуются слагом поля, а у мультиязычного поля
 * слаг у каждого языка свой: условие, заведённое при английском языке
 * данных, записано как `title_en`. После переключения на кириллицу
 * человек смотрит на колонку `title_cyr`, и условие по `title_en`
 * молча отбирает не то, что видно на экране.
 *
 * Переезд только внутри настоящей языковой группы (см. languageGroups)
 * и только на существующий вариант: нет варианта на активном языке —
 * слаг остаётся как есть, условие продолжает бить по тому языку,
 * значения которого и показываются (см. fieldsForLanguage).
 */
export function localizeSlug(
  slug: string,
  fields: Field[],
  languages: string[],
  active: string,
): string {
  const base = slugBase(slug, languages);
  if (base === null || slug === `${base}_${active}`) return slug;

  const variants = languageGroups(fields, languages).get(base);
  return variants?.some((field) => field.slug === `${base}_${active}`)
    ? `${base}_${active}`
    : slug;
}

/**
 * То же для карты условий по слагу — фильтров.
 *
 * Столкновение ключей не тихое: если условие по активному языку уже
 * есть (в адресе лежали оба варианта), чужое остаётся на своём слаге —
 * молча выкинуть заданное условие хуже, чем оставить его на старом языке.
 */
export function localizeKeys<T>(
  map: Record<string, T>,
  fields: Field[],
  languages: string[],
  active: string,
): Record<string, T> {
  const out: Record<string, T> = {};

  for (const [slug, value] of Object.entries(map)) {
    const next = localizeSlug(slug, fields, languages, active);
    const collides = next !== slug && (next in map || next in out);
    out[collides ? slug : next] = value;
  }

  return out;
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
