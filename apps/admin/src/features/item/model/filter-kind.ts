import type { Field } from "@/features/table";
import type { Filter, FilterOperator } from "./query";

/**
 * Каким элементом управления фильтруется поле и какие условия ему
 * доступны.
 *
 * Тип поля решает только это. Дальше фильтр знает своё условие сам,
 * и тело запроса собирается без оглядки на схему.
 */
export type FilterKind = "set" | "text" | "boolean" | "range" | "number" | "relation";

const BY_TYPE: Record<string, FilterKind> = {
  STATUS: "set",
  MULTISELECT: "set",
  PICK_LIST: "set",

  /*
   * Связь. Только LOOKUP: у него в строке лежит своя колонка с uuid
   * (`<чужая таблица>_id`, relation.go:290), и отобрать по ней —
   * обычное условие.
   *
   * LOOKUPS не берём: это либо чужая сторона связи, у которой своей
   * колонки нет вовсе, либо массив `<таблица>_ids` у Many2Many —
   * по нему бэкенд сравнивает через `= ANY`, то есть не «содержит
   * выбранное», а «равно ему целиком» (object_builder.go:1256).
   * Обещать отбор, который вернёт пусто, нельзя.
   */
  LOOKUP: "relation",

  BOOLEAN: "boolean",
  SWITCH: "boolean",
  CHECKBOX: "boolean",

  DATE: "range",
  DATE_TIME: "range",
  DATE_TIME_WITHOUT_TIME_ZONE: "range",

  /*
   * Числа — свой вид, а не текстовый. «Содержит» по числовой колонке
   * бэкенд собирает как точное равенство (`build_query.go:349`,
   * ветка NUMERIC_TYPES), то есть чип обещал бы вхождение, а отбирал
   * совпадение; на нечисловом вводе то же условие отвечает 500.
   *
   * FLOAT_NOLIMIT сюда же, хотя в `helper.NUMERIC_TYPES` его нет
   * (`convert.go:99` знает только NUMBER и FLOAT): голая строка ушла бы
   * у него в `~*` по `double precision` — снова 500. Здесь голая строка
   * не отправляется вовсе: `is` уезжает как `$in` с приведением
   * к VARCHAR, диапазон — сравнениями по числу.
   */
  NUMBER: "number",
  FLOAT: "number",
  FLOAT_NOLIMIT: "number",

  /*
   * Текст — всё, у чего колонка VARCHAR: по ней бэкенд собирает `~*`
   * (регулярку) на «содержит» и `= ANY` с приведением к тексту на
   * «равно» (`build_query.go:93`, `:373`). Оба условия по такой колонке
   * работают, поэтому список идёт по КОЛОНКЕ, а не по смыслу типа:
   * `helper.FIELD_TYPES` (`convert.go:14`) плюс умолчание `GetDataType`
   * — тип, которого он не знает, тоже VARCHAR.
   *
   * Отсюда и штрихкоды с TIME: на складе отбор по коду — первое, что
   * делают, а «10:13» отбирается вхождением не хуже любой строки.
   * Старая админка фильтровала их все — через `default: <DefaultFilter>`
   * (`FilterGenerator/index.jsx:160`), то есть тем же текстовым вводом.
   *
   * Чего здесь нет и не будет: PASSWORD (отбирать по хэшу нечего),
   * файлы и картинки (в колонке адрес), COLOR и ICON, а также всё
   * с нетекстовой колонкой — числа, даты, массивы. Числовая колонка
   * под `~*` отвечает 500, см. NUMBER выше.
   */
  SINGLE_LINE: "text",
  MULTI_LINE: "text",
  EMAIL: "text",
  PHONE: "text",
  INTERNATION_PHONE: "text",
  INCREMENT_ID: "text",
  UUID: "text",
  FORMULA_FRONTEND: "text",
  LINK: "text",
  CODE: "text",
  PROGRAMMING_LANGUAGE: "text",
  MANUAL_STRING: "text",
  RANDOM_TEXT: "text",
  RANDOM_NUMBERS: "text",
  RANDOM_UUID: "text",
  TIME: "text",
  BARCODE: "text",
  CODABAR: "text",
  SCAN_BARCODE: "text",
};

/** Время в колонке, а не только дата: у них граница «по» — до полуночи. */
const TIME_TYPES = new Set(["DATE_TIME", "DATE_TIME_WITHOUT_TIME_ZONE"]);

/**
 * Условия по видам. Первое — по умолчанию.
 *
 * У списка условие одно: «не входит» бэкенд не поддерживает ($nin
 * отвечает 500). Показывать выбор из одного пункта незачем — интерфейс
 * рисует название условия текстом, а стрелку только там, где есть
 * из чего выбирать.
 */
const OPERATORS: Record<FilterKind, FilterOperator[]> = {
  set: ["any"],
  text: ["contains", "is"],
  boolean: ["equals"],
  range: ["between", "after", "before"],
  // «Содержит» у числа нет намеренно: см. NUMBER в BY_TYPE.
  number: ["is", "between", "after", "before"],
  // У связи условие одно: выбранные строки. «Содержит» по uuid
  // означало бы поиск по кускам идентификатора.
  relation: ["is"],
};

/**
 * По каким полям фильтровать нельзя.
 *
 * Картинки и файлы — потому что фильтровать нечего (так же в старой
 * версии).
 */
export function filterKind(field: Field): FilterKind | null {
  // Список вариантов может быть пуст, если админ их не завёл: тогда
  // выбирать не из чего, и поле в список не попадает.
  if (BY_TYPE[field.type] === "set" && field.options.size === 0) return null;

  return BY_TYPE[field.type] ?? null;
}

export function operatorsFor(kind: FilterKind): FilterOperator[] {
  return OPERATORS[kind];
}

/**
 * Значение границы диапазона, как оно ложится в фильтр.
 *
 * Поле ввода даёт «ГГГГ-ММ-ДД», а колонка у времени — `TIMESTAMP`,
 * и бэкенд сравнивает присланное как есть (`build_query.go:371`).
 * Поэтому «по 10 января» без дотяжки означает «до 10 января 00:00»:
 * весь последний день выпадает из отбора молча. Старая админка тянула
 * границу руками — `end.setHours(23, 59, 59)` (`YDatePicker.jsx:38`).
 *
 * Конца суток требует ВЕРХНЯЯ граница: `по` у диапазона и `после`
 * у односторонних. У `до` сравнение строгое (`$lt`), и там полночь
 * как раз и значит «этот день целиком не входит».
 */
export function rangeBound(
  field: Field,
  op: FilterOperator,
  index: number,
  value: string,
): string {
  if (!value || !TIME_TYPES.has(field.type)) return value;

  const upper = op === "after" ? index === 0 : index === 1;
  return upper ? `${value}T23:59:59.999` : value;
}

/**
 * Вид по условию — чтобы нарисовать нужный ввод, зная только фильтр.
 *
 * Связь и число сюда не попадают: у связи то же условие `is`, что
 * и у точного совпадения по тексту, у числа — те же `between`,
 * `after` и `before`, что у даты. Различает их только тип поля,
 * поэтому кто рисует ввод — сначала спрашивает filterKind(field),
 * и лишь потом условие.
 */
export function kindOfOperator(op: FilterOperator): FilterKind {
  if (op === "any") return "set";
  if (op === "equals") return "boolean";
  if (op === "contains" || op === "is") return "text";
  return "range";
}

/** Пустой фильтр нужного вида — чтобы чип появился в подшапке. */
export function emptyFilter(kind: FilterKind): Filter {
  return { op: OPERATORS[kind][0]!, values: [] };
}
