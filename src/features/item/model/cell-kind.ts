import type { Field } from "@/features/table";

/**
 * Как рисовать и как править ячейку.
 *
 * Типов полей в ucode около сорока, и половина из них — одно и то же
 * с точки зрения таблицы: EMAIL, PHONE, LINK и SINGLE_LINE это строка
 * в одну строчку. Поэтому тип поля сводится к виду ячейки один раз
 * здесь, а рендер и редактор ветвятся по виду, а не по типу. Иначе
 * каждый новый тип приходится добавлять в два switch'а, и они расходятся
 * — ровно это в старом ucode и произошло: у него ячейка и форма
 * поддерживают разные наборы типов.
 *
 * Незнакомый тип — текст. Схему меняет админ, и типы появляются быстрее,
 * чем ветки здесь.
 */
export type CellKind =
  | "text"
  /**
   * Подпись-разделитель, а не значение: печатается название поля,
   * одинаковое во всех строках. Так устроен TEXT — см. BY_TYPE.
   */
  | "label"
  /** Адрес: то же поле ввода, но по значению можно перейти. */
  | "link"
  | "longtext"
  | "number"
  | "boolean"
  | "date"
  /** Момент времени с часовым поясом: показывается в поясе браузера. */
  | "datetime"
  /** Время без пояса: показывается ровно так, как записано. */
  | "datetime_naive"
  | "time"
  | "status"
  | "multiselect"
  | "relation"
  | "image"
  | "file"
  | "color"
  | "icon"
  | "json"
  /** Точка на карте: в базе строка «широта,долгота». */
  | "map"
  /** Область на карте: в базе JSON со списком координат. */
  | "polygon"
  /** Выражение, которое считает браузер по полям той же строки. */
  | "formula"
  /**
   * Кнопка: значения нет вовсе. Колонка в базе пустая всегда, а клик
   * зовёт функцию бэкенда — см. ui/ButtonCell.
   */
  | "button"
  /** QR: значение рисуется кодом, а не печатается строкой. */
  | "qr"
  /** Штрихкод: то же самое полосами. */
  | "barcode"
  | "scanner"
  | "password";

const BY_TYPE: Record<string, CellKind> = {
  SINGLE_LINE: "text",
  EMAIL: "text",
  PHONE: "text",
  INTERNATION_PHONE: "text",
  LINK: "link",
  UUID: "text",
  RANDOM_UUID: "text",
  PRIMARY_KEY: "text",
  INCREMENT_ID: "text",
  /*
   * Не число, хотя называется numbers: колонка VARCHAR, а значение
   * собирается как «приставка + дефис + цифры» — причём дефис
   * приклеивается и без приставки (`pkg/helper/generator.go:21`,
   * без проверки на пустоту). Числом такое печатается как «-4821»:
   * моноширинные цифры с ведущим минусом, вид отрицательной величины.
   * Устроен он как INCREMENT_ID и показывается так же.
   */
  RANDOM_NUMBERS: "text",
  // Строка, собранная бэкендом по шаблону из других полей записи.
  MANUAL_STRING: "text",
  /*
   * Выбор одного варианта из списка. Варианты лежат там же, где
   * у MULTISELECT (`attributes.options`), поэтому и рисуется он тем же
   * чипом: тип другой, а данные те же.
   */
  PICK_LIST: "status",

  /*
   * TEXT — не текст записи, а подпись-разделитель. Старая админка
   * печатает у него `field.label`: в таблице одинаково во всех строках
   * (`Grid/FieldRelationGenerator/HFTextComponent.jsx:20`), в форме —
   * жирным заголовком. Колонка при этом заводится настоящая (VARCHAR
   * умолчанием `GetDataType`), но пишет и читает её только админка,
   * а показывает — как название (docs/FIELD-AUDIT.md, F3).
   */
  TEXT: "label",

  MULTI_LINE: "longtext",
  CODE: "longtext",
  PROGRAMMING_LANGUAGE: "longtext",

  NUMBER: "number",
  FLOAT: "number",
  FLOAT_NOLIMIT: "number",
  INCREMENT_NUMBER: "number",
  // Оба считает бэкенд, в строке уже лежит число: FORMULA — агрегат по
  // связанным строкам, FORMULA_FRONTEND — выражение по своим полям,
  // пересчитывается на вставке и правке. Своё вычисление осталось
  // только для строк, которых пересчёт не касался: см. ui/FormulaCell.
  FORMULA: "number",
  FORMULA_FRONTEND: "formula",

  CHECKBOX: "boolean",
  SWITCH: "boolean",
  BOOLEAN: "boolean",

  DATE: "date",
  DATE_TIME: "datetime",
  DATE_TIME_WITHOUT_TIME_ZONE: "datetime_naive",
  TIME: "time",

  STATUS: "status",
  MULTISELECT: "multiselect",

  LOOKUP: "relation",
  LOOKUPS: "relation",

  PHOTO: "image",
  MULTI_IMAGE: "image",
  CUSTOM_IMAGE: "image",

  FILE: "file",
  MULTI_FILE: "file",
  VIDEO: "file",

  COLOR: "color",
  ICON: "icon",

  BUTTON: "button",

  JSON: "json",
  MAP: "map",
  POLYGON: "polygon",

  QR: "qr",
  BARCODE: "barcode",
  CODABAR: "barcode",
  /*
   * SCAN_BARCODE — поле под сканер: в колонке тот же код товара,
   * поэтому показываем его штрихкодом, как остальные, а вот правка
   * у него своя. Сканер «набирает» код в поле ввода, и по готовности
   * поле зовёт функцию — когда именно, задают attributes.pressEnter
   * и attributes.length (см. ui/CellEditor, ScannerEditor).
   */
  SCAN_BARCODE: "scanner",

  PASSWORD: "password",
};

/**
 * Каким кодированием рисовать штрихкод.
 *
 * CODABAR рисуется CODE39, и это не опечатка: ровно так его печатала
 * старая админка (`<Barcode format="CODE39">`), и наклейки, уже
 * наклеенные на товар, читаются сканером как CODE39. Нарисовать
 * «настоящий» Codabar значит выдать другой рисунок под тем же значением.
 *
 * Остальные — CODE128: он принимает и цифры, и буквы, и это умолчание
 * jsbarcode, с которым старая админка рисовала BARCODE.
 */
export function barcodeFormat(type: string): "CODE39" | "CODE128" {
  return type === "CODABAR" ? "CODE39" : "CODE128";
}

export function cellKind(type: string): CellKind {
  return BY_TYPE[type] ?? "text";
}

/**
 * Значение считает бэкенд — такие поля только читаются, каким бы ни был
 * вид ячейки. Отправленное значение он всё равно перезапишет своим,
 * и правка выглядела бы применившейся ровно до перезагрузки.
 */
const COMPUTED = new Set([
  "INCREMENT_ID",
  "INCREMENT_NUMBER",
  // Случайная строка: её выдаёт бэкенд при вставке, как и номер.
  "RANDOM_TEXT",
  /*
   * Собирается по шаблону при вставке. Правку бэкенд не перезапишет —
   * шаблон он читает только на создании, — но править её всё равно
   * нельзя: значение обещает быть шаблоном, а не тем, что кто-то
   * однажды вписал руками.
   */
  "MANUAL_STRING",
  "RANDOM_NUMBERS",
  "RANDOM_UUID",
  "PRIMARY_KEY",
  "UUID",
  "FORMULA",
  "FORMULA_FRONTEND",
]);

/**
 * Что правится прямо в таблице.
 *
 * Всё, кроме пароля и подписи. PASSWORD не правится сознательно: сброс
 * пароля вслепую из строки таблицы — не то действие, которое делают
 * одним кликом (задают его в карточке). У `label` править нечего вовсе:
 * это название поля, а не значение записи.
 */
const EDITABLE: ReadonlySet<CellKind> = new Set<CellKind>([
  "text",
  "link",
  "longtext",
  "relation",
  "number",
  "boolean",
  "date",
  "datetime",
  "datetime_naive",
  "time",
  "status",
  "multiselect",
  "map",
  "image",
  "file",
  "color",
  "icon",
  "json",
  "polygon",
  // Код рисуется из значения, а значение — обычная строка: его вводят
  // руками или присылает сканер.
  "qr",
  "barcode",
  "scanner",
]);

/**
 * Хранит ли поле список значений.
 *
 * Разница видна только в записи: у множественных типов в строке лежит
 * массив адресов, у одиночных — одна строка. Читаются они одинаково
 * (toList), а вот записать массив в колонку PHOTO нельзя.
 */
const MULTI = new Set(["MULTI_IMAGE", "MULTI_FILE"]);

export function isMultiValue(type: string): boolean {
  return MULTI.has(type);
}

/** Вид редактора или null, если поле только читается. */
export function editorKind(field: Field): CellKind | null {
  if (!field.editable || COMPUTED.has(field.type)) return null;

  const kind = cellKind(field.type);
  if (!EDITABLE.has(kind)) return null;

  // Выбор без вариантов — не выбор: админ их не завёл, и редактор
  // открылся бы пустым списком.
  if ((kind === "status" || kind === "multiselect") && field.options.size === 0) return null;

  return kind;
}
