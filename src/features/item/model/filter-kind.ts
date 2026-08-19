import type { Field } from "@/features/table";
import type { Filter, FilterOperator } from "./query";

/**
 * Каким элементом управления фильтруется поле и какие условия ему
 * доступны.
 *
 * Тип поля решает только это. Дальше фильтр знает своё условие сам,
 * и тело запроса собирается без оглядки на схему.
 */
export type FilterKind = "set" | "text" | "boolean" | "range" | "relation";

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

  DATE: "range",
  DATE_TIME: "range",
  DATE_TIME_WITHOUT_TIME_ZONE: "range",

  SINGLE_LINE: "text",
  MULTI_LINE: "text",
  EMAIL: "text",
  PHONE: "text",
  INTERNATION_PHONE: "text",
  NUMBER: "text",
  FLOAT: "text",
  INCREMENT_ID: "text",
  UUID: "text",
  FORMULA_FRONTEND: "text",
};

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
 * Вид по условию — чтобы нарисовать нужный ввод, зная только фильтр.
 *
 * Связь сюда не попадает: у неё то же условие `is`, что и у точного
 * совпадения по тексту, и различает их только тип поля. Кто рисует
 * ввод — сначала спрашивает filterKind(field), и лишь потом условие.
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
