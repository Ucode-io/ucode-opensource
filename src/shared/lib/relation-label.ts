import { formatDate, type DateKind } from "./date-value";
import i18n from "./i18n";
import { formatNumber } from "./number-value";

/**
 * Подпись связанной строки: значения полей показа через пробел.
 *
 * Живёт в shared, потому что читают её обе стороны и по одному правилу:
 * ячейка-связь в таблице (features/item) и выбор строк в условии
 * агрегата (features/table). Держать две копии значило бы однажды
 * показать одну и ту же запись по-разному на двух экранах.
 *
 * Поля показа задаёт админ в настройках связи. Не заданы — подписи нет,
 * и подставлять `title` или `name` нельзя: это угадывание, а не чтение
 * (см. CONTEXT, Relation).
 */
export type ViewField = {
  slug: string;
  /** Тип поля ЧУЖОЙ таблицы: по нему форматируется значение. */
  type: string;
};

/** Временные типы: их значение — не текст, а дата. */
const DATE_KINDS: Record<string, DateKind> = {
  DATE: "date",
  DATE_TIME: "datetime",
  DATE_TIME_WITHOUT_TIME_ZONE: "datetime_naive",
};

/**
 * Числовые типы: у них в подписи разделяются разряды — как в ячейке.
 *
 * Список короткий и свой, а не `cellKind` из features/item: shared
 * не зависит от фич. Здесь важна не форма ячейки, а колонка: числом
 * лежит ровно это (`helper.FIELD_TYPES`: FLOAT и SERIAL).
 */
const NUMBER_TYPES = new Set([
  "NUMBER",
  "FLOAT",
  "FLOAT_NOLIMIT",
  "INCREMENT_NUMBER",
  "FORMULA",
]);

/**
 * @param language язык ДАННЫХ: им отбирается колонка мультиязычного
 *   поля показа. Пусто — отбора нет, берутся все выбранные колонки.
 */
export function relationLabel(
  item: Record<string, unknown>,
  fields: ViewField[],
  language = "",
): string {
  /* Язык ИНТЕРФЕЙСА берётся из i18n, а не передаётся: подпись читают
     из пяти мест, и в трёх из них его нет под рукой, а формат даты
     не может зависеть от того, откуда позвали. */
  const locale = i18n.language;

  return pickLanguage(fields, language)
    .map((field) => part(item[field.slug], field.type, locale))
    .filter(Boolean)
    .join(" ");
}

/**
 * Значение одного поля показа.
 *
 * Связанная строка приезжает как `row_to_json` без обработки
 * (`build_query.go:157`), поэтому дата в ней — сырой ISO. Формат тот же,
 * что в ячейке: разбор и форматтер общие (`shared/lib/date-value`),
 * иначе одна и та же дата в таблице и в подписи связи выглядела бы
 * по-разному.
 *
 * Числа тоже как в ячейке — с разделителями разрядов (FIELD-AUDIT,
 * F15): «Заказ 1 234 567» и «Заказ 1234567» в двух местах одного
 * экрана читаются как два разных числа.
 */
function part(value: unknown, type: string, locale: string): string {
  if (value === null || value === undefined || value === "") return "";
  if (typeof value === "object") return "";

  const kind = DATE_KINDS[type];
  if (kind) return formatDate(value, kind, locale) ?? String(value);

  if (NUMBER_TYPES.has(type)) return formatNumber(value, locale);

  return String(value);
}

/**
 * Мультиязычные поля показа: оставляем колонку активного языка.
 *
 * В полях показа лежат КОЛОНКИ, а мультиязычное поле — это несколько
 * колонок со слагами `title_en`, `title_cyr`. Выбраны обе — без отбора
 * подпись склеивает их подряд: «Заголовок Sarlavha». Старая админка
 * выбирала подходящую по языку данных (`getRelationFieldTabsLabel`).
 *
 * Отбор идёт от активного языка, а не от списка языков проекта: сюда
 * его передавать неоткуда, а знать, что `_en` — язык, а `_id` — нет,
 * по одному слагу нельзя. Поэтому колонка с суффиксом активного языка
 * вытесняет своих однокоренных соседей, а если её среди выбранных нет
 * — остаются все, как и было.
 */
function pickLanguage(fields: ViewField[], language: string): ViewField[] {
  if (!language) return fields;

  const suffix = `_${language}`;
  const bases = fields
    .filter((field) => field.slug.endsWith(suffix))
    .map((field) => field.slug.slice(0, -suffix.length));

  if (!bases.length) return fields;

  return fields.filter(
    (field) =>
      field.slug.endsWith(suffix) ||
      !bases.some((base) => field.slug === base || field.slug.startsWith(`${base}_`)),
  );
}
